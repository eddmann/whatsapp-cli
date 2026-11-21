package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

// pairingEvent is one line of `auth login --format jsonl`.
type pairingEvent struct {
	Event string `json:"event"`
	// qr
	Code      string `json:"code,omitempty"`
	ExpiresIn int    `json:"expires_in,omitempty"` // seconds the code stays valid
	// authenticated
	User   string `json:"user,omitempty"`
	Device uint16 `json:"device,omitempty"`
	// error
	Reason string `json:"reason,omitempty"`
	Error  string `json:"error,omitempty"`
}

// pairingEvents writes the progress of `auth login` as JSON Lines, so a
// program can render the QR code itself and follow the pairing's outcome.
type pairingEvents struct {
	enc *json.Encoder
}

func newPairingEvents(w io.Writer) *pairingEvents {
	return &pairingEvents{enc: json.NewEncoder(w)}
}

func (p *pairingEvents) emit(evt pairingEvent) {
	_ = p.enc.Encode(evt)
}

// qr reports a QR code to show; the next one replaces it once it expires.
func (p *pairingEvents) qr(code string, timeout time.Duration) {
	p.emit(pairingEvent{Event: "qr", Code: code, ExpiresIn: int(timeout.Seconds())})
}

// authenticated reports the linked device, once paired (or already paired).
func (p *pairingEvents) authenticated(user string, device uint16) {
	p.emit(pairingEvent{Event: "authenticated", User: user, Device: device})
}

func (p *pairingEvents) syncComplete() { p.emit(pairingEvent{Event: "sync_complete"}) }

func (p *pairingEvents) syncTimeout() { p.emit(pairingEvent{Event: "sync_timeout"}) }

func (p *pairingEvents) interrupted() { p.emit(pairingEvent{Event: "interrupted"}) }

// failed reports a pairing or connection that ended without success. The
// reason is the QR channel's event (e.g. "timeout"), "interrupted", or "error".
func (p *pairingEvents) failed(err error) {
	reason := "error"
	var pairingErr *whatsapp.PairingError
	switch {
	case errors.As(err, &pairingErr):
		reason = pairingErr.Event
	case errors.Is(err, context.Canceled):
		reason = "interrupted"
	}
	p.emit(pairingEvent{Event: "error", Reason: reason, Error: err.Error()})
}
