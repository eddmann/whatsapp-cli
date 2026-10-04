package whatsapp

import (
	"context"
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// Activity is a chat presence update: someone in a chat is typing, recording a
// voice note, or has stopped. WhatsApp sends these as <chatstate> nodes, which
// whatsmeow surfaces as events.ChatPresence. They carry no timestamp, so the
// time is when this client received it.
type Activity struct {
	ChatJID string `json:"chat_jid"`
	// ChatAlt is the same one-to-one chat under its other address (@lid for a
	// phone JID, or the reverse), when known. A chatstate and the messages of the
	// same person do not always use the same form, so filters match either.
	ChatAlt    string    `json:"chat_alt,omitempty"`
	Sender     string    `json:"sender"`
	SenderName string    `json:"sender_name,omitempty"`
	State      string    `json:"state"`           // "composing" or "paused"
	Media      string    `json:"media,omitempty"` // "audio" while recording a voice note
	Timestamp  time.Time `json:"timestamp"`
	IsFromMe   bool      `json:"is_from_me"`
}

// handleChatPresence converts a whatsmeow chat presence event and hands it to
// OnActivity, when set.
func (c *Client) handleChatPresence(evt *events.ChatPresence) {
	if c.OnActivity == nil {
		return
	}
	sender := evt.Sender.User
	c.OnActivity(Activity{
		ChatJID:    evt.Chat.String(),
		ChatAlt:    c.altChatJID(evt.Chat),
		Sender:     sender,
		SenderName: c.resolveSenderName(sender, evt.Sender, ""),
		State:      string(evt.State),
		Media:      string(evt.Media),
		Timestamp:  time.Now(),
		IsFromMe:   evt.IsFromMe,
	})
}

// altChatJID returns the other address of a one-to-one chat (@lid <-> phone),
// or "" for groups and unknown mappings.
func (c *Client) altChatJID(chat types.JID) string {
	if c.WA == nil || c.WA.Store == nil || c.WA.Store.LIDs == nil {
		return ""
	}
	ctx := context.Background()
	var alt types.JID
	var err error
	switch chat.Server {
	case types.DefaultUserServer:
		alt, err = c.WA.Store.LIDs.GetLIDForPN(ctx, chat.ToNonAD())
	case types.HiddenUserServer:
		alt, err = c.WA.Store.LIDs.GetPNForLID(ctx, chat.ToNonAD())
	default:
		return ""
	}
	if err != nil || alt.IsEmpty() {
		return ""
	}
	return alt.ToNonAD().String()
}
