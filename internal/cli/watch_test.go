package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

func TestWatchEmitterFiltersToOneChat(t *testing.T) {
	var buf bytes.Buffer
	emit := newWatchEmitter(&buf, "123@g.us")
	body := "hello"
	emit(store.Message{ID: "A", ChatJID: "123@g.us", Content: &body, Timestamp: time.Unix(1, 0)})
	emit(store.Message{ID: "B", ChatJID: "999@g.us", Content: &body, Timestamp: time.Unix(2, 0)})

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("want 1 line, got %d: %q", len(lines), buf.String())
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("line is not JSON: %v", err)
	}
	if got["id"] != "A" || got["content"] != "hello" || got["chat_jid"] != "123@g.us" {
		t.Fatalf("unexpected event: %v", got)
	}
}

func TestWatchEmitterNoFilterPassesEverything(t *testing.T) {
	var buf bytes.Buffer
	emit := newWatchEmitter(&buf, "")
	emit(store.Message{ID: "A", ChatJID: "1@g.us"})
	emit(store.Message{ID: "B", ChatJID: "2@s.whatsapp.net"})
	if n := strings.Count(buf.String(), "\n"); n != 2 {
		t.Fatalf("want 2 lines, got %d", n)
	}
}

func TestWatchActivityFilteredAndWrapped(t *testing.T) {
	var buf bytes.Buffer
	_, onActivity := newWatchEmitters(&buf, "123@g.us")
	now := time.Unix(10, 0)
	onActivity(whatsapp.Activity{ChatJID: "123@g.us", Sender: "316", State: "composing", Media: "audio", Timestamp: now})
	onActivity(whatsapp.Activity{ChatJID: "999@g.us", Sender: "316", State: "composing", Timestamp: now})
	onActivity(whatsapp.Activity{ChatJID: "123@g.us", Sender: "me", State: "composing", Timestamp: now, IsFromMe: true})

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("want 1 line, got %d: %q", len(lines), buf.String())
	}
	var got struct {
		Activity map[string]any `json:"activity"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("line is not JSON: %v", err)
	}
	a := got.Activity
	if a["chat_jid"] != "123@g.us" || a["state"] != "composing" || a["media"] != "audio" || a["sender"] != "316" {
		t.Fatalf("unexpected activity: %v", a)
	}
}

func TestWatchActivityTypingHasNoMediaField(t *testing.T) {
	var buf bytes.Buffer
	_, onActivity := newWatchEmitters(&buf, "")
	onActivity(whatsapp.Activity{ChatJID: "1@s.whatsapp.net", Sender: "1", State: "paused", Timestamp: time.Unix(1, 0)})
	if strings.Contains(buf.String(), `"media"`) {
		t.Fatalf("paused/typing line should omit media: %s", buf.String())
	}
	if !strings.HasPrefix(buf.String(), `{"activity":{`) {
		t.Fatalf("activity line not wrapped: %s", buf.String())
	}
}

func TestWatchActivityMatchesAlternateAddress(t *testing.T) {
	var buf bytes.Buffer
	_, onActivity := newWatchEmitters(&buf, "256104823034051@lid")
	onActivity(whatsapp.Activity{ChatJID: "31612345678@s.whatsapp.net", ChatAlt: "256104823034051@lid",
		Sender: "31612345678", State: "composing", Media: "audio", Timestamp: time.Unix(1, 0)})
	if n := strings.Count(buf.String(), "\n"); n != 1 {
		t.Fatalf("activity under the phone JID should match a @lid filter, got %d lines", n)
	}
}
