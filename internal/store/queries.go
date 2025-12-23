package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ListChats returns chats matching the given options.
func (d *DB) ListChats(opts ListChatsOptions) ([]Chat, error) {
	query := `
		SELECT c.jid, c.name, c.last_message_time,
			(SELECT content FROM messages WHERE chat_jid = c.jid ORDER BY timestamp DESC LIMIT 1) as last_message,
			(SELECT sender FROM messages WHERE chat_jid = c.jid ORDER BY timestamp DESC LIMIT 1) as last_sender,
			(SELECT is_from_me FROM messages WHERE chat_jid = c.jid ORDER BY timestamp DESC LIMIT 1) as last_is_from_me
		FROM chats c
		WHERE 1=1
	`
	var args []any

	if opts.Query != "" {
		query += " AND (LOWER(c.name) LIKE ? OR c.jid LIKE ?)"
		pattern := "%" + strings.ToLower(opts.Query) + "%"
		args = append(args, pattern, pattern)
	}

	if opts.OnlyGroups {
		query += " AND c.jid LIKE '%@g.us'"
	}

	query += " ORDER BY c.last_message_time DESC NULLS LAST"

	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}

	rows, err := d.Messages.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var chats []Chat
	for rows.Next() {
		var jid string
		var name sql.NullString
		var lastTime sql.NullTime
		var lastMsg, lastSender sql.NullString
		var lastFromMe sql.NullBool

		if err := rows.Scan(&jid, &name, &lastTime, &lastMsg, &lastSender, &lastFromMe); err != nil {
			continue
		}

		chat := Chat{
			JID:     jid,
			IsGroup: strings.HasSuffix(jid, "@g.us"),
		}

		if name.Valid && name.String != "" {
			chat.Name = &name.String
		}
		if lastTime.Valid {
			chat.LastMessageTime = &lastTime.Time
		}
		if lastMsg.Valid {
			chat.LastMessage = &lastMsg.String
		}
		if lastSender.Valid {
			chat.LastSender = &lastSender.String
		}
		if lastFromMe.Valid {
			chat.LastIsFromMe = &lastFromMe.Bool
		}

		chats = append(chats, chat)
	}

	return chats, nil
}

// ListMessages returns messages matching the given options.
func (d *DB) ListMessages(opts ListMessagesOptions) ([]Message, error) {
	query := `
		SELECT m.id, m.chat_jid, m.sender,
		       COALESCE(m.sender_name, l.name) as sender_name,
		       m.content, m.timestamp, m.is_from_me,
		       m.media_type, m.filename, c.name as chat_name
		FROM messages m
		LEFT JOIN chats c ON m.chat_jid = c.jid
		LEFT JOIN lid_mappings l ON m.sender = l.lid
		WHERE 1=1
	`
	var args []any

	if opts.ChatJID != "" {
		query += " AND m.chat_jid = ?"
		args = append(args, opts.ChatJID)
	}

	if opts.After != "" {
		afterTime, err := time.Parse(time.RFC3339, opts.After)
		if err == nil {
			query += " AND m.timestamp >= ?"
			args = append(args, afterTime)
		}
	}

	if opts.Before != "" {
		beforeTime, err := time.Parse(time.RFC3339, opts.Before)
		if err == nil {
			query += " AND m.timestamp <= ?"
			args = append(args, beforeTime)
		}
	}

	if opts.Type != "" {
		switch opts.Type {
		case "text":
			query += " AND (m.media_type IS NULL OR m.media_type = '')"
		case "image", "video", "audio", "document", "sticker":
			query += " AND m.media_type = ?"
			args = append(args, opts.Type)
		}
	}

	query += " ORDER BY m.timestamp DESC"

	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}

	return d.scanMessages(query, args)
}

// SearchMessages performs full-text search on messages.
func (d *DB) SearchMessages(opts SearchMessagesOptions) ([]Message, error) {
	query := `
		SELECT m.id, m.chat_jid, m.sender,
		       COALESCE(m.sender_name, l.name) as sender_name,
		       m.content, m.timestamp, m.is_from_me,
		       m.media_type, m.filename, c.name as chat_name
		FROM messages m
		JOIN messages_fts fts ON m.rowid = fts.rowid
		LEFT JOIN chats c ON m.chat_jid = c.jid
		LEFT JOIN lid_mappings l ON m.sender = l.lid
		WHERE messages_fts MATCH ?
	`
	args := []any{opts.Query}

	if opts.ChatJID != "" {
		query += " AND m.chat_jid = ?"
		args = append(args, opts.ChatJID)
	}

	if opts.FromJID != "" {
		query += " AND m.sender = ?"
		args = append(args, opts.FromJID)
	}

	if opts.After != "" {
		afterTime, err := time.Parse(time.RFC3339, opts.After)
		if err == nil {
			query += " AND m.timestamp >= ?"
			args = append(args, afterTime)
		}
	}

	if opts.Before != "" {
		beforeTime, err := time.Parse(time.RFC3339, opts.Before)
		if err == nil {
			query += " AND m.timestamp <= ?"
			args = append(args, beforeTime)
		}
	}

	if opts.Type != "" {
		switch opts.Type {
		case "text":
			query += " AND (m.media_type IS NULL OR m.media_type = '')"
		case "image", "video", "audio", "document", "sticker":
			query += " AND m.media_type = ?"
			args = append(args, opts.Type)
		}
	}

	query += " ORDER BY m.timestamp DESC"

	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}

	return d.scanMessages(query, args)
}

// GetChatName returns the name of a chat by JID.
func (d *DB) GetChatName(jid string) string {
	var name sql.NullString
	_ = d.Messages.QueryRow("SELECT name FROM chats WHERE jid = ?", jid).Scan(&name)
	if name.Valid {
		return name.String
	}
	return ""
}

// scanMessages is a helper to scan message rows into Message structs.
func (d *DB) scanMessages(query string, args []any) ([]Message, error) {
	rows, err := d.Messages.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var messages []Message
	for rows.Next() {
		var m Message
		var senderName, content, mediaType, filename, chatName sql.NullString

		if err := rows.Scan(&m.ID, &m.ChatJID, &m.Sender, &senderName, &content, &m.Timestamp, &m.IsFromMe, &mediaType, &filename, &chatName); err != nil {
			continue
		}

		if senderName.Valid && senderName.String != "" {
			m.SenderName = &senderName.String
		}
		if content.Valid {
			m.Content = &content.String
		}
		if mediaType.Valid && mediaType.String != "" {
			m.MediaType = &mediaType.String
		}
		if filename.Valid && filename.String != "" {
			m.Filename = &filename.String
		}
		if chatName.Valid {
			m.ChatName = &chatName.String
		}

		messages = append(messages, m)
	}

	return messages, nil
}
