package whatsapp

import (
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/types"

	"github.com/eddmann/whatsapp-cli/internal/store"
)

const (
	testLID   = "172610642026735"
	testPhone = "15551234567"
	groupJID  = "120363000000000000@g.us"
)

// newNameTestClient returns a client without a WhatsApp connection, so name
// resolution relies on the lid_mappings table only.
func newNameTestClient(t *testing.T) *Client {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "messages.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(db.CloseQuietly)
	return &Client{Store: db, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func insertMessage(t *testing.T, c *Client, id, chatJID, sender string, senderName any) {
	t.Helper()
	if _, err := c.Store.Messages.Exec(`INSERT OR IGNORE INTO chats (jid) VALUES (?)`, chatJID); err != nil {
		t.Fatalf("insert chat: %v", err)
	}
	if _, err := c.Store.Messages.Exec(`INSERT INTO messages (id, chat_jid, sender, sender_name, content, timestamp, is_from_me) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, chatJID, sender, senderName, "hi", time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC), false); err != nil {
		t.Fatalf("insert message: %v", err)
	}
}

func senderNameOf(t *testing.T, c *Client, id string) string {
	t.Helper()
	var name sql.NullString
	if err := c.Store.Messages.QueryRow(`SELECT sender_name FROM messages WHERE id = ?`, id).Scan(&name); err != nil {
		t.Fatalf("read sender_name: %v", err)
	}
	return name.String
}

func TestPhoneJIDForMapsLIDViaStore(t *testing.T) {
	c := newNameTestClient(t)
	if err := c.Store.StoreLIDMapping(testLID, testPhone, ""); err != nil {
		t.Fatalf("store mapping: %v", err)
	}

	pn, ok := c.phoneJIDFor(types.JID{User: testLID, Server: types.HiddenUserServer})
	if !ok || pn.String() != testPhone+"@s.whatsapp.net" {
		t.Fatalf("expected %s@s.whatsapp.net, got %s (ok=%v)", testPhone, pn, ok)
	}

	if _, ok := c.phoneJIDFor(types.JID{User: "999", Server: types.HiddenUserServer}); ok {
		t.Fatalf("expected unmapped LID to be unresolved")
	}
	if _, ok := c.phoneJIDFor(types.JID{User: "1203", Server: types.GroupServer}); ok {
		t.Fatalf("expected group JID to be rejected")
	}
}

func TestSenderJIDFromUserPrefersKnownLID(t *testing.T) {
	c := newNameTestClient(t)
	_ = c.Store.StoreLIDMapping(testLID, testPhone, "")

	if got := c.senderJIDFromUser(testLID); got.Server != types.HiddenUserServer {
		t.Fatalf("expected LID server for mapped LID, got %s", got)
	}
	if got := c.senderJIDFromUser(testPhone); got.Server != types.DefaultUserServer {
		t.Fatalf("expected phone server for unmapped number, got %s", got)
	}
}

func TestResolveSenderNameIgnoresNumericPlaceholderChat(t *testing.T) {
	c := newNameTestClient(t)
	_ = c.Store.StoreLIDMapping(testLID, testPhone, "")
	// Older versions created this placeholder chat named after the LID itself.
	if _, err := c.Store.Messages.Exec(`INSERT INTO chats (jid, name) VALUES (?, ?)`, testLID+"@s.whatsapp.net", testLID); err != nil {
		t.Fatalf("insert placeholder chat: %v", err)
	}

	lidJID := types.JID{User: testLID, Server: types.HiddenUserServer}
	if got := c.resolveSenderName(testLID, lidJID, "Ana"); got != "Ana" {
		t.Fatalf("expected push name, got %q", got)
	}
	if got := c.resolveSenderName(testLID, lidJID, ""); got != testPhone {
		t.Fatalf("expected phone number fallback, got %q", got)
	}
	if got := c.resolveSenderName("999", types.JID{User: "999", Server: types.HiddenUserServer}, ""); got != "" {
		t.Fatalf("expected empty name for unmapped LID, got %q", got)
	}
}

func TestBackfillSenderNamesRepairsNumericNames(t *testing.T) {
	c := newNameTestClient(t)
	_ = c.Store.StoreLIDMapping(testLID, testPhone, "")
	_ = c.Store.StoreLIDMapping("200000000000001", "15550000001", "Bea")

	insertMessage(t, c, "lid-echo", groupJID, testLID, testLID)
	insertMessage(t, c, "lid-null", groupJID, testLID, nil)
	insertMessage(t, c, "named-lid", groupJID, "200000000000001", "200000000000001")
	insertMessage(t, c, "real-name", groupJID, testLID, "Ana")
	insertMessage(t, c, "unmapped", groupJID, "999", "999")

	c.backfillSenderNames()

	for id, want := range map[string]string{
		"lid-echo":  testPhone,
		"lid-null":  testPhone,
		"named-lid": "Bea",
		"real-name": "Ana",
		"unmapped":  "999",
	} {
		if got := senderNameOf(t, c, id); got != want {
			t.Errorf("%s: expected sender_name %q, got %q", id, want, got)
		}
	}
}

func TestPruneLIDSenderChatsRemovesOnlyEmptyPlaceholders(t *testing.T) {
	c := newNameTestClient(t)
	_ = c.Store.StoreLIDMapping(testLID, testPhone, "")
	_ = c.Store.StoreLIDMapping("200000000000001", "15550000001", "")

	placeholder := testLID + "@s.whatsapp.net"
	withMessages := "200000000000001@s.whatsapp.net"
	realChat := testPhone + "@s.whatsapp.net"
	for _, jid := range []string{placeholder, withMessages, realChat} {
		if _, err := c.Store.Messages.Exec(`INSERT INTO chats (jid, name) VALUES (?, ?)`, jid, "x"); err != nil {
			t.Fatalf("insert chat: %v", err)
		}
	}
	insertMessage(t, c, "keep", withMessages, "200000000000001", "")

	c.pruneLIDSenderChats()

	remaining := map[string]bool{}
	rows, err := c.Store.Messages.Query(`SELECT jid FROM chats`)
	if err != nil {
		t.Fatalf("query chats: %v", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var jid string
		_ = rows.Scan(&jid)
		remaining[jid] = true
	}

	if remaining[placeholder] {
		t.Errorf("expected empty LID placeholder chat to be removed")
	}
	if !remaining[withMessages] || !remaining[realChat] {
		t.Errorf("expected chats with messages and real phone chats to remain, got %v", remaining)
	}
}

func TestEnsureSenderChatUsesPhoneJID(t *testing.T) {
	c := newNameTestClient(t)
	_ = c.Store.StoreLIDMapping(testLID, testPhone, "")

	c.ensureSenderChat(types.JID{User: testLID, Server: types.HiddenUserServer})
	c.ensureSenderChat(types.JID{User: "999", Server: types.HiddenUserServer})

	var count int
	_ = c.Store.Messages.QueryRow(`SELECT COUNT(*) FROM chats WHERE jid = ?`, testPhone+"@s.whatsapp.net").Scan(&count)
	if count != 1 {
		t.Fatalf("expected chat keyed by phone JID, got %d", count)
	}
	_ = c.Store.Messages.QueryRow(`SELECT COUNT(*) FROM chats WHERE jid LIKE ?`, "%"+testLID+"%").Scan(&count)
	if count != 0 {
		t.Fatalf("expected no chat keyed by LID user, got %d", count)
	}
	_ = c.Store.Messages.QueryRow(`SELECT COUNT(*) FROM chats WHERE jid LIKE '999@%'`).Scan(&count)
	if count != 0 {
		t.Fatalf("expected unmapped LID to be skipped, got %d", count)
	}
}
