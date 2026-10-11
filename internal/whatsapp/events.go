package whatsapp

import (
	"time"

	"go.mau.fi/whatsmeow/types/events"
)

// MessageDescription is a serializable summary of a live message event,
// for consumers that watch the event stream without touching the store.
type MessageDescription struct {
	ID         string    `json:"id"`
	ChatJID    string    `json:"chat_jid"`
	Sender     string    `json:"sender"`
	SenderAlt  string    `json:"sender_alt,omitempty"`
	SenderName string    `json:"sender_name,omitempty"`
	PushName   string    `json:"push_name,omitempty"`
	FromMe     bool      `json:"from_me"`
	Timestamp  time.Time `json:"timestamp"`
	Type       string    `json:"type,omitempty"`
	Content    string    `json:"content,omitempty"`
	// DeviceSent is true when this is a copy of a message sent from
	// another one of the user's own linked devices (phone, desktop,
	// another CLI instance). On a dedicated watcher device these
	// copies are the reliable outbound oracle.
	DeviceSent bool `json:"device_sent,omitempty"`
}

// DescribeMessage summarizes a live events.Message without persisting anything.
func (c *Client) DescribeMessage(msg *events.Message) MessageDescription {
	content := extractTextContent(msg.Message)
	mediaType, _, _, _, _, _, _ := extractMediaInfo(msg.Message)

	d := MessageDescription{
		ID:        string(msg.Info.ID),
		ChatJID:   msg.Info.Chat.String(),
		Sender:    msg.Info.Sender.String(),
		PushName:  msg.Info.PushName,
		FromMe:    msg.Info.IsFromMe,
		Timestamp: msg.Info.Timestamp,
		Type:      mediaType,
		Content:   content,
	}

	if !msg.Info.SenderAlt.IsEmpty() {
		d.SenderAlt = msg.Info.SenderAlt.String()
	}
	if msg.Info.DeviceSentMeta != nil {
		d.DeviceSent = true
	}
	if name := c.resolveSenderName(msg.Info.Sender.User, msg.Info.Sender, msg.Info.PushName); name != "" {
		d.SenderName = name
	}

	return d
}
