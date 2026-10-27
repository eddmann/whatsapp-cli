package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestOldestMessageForChatAfterSkipsHistoryBeforeGap(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "messages.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.CloseQuietly()

	chatJID := "120363000000000000@g.us"
	if _, err := db.Messages.Exec(`INSERT INTO chats (jid, name) VALUES (?, ?)`, chatJID, "Group"); err != nil {
		t.Fatalf("insert chat: %v", err)
	}

	lisbon := time.FixedZone("WEST", 60*60)
	central := time.FixedZone("CDT", -5*60*60)
	for id, ts := range map[string]time.Time{
		"before-gap": time.Date(2026, 4, 20, 12, 0, 0, 0, lisbon),
		"after-gap":  time.Date(2026, 8, 2, 9, 0, 0, 0, central),
		"newest":     time.Date(2026, 10, 1, 9, 0, 0, 0, central),
	} {
		if _, err := db.Messages.Exec(`INSERT INTO messages (id, chat_jid, sender, content, timestamp, is_from_me) VALUES (?, ?, ?, ?, ?, ?)`,
			id, chatJID, "1", id, ts, false); err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
	}

	got, err := db.OldestMessageForChatAfter(chatJID, time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("oldest after: %v", err)
	}
	if got.ID != "after-gap" {
		t.Fatalf("expected after-gap, got %q", got.ID)
	}

	if _, err := db.OldestMessageForChatAfter(chatJID, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows after newest message, got %v", err)
	}
}
