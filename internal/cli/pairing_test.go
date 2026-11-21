package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

func lines(buf *bytes.Buffer) []string {
	return strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
}

func TestPairingEventsWriteOneJSONObjectPerLine(t *testing.T) {
	var buf bytes.Buffer
	events := newPairingEvents(&buf)

	events.qr("2@abc,def", 60*time.Second)
	events.authenticated("447700900123", 7)
	events.syncComplete()

	want := []string{
		`{"event":"qr","code":"2@abc,def","expires_in":60}`,
		`{"event":"authenticated","user":"447700900123","device":7}`,
		`{"event":"sync_complete"}`,
	}
	got := lines(&buf)
	if len(got) != len(want) {
		t.Fatalf("expected %d lines, got %d: %q", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d: expected %s, got %s", i, want[i], got[i])
		}
	}
}

func TestPairingEventsReportFailureReason(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{&whatsapp.PairingError{Event: "timeout"}, `{"event":"error","reason":"timeout","error":"pairing failed: QR code timed out"}`},
		{fmt.Errorf("wrapped: %w", &whatsapp.PairingError{Event: "err-client-outdated"}), `{"event":"error","reason":"err-client-outdated","error":"wrapped: pairing failed: err-client-outdated"}`},
		{context.Canceled, `{"event":"error","reason":"interrupted","error":"context canceled"}`},
		{errors.New("dial failed"), `{"event":"error","reason":"error","error":"dial failed"}`},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		newPairingEvents(&buf).failed(c.err)
		if got := strings.TrimRight(buf.String(), "\n"); got != c.want {
			t.Fatalf("expected %s, got %s", c.want, got)
		}
	}
}

func TestPairingEventsReportSyncTimeoutAndInterrupt(t *testing.T) {
	var buf bytes.Buffer
	events := newPairingEvents(&buf)
	events.syncTimeout()
	events.interrupted()

	got := lines(&buf)
	if got[0] != `{"event":"sync_timeout"}` || got[1] != `{"event":"interrupted"}` {
		t.Fatalf("unexpected lines %q", got)
	}
}
