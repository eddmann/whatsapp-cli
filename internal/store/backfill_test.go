package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestOldestMessageForChatReturnsEarliestMessage(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "messages.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.CloseQuietly()

	chatJID := "12345@s.whatsapp.net"
	_, err = db.Messages.Exec(`INSERT INTO chats (jid, name) VALUES (?, ?)`, chatJID, "Test Chat")
	if err != nil {
		t.Fatalf("insert chat: %v", err)
	}

	newer := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	older := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	_, err = db.Messages.Exec(`INSERT INTO messages (id, chat_jid, sender, content, timestamp, is_from_me) VALUES (?, ?, ?, ?, ?, ?)`,
		"newer-id", chatJID, "12345", "newer", newer, false)
	if err != nil {
		t.Fatalf("insert newer message: %v", err)
	}
	_, err = db.Messages.Exec(`INSERT INTO messages (id, chat_jid, sender, content, timestamp, is_from_me) VALUES (?, ?, ?, ?, ?, ?)`,
		"older-id", chatJID, "12345", "older", older, true)
	if err != nil {
		t.Fatalf("insert older message: %v", err)
	}

	msg, err := db.OldestMessageForChat(chatJID)
	if err != nil {
		t.Fatalf("oldest message: %v", err)
	}

	if msg.ID != "older-id" {
		t.Fatalf("expected oldest-id, got %q", msg.ID)
	}
	if !msg.Timestamp.Equal(older) {
		t.Fatalf("expected timestamp %s, got %s", older, msg.Timestamp)
	}
	if !msg.IsFromMe {
		t.Fatalf("expected is_from_me=true for oldest message")
	}
}

func TestOldestMessageForChatReturnsErrorWhenNoAnchorExists(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "messages.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.CloseQuietly()

	_, err = db.OldestMessageForChat("missing@s.whatsapp.net")
	if err == nil {
		t.Fatalf("expected error when chat has no messages")
	}
}
