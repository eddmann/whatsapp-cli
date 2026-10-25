package whatsapp

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"

	"github.com/eddmann/whatsapp-cli/internal/store"
)

// Client wraps a WhatsApp client with store integration.
type Client struct {
	WA           *whatsmeow.Client
	Store        *store.DB
	Logger       *slog.Logger
	BaseDir      string
	SyncComplete chan struct{} // Signals when history sync is complete

	syncCompleteMu    sync.Mutex
	syncCompleteTimer *time.Timer
	backfillMu        sync.Mutex
	pendingBackfill   *pendingBackfillRequest
}

// New creates a new WhatsApp client.
func New(db *store.DB, baseDir string, verbose bool, logger *slog.Logger) (*Client, error) {
	if baseDir == "" {
		return nil, fmt.Errorf("baseDir is required")
	}

	// Configure logging level based on verbose flag
	// Default to Error to suppress whatsmeow warnings (e.g., encryption warnings for multi-device)
	zerologLevel := zerolog.ErrorLevel
	if verbose {
		zerologLevel = zerolog.InfoLevel
	}

	waZerolog := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"}).
		Level(zerologLevel).
		With().
		Timestamp().
		Str("module", "wa").
		Logger()

	dbZerolog := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"}).
		Level(zerologLevel).
		With().
		Timestamp().
		Str("module", "wa-db").
		Logger()

	waLogger := waLog.Zerolog(waZerolog)
	dbLog := waLog.Zerolog(dbZerolog)

	if logger == nil {
		logger = slog.Default()
	}

	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create store dir: %w", err)
	}

	waDBURI := fmt.Sprintf("file:%s/session.db?_foreign_keys=on", baseDir)
	container, err := sqlstore.New(context.Background(), "sqlite3", waDBURI, dbLog)
	if err != nil {
		return nil, fmt.Errorf("failed to open session db: %w", err)
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		if err == sql.ErrNoRows {
			deviceStore = container.NewDevice()
		} else {
			return nil, fmt.Errorf("failed to get device: %w", err)
		}
	}

	client := whatsmeow.NewClient(deviceStore, waLogger)
	if client == nil {
		return nil, fmt.Errorf("failed to create client")
	}

	c := &Client{
		WA:           client,
		Store:        db,
		Logger:       logger,
		BaseDir:      baseDir,
		SyncComplete: make(chan struct{}, 1),
	}
	c.registerHandlers()

	return c, nil
}

// IsAuthenticated returns true if there's a stored session.
func (c *Client) IsAuthenticated() bool {
	return c.WA.Store.ID != nil
}

// IsConnected returns true if connected to WhatsApp.
func (c *Client) IsConnected() bool {
	return c.WA.IsConnected()
}

// IsLoggedIn returns true if logged in to WhatsApp.
func (c *Client) IsLoggedIn() bool {
	return c.WA.IsLoggedIn()
}

// GetDeviceID returns the device ID if authenticated.
func (c *Client) GetDeviceID() (user string, device uint16) {
	if c.WA.Store.ID == nil {
		return "", 0
	}
	return c.WA.Store.ID.User, c.WA.Store.ID.Device
}

// Disconnect disconnects from WhatsApp.
func (c *Client) Disconnect() {
	c.syncCompleteMu.Lock()
	if c.syncCompleteTimer != nil {
		c.syncCompleteTimer.Stop()
		c.syncCompleteTimer = nil
	}
	c.syncCompleteMu.Unlock()

	if c.WA != nil && c.WA.IsConnected() {
		c.WA.Disconnect()
	}
}

// Logout logs out and clears the session.
func (c *Client) Logout() error {
	if c.WA == nil {
		return nil
	}

	if c.WA.IsConnected() {
		if err := c.WA.Logout(context.Background()); err != nil {
			return fmt.Errorf("logout failed: %w", err)
		}
	}

	return nil
}

// getChatName attempts to resolve a friendly chat name.
func (c *Client) getChatName(jid, chatJID string, _ any, sender string) string {
	// Try to get existing name from DB
	var existing sql.NullString
	_ = c.Store.Messages.QueryRow("SELECT name FROM chats WHERE jid = ?", chatJID).Scan(&existing)
	if existing.Valid && existing.String != "" {
		return existing.String
	}

	// Try to resolve from WhatsApp
	parsedJID, err := parseJID(jid)
	if err == nil {
		if name := c.resolvePreferredName(parsedJID); name != "" {
			return name
		}
	}

	// Fallback to sender or JID user part
	if sender != "" {
		return sender
	}

	if idx := strings.Index(chatJID, "@"); idx > 0 {
		return chatJID[:idx]
	}

	return chatJID
}

