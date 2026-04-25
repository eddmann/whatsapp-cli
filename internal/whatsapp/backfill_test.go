package whatsapp

import (
	"testing"
	"time"

	"github.com/eddmann/whatsapp-cli/internal/store"
)

func TestBuildBackfillAnchorUsesOldestStoredMessage(t *testing.T) {
	oldest := store.Message{
		ID:        "anchor-id",
		ChatJID:   "12345@s.whatsapp.net",
		Sender:    "67890",
		Timestamp: time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC),
		IsFromMe:  false,
	}

	anchor, err := buildBackfillAnchor(oldest)
	if err != nil {
		t.Fatalf("build anchor: %v", err)
	}

	if anchor.ID != "anchor-id" {
		t.Fatalf("expected ID anchor-id, got %q", anchor.ID)
	}
	if anchor.Chat.String() != "12345@s.whatsapp.net" {
		t.Fatalf("expected chat JID, got %s", anchor.Chat.String())
	}
	if !anchor.Timestamp.Equal(oldest.Timestamp) {
		t.Fatalf("expected timestamp %s, got %s", oldest.Timestamp, anchor.Timestamp)
	}
	if anchor.IsFromMe {
		t.Fatalf("expected incoming anchor to preserve is_from_me=false")
	}
}

func TestBuildBackfillAnchorRejectsMissingMessageID(t *testing.T) {
	_, err := buildBackfillAnchor(store.Message{ChatJID: "12345@s.whatsapp.net"})
	if err == nil {
		t.Fatalf("expected missing message ID error")
	}
}
