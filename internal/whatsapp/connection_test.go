package whatsapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
)

func qrEvents(items ...whatsmeow.QRChannelItem) <-chan whatsmeow.QRChannelItem {
	ch := make(chan whatsmeow.QRChannelItem, len(items))
	for _, item := range items {
		ch <- item
	}
	close(ch)
	return ch
}

func TestWaitForPairingPassesEachCodeThenSucceeds(t *testing.T) {
	var codes []string
	var timeouts []time.Duration
	err := waitForPairing(context.Background(), qrEvents(
		whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode, Code: "code-1", Timeout: 60 * time.Second},
		whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode, Code: "code-2", Timeout: 20 * time.Second},
		whatsmeow.QRChannelSuccess,
	), func(code string, timeout time.Duration) {
		codes = append(codes, code)
		timeouts = append(timeouts, timeout)
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(codes) != 2 || codes[0] != "code-1" || codes[1] != "code-2" {
		t.Fatalf("expected both codes in order, got %v", codes)
	}
	if timeouts[0] != 60*time.Second || timeouts[1] != 20*time.Second {
		t.Fatalf("expected each code's timeout, got %v", timeouts)
	}
}

func TestWaitForPairingReportsTimeout(t *testing.T) {
	err := waitForPairing(context.Background(), qrEvents(
		whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode, Code: "code-1"},
		whatsmeow.QRChannelTimeout,
	), nil)
	var pairingErr *PairingError
	if !errors.As(err, &pairingErr) || pairingErr.Event != "timeout" {
		t.Fatalf("expected a timeout PairingError, got %v", err)
	}
}

func TestWaitForPairingReportsErrorEventWithCause(t *testing.T) {
	cause := errors.New("pair rejected")
	err := waitForPairing(context.Background(), qrEvents(
		whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventError, Error: cause},
	), nil)
	var pairingErr *PairingError
	if !errors.As(err, &pairingErr) || pairingErr.Event != "error" || !errors.Is(err, cause) {
		t.Fatalf("expected an error PairingError wrapping the cause, got %v", err)
	}
}

func TestWaitForPairingKeepsWaitingAfterScanWithoutMultidevice(t *testing.T) {
	err := waitForPairing(context.Background(), qrEvents(
		whatsmeow.QRChannelScannedWithoutMultidevice,
		whatsmeow.QRChannelSuccess,
	), nil)
	if err != nil {
		t.Fatalf("expected success after a scan without multidevice, got %v", err)
	}
}

func TestWaitForPairingStopsWhenCancelledWhileTheChannelStaysOpen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	open := make(chan whatsmeow.QRChannelItem)
	done := make(chan error, 1)
	go func() { done <- waitForPairing(ctx, open, nil) }()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("waitForPairing did not return after cancellation")
	}
}

func TestWaitForPairingReportsAChannelClosedWithoutAFinalEvent(t *testing.T) {
	err := waitForPairing(context.Background(), qrEvents(
		whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode, Code: "code-1"},
	), nil)
	var pairingErr *PairingError
	if !errors.As(err, &pairingErr) || pairingErr.Event != "closed" {
		t.Fatalf("expected a closed PairingError, got %v", err)
	}
}
