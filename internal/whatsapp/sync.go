package whatsapp

import (
	"database/sql"
	"strings"
	"time"

	waHistorySync "go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/eddmann/whatsapp-cli/internal/store"
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
		c.ensureSenderChat(msg.Info.Sender)
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
func (c *Client) handleHistorySync(hs *events.HistorySync) HistorySyncResult {
	if hs == nil || hs.Data == nil || hs.Data.Conversations == nil {
		return HistorySyncResult{}
	}

	synced := 0
	moreAvailable := false
	isOnDemand := hs.Data.GetSyncType() == waHistorySync.HistorySync_ON_DEMAND
	conversationCount := len(hs.Data.Conversations)

	for _, conv := range hs.Data.Conversations {
		if conv == nil || conv.ID == nil {
			continue
		}

		responseChatJID := *conv.ID
		chatJID := c.backfillStorageChatJID(responseChatJID, isOnDemand, conversationCount)
		jid, err := types.ParseJID(chatJID)
		if err != nil {
			c.Logger.Warn("history sync: bad JID", "jid", chatJID, "response_jid", responseChatJID, "err", err)
			continue
		}

		name := c.getChatName(jid.String(), chatJID, conv, "")
		endType := conv.GetEndOfHistoryTransferType()
		if endType == waHistorySync.Conversation_COMPLETE_BUT_MORE_MESSAGES_REMAIN_ON_PRIMARY ||
			endType == waHistorySync.Conversation_COMPLETE_ON_DEMAND_SYNC_BUT_MORE_MSG_REMAIN_ON_PRIMARY {
			moreAvailable = true
		}

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
			sndJID := jid
			if m.Message.Key != nil {
				if m.Message.Key.FromMe != nil {
					fromMe = *m.Message.Key.FromMe
				}
				if !fromMe && m.Message.Key.Participant != nil && *m.Message.Key.Participant != "" {
					snd = *m.Message.Key.Participant
				}
				if fromMe && c.WA != nil && c.WA.Store != nil && c.WA.Store.ID != nil {
					snd = c.WA.Store.ID.User
					sndJID = c.WA.Store.ID.ToNonAD()
				}
			}

			if strings.Contains(snd, "@") {
				if pj, err := types.ParseJID(snd); err == nil {
					snd = pj.User
					sndJID = pj.ToNonAD()
				} else {
					if i := strings.Index(snd, "@"); i > 0 {
						snd = snd[:i]
					}
					sndJID = types.JID{}
				}
			}

			// Upsert a per-sender chat entry
			if !fromMe && snd != "" {
				c.ensureSenderChat(sndJID)
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
			senderName := c.resolveSenderName(snd, sndJID, m.Message.GetPushName())

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
	return HistorySyncResult{MessagesSynced: synced, MoreAvailable: moreAvailable}
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
			if name == "" || name == parsed.User || name == "Group "+parsed.User {
				toUpdate = append(toUpdate, row{jid: jidStr, name: name})
			}
			continue
		}

		if name == "" || store.IsBareNumber(name) || strings.HasSuffix(name, "@s.whatsapp.net") {
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

// ensureSenderChat upserts a per-sender chat entry keyed by the sender's phone
// JID. Senders whose phone number is unknown (unmapped LIDs) are skipped rather
// than stored under a fake <lid>@s.whatsapp.net JID.
func (c *Client) ensureSenderChat(senderJID types.JID) {
	indiv, ok := c.phoneJIDFor(senderJID)
	if !ok {
		return
	}

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

// backfillSenderNames re-resolves stored messages whose sender_name is missing
// or just a number (typically a LID recorded before its mapping was known).
func (c *Client) backfillSenderNames() {
	if c.Store == nil || c.Store.Messages == nil {
		return
	}

	rows, err := c.Store.Messages.Query(`SELECT DISTINCT sender, COALESCE(sender_name, '') FROM messages WHERE sender != ''`)
	if err != nil {
		c.Logger.Warn("backfill: query senders failed", "err", err)
		return
	}

	pending := map[string]bool{}
	for rows.Next() {
		var sender, name string
		if err := rows.Scan(&sender, &name); err != nil {
			c.Logger.Warn("backfill: scan failed", "err", err)
			continue
		}
		if name == "" || store.IsBareNumber(name) {
			pending[sender] = true
		}
	}
	_ = rows.Close()

	updated := 0
	for sender := range pending {
		resolved := c.resolveSenderName(sender, c.senderJIDFromUser(sender), "")
		if resolved == "" {
			continue
		}

		res, err := c.Store.Messages.Exec(`UPDATE messages SET sender_name = ?
			WHERE sender = ? AND COALESCE(sender_name, '') != ?
			AND (COALESCE(sender_name, '') = '' OR LTRIM(sender_name, '+') NOT GLOB '*[^0-9]*')`,
			resolved, sender, resolved)
		if err != nil {
			c.Logger.Warn("backfill: update sender name failed", "sender", sender, "err", err)
			continue
		}
		if n, _ := res.RowsAffected(); n > 0 {
			updated += int(n)
		}
	}

	if updated > 0 {
		c.Logger.Info("backfill: updated sender names", "count", updated)
	}
}

// pruneLIDSenderChats removes placeholder chats created under <lid>@s.whatsapp.net
// by older versions. They hold no messages and duplicate the real phone JID entry.
func (c *Client) pruneLIDSenderChats() {
	if c.Store == nil || c.Store.Messages == nil {
		return
	}

	rows, err := c.Store.Messages.Query(`SELECT c.jid FROM chats c
		WHERE c.jid LIKE '%@s.whatsapp.net' AND c.last_message_time IS NULL
		AND NOT EXISTS (SELECT 1 FROM messages m WHERE m.chat_jid = c.jid)`)
	if err != nil {
		c.Logger.Warn("backfill: query placeholder chats failed", "err", err)
		return
	}

	// Collect first: the store allows one connection, so lookups can't run
	// while this cursor is open.
	var candidates []string
	for rows.Next() {
		var jidStr string
		if err := rows.Scan(&jidStr); err == nil {
			candidates = append(candidates, jidStr)
		}
	}
	_ = rows.Close()

	removed := 0
	for _, jidStr := range candidates {
		parsed, err := types.ParseJID(jidStr)
		if err != nil {
			continue
		}
		lid := types.JID{User: parsed.User, Server: types.HiddenUserServer}
		if pn, ok := c.phoneJIDFor(lid); !ok || pn.User == parsed.User {
			continue
		}
		if _, err := c.Store.Messages.Exec(`DELETE FROM chats WHERE jid = ?`, jidStr); err != nil {
			c.Logger.Warn("backfill: delete placeholder chat failed", "jid", jidStr, "err", err)
			continue
		}
		removed++
	}

	if removed > 0 {
		c.Logger.Info("backfill: removed LID placeholder chats", "count", removed)
	}
}

// backfillNames repairs stored chat and sender names after a sync.
func (c *Client) backfillNames() {
	c.pruneLIDSenderChats()
	c.backfillSenderNames()
	c.backfillChatNames()
}
