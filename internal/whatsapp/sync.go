package whatsapp

import (
	"database/sql"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// handleMessage processes real-time incoming messages and persists them.
func (c *Client) handleMessage(msg *events.Message) {
	chatJID := msg.Info.Chat.String()
	sender := msg.Info.Sender.User
	content := extractTextContent(msg.Message)
	mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength := extractMediaInfo(msg.Message)

	if content == "" && mediaType == "" {
		return
	}

	// Resolve sender name
	senderName := c.resolveSenderName(sender, msg.Info.Sender, msg.Info.PushName)

	// Ensure we have a per-sender chat entry
	if sender != "" {
		indiv := types.JID{User: sender, Server: "s.whatsapp.net"}
		var existing sql.NullString
		_ = c.Store.Messages.QueryRow("SELECT name FROM chats WHERE jid = ?", indiv.String()).Scan(&existing)
		if !existing.Valid {
			resolved := c.resolvePreferredName(indiv.String())
			_, _ = c.Store.Messages.Exec("INSERT INTO chats (jid, name) VALUES (?, ?)", indiv.String(), resolved)
		} else if existing.String == "" {
			resolved := c.resolvePreferredName(indiv.String())
			if resolved != "" {
				_, _ = c.Store.Messages.Exec("UPDATE chats SET name = ? WHERE jid = ?", resolved, indiv.String())
			}
		}
	}

	name := c.getChatName(msg.Info.Chat.String(), chatJID, nil, sender)
	if _, err := c.Store.Messages.Exec("INSERT OR REPLACE INTO chats (jid, name, last_message_time) VALUES (?, ?, ?)", chatJID, name, msg.Info.Timestamp); err != nil {
		c.Logger.Warn("failed to upsert chat", "jid", chatJID, "err", err)
	}

	if _, err := c.Store.Messages.Exec(`INSERT OR REPLACE INTO messages
		(id, chat_jid, sender, sender_name, content, timestamp, is_from_me, media_type, filename, url, media_key, file_sha256, file_enc_sha256, file_length)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		msg.Info.ID, chatJID, sender, senderName, content, msg.Info.Timestamp, msg.Info.IsFromMe, mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength,
	); err != nil {
		c.Logger.Warn("failed to store message", "id", msg.Info.ID, "chat_jid", chatJID, "err", err)
	}
}

// handleHistorySync persists conversations and messages received during a history sync.
func (c *Client) handleHistorySync(hs *events.HistorySync) {
	if hs == nil || hs.Data.Conversations == nil {
		return
	}

	synced := 0
	for _, conv := range hs.Data.Conversations {
		if conv == nil || conv.ID == nil {
			continue
		}

		chatJID := *conv.ID
		jid, err := types.ParseJID(chatJID)
		if err != nil {
			c.Logger.Warn("history sync: bad JID", "jid", chatJID, "err", err)
			continue
		}

		name := c.getChatName(jid.String(), chatJID, conv, "")

		if len(conv.Messages) > 0 && conv.Messages[0] != nil && conv.Messages[0].Message != nil {
			ts := conv.Messages[0].Message.GetMessageTimestamp()
			if ts != 0 {
				t := time.Unix(int64(ts), 0)
				if _, err := c.Store.Messages.Exec("INSERT OR REPLACE INTO chats (jid, name, last_message_time) VALUES (?, ?, ?)", chatJID, name, t); err != nil {
					c.Logger.Warn("history sync: failed to upsert chat", "jid", chatJID, "err", err)
				}
			}
		}

		for _, m := range conv.Messages {
			if m == nil || m.Message == nil {
				continue
			}

			var text string
			if m.Message.Message != nil {
				text = extractTextContent(m.Message.Message)
			}

			mt, fn, u, mk, sha, enc, fl := "", "", "", ([]byte)(nil), ([]byte)(nil), ([]byte)(nil), uint64(0)
			if m.Message.Message != nil {
				mt, fn, u, mk, sha, enc, fl = extractMediaInfo(m.Message.Message)
			}

			if text == "" && mt == "" {
				continue
			}

			fromMe := false
			snd := jid.User
			if m.Message.Key != nil {
				if m.Message.Key.FromMe != nil {
					fromMe = *m.Message.Key.FromMe
				}
				if !fromMe && m.Message.Key.Participant != nil && *m.Message.Key.Participant != "" {
					snd = *m.Message.Key.Participant
				}
				if fromMe && c.WA != nil && c.WA.Store != nil && c.WA.Store.ID != nil {
					snd = c.WA.Store.ID.User
				}
			}

			if strings.Contains(snd, "@") {
				if pj, err := types.ParseJID(snd); err == nil {
					snd = pj.User
				} else {
					if i := strings.Index(snd, "@"); i > 0 {
						snd = snd[:i]
					}
				}
			}

			// Upsert a per-sender chat entry
			if !fromMe && snd != "" {
				indiv := types.JID{User: snd, Server: "s.whatsapp.net"}
				var existing sql.NullString
				_ = c.Store.Messages.QueryRow("SELECT name FROM chats WHERE jid = ?", indiv.String()).Scan(&existing)
				if !existing.Valid {
					resolved := c.resolvePreferredName(indiv.String())
					_, _ = c.Store.Messages.Exec("INSERT INTO chats (jid, name) VALUES (?, ?)", indiv.String(), resolved)
				} else if existing.String == "" {
					resolved := c.resolvePreferredName(indiv.String())
					if resolved != "" {
						_, _ = c.Store.Messages.Exec("UPDATE chats SET name = ? WHERE jid = ?", resolved, indiv.String())
					}
				}
			}

			id := ""
			if m.Message.Key != nil && m.Message.Key.ID != nil {
				id = *m.Message.Key.ID
			}

			ts := m.Message.GetMessageTimestamp()
			if ts == 0 {
				continue
			}
			t := time.Unix(int64(ts), 0)

			// Resolve sender name for history sync messages
			senderName := c.Store.ResolveSenderName(snd)
			if senderName == "" {
				// Try to resolve from phone-based JID
				phoneJID := types.JID{User: snd, Server: "s.whatsapp.net"}
				senderName = c.resolvePreferredName(phoneJID.String())
			}

			if _, err := c.Store.Messages.Exec(`INSERT OR REPLACE INTO messages
				(id, chat_jid, sender, sender_name, content, timestamp, is_from_me, media_type, filename, url, media_key, file_sha256, file_enc_sha256, file_length)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, chatJID, snd, senderName, text, t, fromMe, mt, fn, u, mk, sha, enc, fl); err != nil {
				c.Logger.Warn("history sync: failed to store message", "id", id, "chat_jid", chatJID, "err", err)
				continue
			}
			synced++
		}
	}

	c.Logger.Info("history sync persisted messages", "count", synced)
}

