package store

import "time"

// Chat represents a WhatsApp chat (direct message or group).
type Chat struct {
	JID             string     `json:"jid"`
	Name            *string    `json:"name,omitempty"`
	IsGroup         bool       `json:"is_group"`
	LastMessageTime *time.Time `json:"last_message_time,omitempty"`
	LastMessage     *string    `json:"last_message,omitempty"`
	LastSender      *string    `json:"last_sender,omitempty"`
	LastIsFromMe    *bool      `json:"last_is_from_me,omitempty"`
}

// Message represents a WhatsApp message.
type Message struct {
	ID         string    `json:"id"`
	ChatJID    string    `json:"chat_jid"`
	Sender     string    `json:"sender"`
	SenderName *string   `json:"sender_name,omitempty"`
	Content    *string   `json:"content,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	IsFromMe   bool      `json:"is_from_me"`
	MediaType  *string   `json:"media_type,omitempty"`
	Filename   *string   `json:"filename,omitempty"`
	ChatName   *string   `json:"chat_name,omitempty"`
}

// Contact represents a WhatsApp contact.
type Contact struct {
	JID   string  `json:"jid"`
	Phone string  `json:"phone_number"`
	Name  *string `json:"name,omitempty"`
}

// SendResult represents the result of sending a message.
type SendResult struct {
	MessageID string `json:"message_id"`
	ChatJID   string `json:"chat_jid"`
	Timestamp string `json:"timestamp"`
}

// DownloadResult represents the result of downloading media.
type DownloadResult struct {
	Filename string `json:"filename"`
	Path     string `json:"path"`
}

// ConnectionStatus represents the WhatsApp connection status.
type ConnectionStatus struct {
	Connected bool        `json:"connected"`
	LoggedIn  bool        `json:"logged_in"`
	Device    *DeviceInfo `json:"device,omitempty"`
	Database  *DBStats    `json:"database,omitempty"`
}

// DeviceInfo contains device information.
type DeviceInfo struct {
	User   string `json:"user"`
	Device uint16 `json:"device"`
}

// DBStats contains database statistics.
type DBStats struct {
	Chats    int `json:"chats"`
	Messages int `json:"messages"`
}

// GroupInfo represents WhatsApp group information.
type GroupInfo struct {
	JID          string        `json:"jid"`
	Name         string        `json:"name"`
	Topic        string        `json:"topic,omitempty"`
	Created      time.Time     `json:"created"`
	CreatorJID   string        `json:"creator_jid,omitempty"`
	Participants []Participant `json:"participants,omitempty"`
}

// Participant represents a group participant.
type Participant struct {
	JID     string  `json:"jid"`
	LID     *string `json:"lid,omitempty"`
	Phone   *string `json:"phone,omitempty"`
	IsAdmin bool    `json:"is_admin"`
	Name    string  `json:"name,omitempty"`
}

// ListChatsOptions contains options for listing chats.
type ListChatsOptions struct {
	Query      string
	OnlyGroups bool
	Limit      int
	Page       int
}

// ListMessagesOptions contains options for listing messages.
type ListMessagesOptions struct {
	After     string
	Before    string
	Timeframe string
	ChatJID   string
	Type      string
	Limit     int
	Page      int
}

// SearchMessagesOptions contains options for searching messages.
type SearchMessagesOptions struct {
	Query     string
	ChatJID   string
	FromJID   string
	After     string
	Before    string
	Timeframe string
	Type      string
	Limit     int
	Page      int
}

// ContextResult represents aggregated context for LLMs.
type ContextResult struct {
	Connection  *ConnectionStatus `json:"connection"`
	RecentChats []ChatWithRecent  `json:"recent_chats"`
}

// ChatWithRecent represents a chat with its recent messages.
type ChatWithRecent struct {
	Chat           Chat      `json:"chat"`
	RecentMessages []Message `json:"recent_messages"`
}
