package whatsapp

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"

	"github.com/eddmann/whatsapp-cli/internal/store"
)

type BackfillResult struct {
	ChatJID         string    `json:"jid"`
	Count           int       `json:"count"`
	PagesRequested  int       `json:"pages_requested"`
	MessagesSynced  int       `json:"messages_synced"`
	MoreAvailable   bool      `json:"more_available"`
	AnchorMessageID string    `json:"anchor_message_id"`
	AnchorTimestamp time.Time `json:"anchor_timestamp"`
}

type HistorySyncResult struct {
	MessagesSynced int
	MoreAvailable  bool
}

// Client wraps a WhatsApp client with store integration.
type Client struct {
	WA                  *whatsmeow.Client
	Store               *store.DB
	Logger              *slog.Logger
	BaseDir             string
	SyncComplete        chan struct{} // Signals when history sync is complete
	OnDemandHistorySync chan HistorySyncResult
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
		WA:                  client,
		Store:               db,
		Logger:              logger,
		BaseDir:             baseDir,
		SyncComplete:        make(chan struct{}, 1),
		OnDemandHistorySync: make(chan HistorySyncResult, 1),
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

// RequestBackfill requests historical messages before the oldest stored
// message for a chat. WhatsApp controls how much history is returned; the
// recommended request size is 50 messages at a time.
func (c *Client) RequestBackfill(jidStr string, count, pages int, wait time.Duration) (*BackfillResult, error) {
	if count <= 0 {
		return nil, fmt.Errorf("count must be greater than zero")
	}
	if pages <= 0 {
		return nil, fmt.Errorf("pages must be greater than zero")
	}
	if wait <= 0 {
		return nil, fmt.Errorf("wait duration must be greater than zero")
	}
	if c.Store == nil {
		return nil, fmt.Errorf("message store is required")
	}

	jid, err := parseJID(jidStr)
	if err != nil {
		return nil, err
	}

	result := &BackfillResult{ChatJID: jid.String(), Count: count}
	for page := 0; page < pages; page++ {
		oldest, err := c.Store.OldestMessageForChat(jid.String())
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("no stored messages found for %s; sync the chat before requesting older history", jid.String())
			}
			return nil, fmt.Errorf("failed to find oldest message for %s: %w", jid.String(), err)
		}

		anchor, err := buildBackfillAnchor(oldest)
		if err != nil {
			return nil, err
		}
		result.AnchorMessageID = anchor.ID
		result.AnchorTimestamp = anchor.Timestamp

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		c.Logger.Info("requesting backfill", "jid", jid.String(), "count", count, "page", page+1, "before_message_id", anchor.ID, "before_timestamp", anchor.Timestamp)
		_, err = c.WA.SendPeerMessage(ctx, c.WA.BuildHistorySyncRequest(anchor, count))
		cancel()
		if err != nil {
			return nil, fmt.Errorf("failed to send backfill request: %w", err)
		}
		result.PagesRequested++

		select {
		case syncResult := <-c.OnDemandHistorySync:
			result.MessagesSynced += syncResult.MessagesSynced
			result.MoreAvailable = syncResult.MoreAvailable
			if syncResult.MessagesSynced == 0 || !syncResult.MoreAvailable {
				return result, nil
			}
		case <-time.After(wait):
			return result, fmt.Errorf("timed out waiting for on-demand history sync after %s", wait)
		}
	}

	return result, nil
}

func buildBackfillAnchor(message store.Message) (*types.MessageInfo, error) {
	if message.ID == "" {
		return nil, fmt.Errorf("oldest stored message has no ID")
	}

	chat, err := parseJID(message.ChatJID)
	if err != nil {
		return nil, fmt.Errorf("invalid chat JID %q: %w", message.ChatJID, err)
	}

	return &types.MessageInfo{
		MessageSource: types.MessageSource{
			Chat:     chat,
			IsFromMe: message.IsFromMe,
			IsGroup:  chat.Server == types.GroupServer,
		},
		ID:        message.ID,
		Timestamp: message.Timestamp,
	}, nil
}

// resolveSenderName resolves a sender identifier to a display name.
// It checks: 1) LID mappings, 2) contact store, 3) push name.
func (c *Client) resolveSenderName(sender string, senderJID types.JID, pushName string) string {
	// First try LID mappings from our store
	if c.Store != nil {
		if name := c.Store.ResolveSenderName(sender); name != "" {
			return name
		}
	}

	// Try the contact store with phone-based JID
	if !senderJID.IsEmpty() {
		if contact, err := c.WA.Store.Contacts.GetContact(context.Background(), senderJID); err == nil {
			if contact.FullName != "" {
				return contact.FullName
			}
			if contact.PushName != "" {
				return contact.PushName
			}
		}

		// Try as phone-based JID
		phoneJID := types.JID{User: sender, Server: "s.whatsapp.net"}
		if contact, err := c.WA.Store.Contacts.GetContact(context.Background(), phoneJID); err == nil {
			if contact.FullName != "" {
				return contact.FullName
			}
			if contact.PushName != "" {
				return contact.PushName
			}
		}
	}

	// Fall back to push name from the message
	if pushName != "" {
		return pushName
	}

	return ""
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
	if contact, err := c.WA.Store.Contacts.GetContact(context.Background(), parsedJID); err == nil {
		if contact.FullName != "" {
			return contact.FullName
		}
		if contact.BusinessName != "" {
			return contact.BusinessName
		}
		if contact.PushName != "" {
			return contact.PushName
		}
	}

	return parsedJID.User
}