// resolveSenderName resolves a sender identifier to a display name.
// It checks: 1) LID mappings, 2) contact store (directly, then via the sender's
// phone JID when the sender is a LID), 3) push name, 4) the sender's phone number.
// It returns "" only when nothing better than the raw identifier is known.
func (c *Client) resolveSenderName(sender string, senderJID types.JID, pushName string) string {
	// First try LID mappings from our store
	if c.Store != nil {
		if name := c.Store.ResolveSenderName(sender); name != "" {
			return name
		}
	}

	if !senderJID.IsEmpty() {
		if name := c.contactName(senderJID); name != "" {
			return name
		}

		// Group senders are usually LIDs; contacts are keyed by phone JID.
		if pn, ok := c.phoneJIDFor(senderJID); ok && pn != senderJID.ToNonAD() {
			if name := c.contactName(pn); name != "" {
				return name
			}
		}
	}

	// Fall back to push name from the message
	if pushName != "" && pushName != "-" {
		return pushName
	}

	// A phone number is still more useful than an opaque LID.
	if pn, ok := c.phoneJIDFor(senderJID); ok && pn.User != sender {
		return pn.User
	}

	return ""
}

// phoneJIDFor returns the phone-number JID for a user JID. LIDs are mapped via
// whatsmeow's LID store, falling back to our lid_mappings table.
func (c *Client) phoneJIDFor(jid types.JID) (types.JID, bool) {
	jid = jid.ToNonAD()
	switch jid.Server {
	case types.DefaultUserServer:
		return jid, true
	case types.HiddenUserServer:
	default:
		return types.JID{}, false
	}

	if c.WA != nil && c.WA.Store != nil && c.WA.Store.LIDs != nil {
		if pn, err := c.WA.Store.LIDs.GetPNForLID(context.Background(), jid); err == nil && !pn.IsEmpty() {
			return pn.ToNonAD(), true
		}
	}

	if c.Store != nil {
		if phone, _, found := c.Store.GetLIDMapping(jid.User); found && phone != "" {
			if parsed, err := types.ParseJID(phone); err == nil && strings.Contains(phone, "@") {
				return parsed.ToNonAD(), true
			}
			return types.JID{User: phone, Server: types.DefaultUserServer}, true
		}
	}

	return types.JID{}, false
}

// senderJIDFromUser rebuilds a full JID from a stored sender user part. LIDs
// and phone numbers are both numeric, so a known LID mapping wins.
func (c *Client) senderJIDFromUser(user string) types.JID {
	lid := types.JID{User: user, Server: types.HiddenUserServer}
	if _, ok := c.phoneJIDFor(lid); ok {
		return lid
	}
	return types.JID{User: user, Server: types.DefaultUserServer}
}

// contactName returns the best name in the contact store for a JID, or "".
func (c *Client) contactName(jid types.JID) string {
	if c.WA == nil || c.WA.Store == nil || c.WA.Store.Contacts == nil {
		return ""
	}
	contact, err := c.WA.Store.Contacts.GetContact(context.Background(), jid.ToNonAD())
	if err != nil {
		return ""
	}
	if contact.FullName != "" {
		return contact.FullName
	}
	if contact.BusinessName != "" {
		return contact.BusinessName
	}
	return contact.PushName
}

// resolvePreferredName tries to resolve a human-friendly name for a JID.
func (c *Client) resolvePreferredName(jid interface{}) string {
	// This handles both string and types.JID
	var jidStr string
	switch v := jid.(type) {
	case string:
		jidStr = v
	default:
		jidStr = fmt.Sprintf("%v", jid)
	}

	parsedJID, err := parseJID(jidStr)
	if err != nil {
		return ""
	}

	// Groups
	if parsedJID.Server == "g.us" {
		if info, err := c.WA.GetGroupInfo(context.Background(), parsedJID); err == nil && info.Name != "" {
			return info.Name
		}
		return fmt.Sprintf("Group %s", parsedJID.User)
	}

	// Contacts
	if name := c.contactName(parsedJID); name != "" {
		return name
	}

	// LID-addressed chats: look up the contact by phone JID, and prefer the
	// phone number over the opaque LID when no name is known.
	if pn, ok := c.phoneJIDFor(parsedJID); ok && pn != parsedJID.ToNonAD() {
		if name := c.contactName(pn); name != "" {
			return name
		}
		return pn.User
	}

	return parsedJID.User
}