// backfillChatNames finds chats without a proper name and updates them.
func (c *Client) backfillChatNames() {
	if c.Store == nil || c.Store.Messages == nil {
		return
	}

	rows, err := c.Store.Messages.Query(`SELECT jid, COALESCE(name, '') FROM chats`)
	if err != nil {
		c.Logger.Warn("backfill: query chats failed", "err", err)
		return
	}
	defer func() { _ = rows.Close() }()

	type row struct {
		jid  string
		name string
	}
	var toUpdate []row

	for rows.Next() {
		var jidStr, name string
		if err := rows.Scan(&jidStr, &name); err != nil {
			c.Logger.Warn("backfill: scan failed", "err", err)
			continue
		}

		parsed, err := types.ParseJID(jidStr)
		if err != nil {
			continue
		}

		if parsed.Server == "g.us" {
			if name == "" || name == parsed.User {
				toUpdate = append(toUpdate, row{jid: jidStr, name: name})
			}
			continue
		}

		phone := parsed.User
		if name == "" || name == phone || strings.HasSuffix(name, "@s.whatsapp.net") {
			toUpdate = append(toUpdate, row{jid: jidStr, name: name})
		}
	}

	updated := 0
	for _, r := range toUpdate {
		resolved := c.resolvePreferredName(r.jid)
		if resolved == "" || resolved == r.name {
			continue
		}

		if _, err := c.Store.Messages.Exec(`UPDATE chats SET name = ? WHERE jid = ?`, resolved, r.jid); err != nil {
			c.Logger.Warn("backfill: update failed", "jid", r.jid, "err", err)
			continue
		}
		updated++
	}

	if updated > 0 {
		c.Logger.Info("backfill: updated chat names", "count", updated)
	}
}
