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

func TestBackfillStorageChatJIDUsesRequestedJIDForSingleOnDemandResponse(t *testing.T) {
	client := &Client{}
	response, clear, err := client.beginBackfillRequest("233564700451061@lid")
	if err != nil {
		t.Fatalf("begin backfill request: %v", err)
	}
	defer clear()

	got := client.backfillStorageChatJID("6581632144@s.whatsapp.net", true, 1)
	if got != "233564700451061@lid" {
		t.Fatalf("expected requested JID to be used for storage, got %q", got)
	}

	client.completeBackfillRequest(HistorySyncResult{MessagesSynced: 3, MoreAvailable: true})

	select {
	case result := <-response:
		if result.MessagesSynced != 3 || !result.MoreAvailable {
			t.Fatalf("unexpected result: %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for backfill result")
	}
}

func TestBackfillStorageChatJIDUsesResponseJIDWithoutPendingRequest(t *testing.T) {
	client := &Client{}

	got := client.backfillStorageChatJID("6581632144@s.whatsapp.net", true, 1)
	if got != "6581632144@s.whatsapp.net" {
		t.Fatalf("expected response JID when no request pending, got %q", got)
	}
}

func TestBeginBackfillRequestRejectsConcurrentRequest(t *testing.T) {
	client := &Client{}
	_, clear, err := client.beginBackfillRequest("12345@s.whatsapp.net")
	if err != nil {
		t.Fatalf("begin first request: %v", err)
	}
	defer clear()

	_, _, err = client.beginBackfillRequest("67890@s.whatsapp.net")
	if err == nil {
		t.Fatalf("expected concurrent request error")
	}
}
