package whatsapp

import (
	"context"
	"fmt"
	"time"

	"go.mau.fi/whatsmeow"
	waHistorySync "go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/types/events"
)

const syncCompletionSettleDelay = 5 * time.Second

// registerHandlers registers event handlers for WhatsApp events.
func (c *Client) registerHandlers() {
	c.WA.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			c.handleMessage(v)
		case *events.HistorySync:
			result := c.handleHistorySync(v)
			if v.Data != nil && v.Data.GetSyncType() == waHistorySync.HistorySync_ON_DEMAND {
				c.completeBackfillRequest(result)
			}
			// Check if sync is complete (progress == 100)
			if v.Data != nil && v.Data.Progress != nil && *v.Data.Progress >= 100 {
				c.Logger.Info("history sync complete")
				c.backfillChatNames()
				c.signalSyncCompleteAfterSettleDelay()
			}
		case *events.OfflineSyncCompleted:
			c.backfillChatNames()
			count, _ := c.Store.CountMessages()
			if count > 0 {
				c.signalSyncCompleteAfterSettleDelay()
			}
		case *events.Connected:
			c.Logger.Info("connected to WhatsApp")
		case *events.LoggedOut:
			c.Logger.Warn("logged out of WhatsApp")
		}
	})
}

func (c *Client) signalSyncCompleteAfterSettleDelay() {
	c.syncCompleteMu.Lock()
	defer c.syncCompleteMu.Unlock()

	if c.syncCompleteTimer != nil {
		c.syncCompleteTimer.Stop()
	}

	c.syncCompleteTimer = time.AfterFunc(syncCompletionSettleDelay, func() {
		if !c.WA.IsConnected() {
			return
		}

		select {
		case c.SyncComplete <- struct{}{}:
		default:
		}
	})
}

// QRHandler receives each pairing QR code and how long it stays valid.
type QRHandler func(code string, timeout time.Duration)

// PairingError reports a QR pairing that ended without success.
type PairingError struct {
	// Event is the QR channel event that ended the pairing, e.g. "timeout" or "err-client-outdated".
	Event string
	Err   error
}

func (e *PairingError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("pairing failed (%s): %v", e.Event, e.Err)
	}
	if e.Event == whatsmeow.QRChannelTimeout.Event {
		return "pairing failed: QR code timed out"
	}
	return fmt.Sprintf("pairing failed: %s", e.Event)
}

func (e *PairingError) Unwrap() error { return e.Err }

// ConnectWithQR connects to WhatsApp. When the device is not paired yet, it
// passes each QR code to onQR and returns once pairing succeeds, or a
// *PairingError when pairing ends without success.
func (c *Client) ConnectWithQR(ctx context.Context, onQR QRHandler) error {
	if c.WA.Store.ID == nil {
		qrChan, _ := c.WA.GetQRChannel(ctx)
		if err := c.WA.Connect(); err != nil {
			return err
		}
		return waitForPairing(ctx, qrChan, onQR)
	}

	return c.WA.Connect()
}

// waitForPairing reads the QR channel until pairing succeeds or fails.
func waitForPairing(ctx context.Context, qrChan <-chan whatsmeow.QRChannelItem, onQR QRHandler) error {
	for {
		var evt whatsmeow.QRChannelItem
		select {
		case <-ctx.Done():
			return ctx.Err()
		case item, ok := <-qrChan:
			if !ok {
				// Closed without a final event
				return &PairingError{Event: "closed"}
			}
			evt = item
		}
		switch evt.Event {
		case whatsmeow.QRChannelEventCode:
			if onQR != nil {
				onQR(evt.Code, evt.Timeout)
			}
		case whatsmeow.QRChannelSuccess.Event:
			return nil
		case whatsmeow.QRChannelScannedWithoutMultidevice.Event:
			// Not final: the phone can enable multi-device and scan the next code.
		default:
			return &PairingError{Event: evt.Event, Err: evt.Error}
		}
	}
}

// Connect connects to WhatsApp without QR (requires existing session).
func (c *Client) Connect() error {
	return c.WA.Connect()
}
