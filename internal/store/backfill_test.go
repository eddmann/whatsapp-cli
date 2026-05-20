package store

import (
	"database/sql"
	"errors"
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
	if _, err := db.Messages.Exec(`INSERT INTO chats (jid, name) VALUES (?, ?)`, chatJID, "Test Chat"); err != nil {
		t.Fatalf("insert chat: %v", err)
	}

	newer := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	older := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	if _, err := db.Messages.Exec(`INSERT INTO messages (id, chat_jid, sender, content, timestamp, is_from_me) VALUES (?, ?, ?, ?, ?, ?)`,
		"newer-id", chatJID, "12345", "newer", newer, false); err != nil {
		t.Fatalf("insert newer message: %v", err)
	}
	if _, err := db.Messages.Exec(`INSERT INTO messages (id, chat_jid, sender, content, timestamp, is_from_me) VALUES (?, ?, ?, ?, ?, ?)`,
		"older-id", chatJID, "12345", "older", older, true); err != nil {
		t.Fatalf("insert older message: %v", err)
	}

	message, err := db.OldestMessageForChat(chatJID)
	if err != nil {
		t.Fatalf("oldest message: %v", err)
	}

	if message.ID != "older-id" {
		t.Fatalf("expected older-id, got %q", message.ID)
	}
	if !message.Timestamp.Equal(older) {
		t.Fatalf("expected timestamp %s, got %s", older, message.Timestamp)
	}
	if !message.IsFromMe {
		t.Fatalf("expected is_from_me=true for oldest message")
	}
}

func TestOldestMessageForChatReturnsNoRowsWhenChatHasNoMessages(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "messages.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.CloseQuietly()

	_, err = db.OldestMessageForChat("missing@s.whatsapp.net")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}
