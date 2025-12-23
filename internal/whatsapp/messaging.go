package whatsapp

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.mau.fi/whatsmeow"
	waCommon "go.mau.fi/whatsmeow/proto/waCommon"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

// SendMessageResult represents the result of sending a WhatsApp message.
type SendMessageResult struct {
	Success   bool
	Message   string
	MessageID string
	ChatJID   string
	Timestamp string
}

// DownloadMediaResult represents the result of downloading media from WhatsApp.
type DownloadMediaResult struct {
	Success   bool
	MediaType string
	Filename  string
	Path      string
}

// SendText sends a text message to a JID or phone number string (without +) or group JID.
// If replyToMessageID is provided, sends as a quoted reply.
func (c *Client) SendText(recipient, text, replyToMessageID string) (*SendMessageResult, error) {
	if !c.WA.IsConnected() {
		return &SendMessageResult{Success: false, Message: "not connected"}, fmt.Errorf("not connected")
	}

	jid, err := parseRecipient(recipient)
	if err != nil {
		return &SendMessageResult{Success: false, Message: "invalid recipient"}, err
	}

	msg := &waE2E.Message{}

	if replyToMessageID != "" {
		quotedMsg, err := c.buildQuotedMessage(replyToMessageID, jid.String())
		if err != nil {
			return &SendMessageResult{Success: false, Message: "failed to build quote"}, err
		}

		msg.ExtendedTextMessage = &waE2E.ExtendedTextMessage{
			Text:        protoString(text),
			ContextInfo: quotedMsg,
		}
	} else {
		msg.Conversation = protoString(text)
	}

	resp, err := c.WA.SendMessage(context.Background(), jid, msg)
	if err != nil {
		return &SendMessageResult{Success: false, Message: err.Error()}, err
	}

	return &SendMessageResult{
		Success:   true,
		Message:   fmt.Sprintf("sent to %s", recipient),
		MessageID: resp.ID,
		ChatJID:   jid.String(),
		Timestamp: resp.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

// SendMedia sends an image/video/document/audio with optional caption.
// If replyToMessageID is provided, sends as a quoted reply.
func (c *Client) SendMedia(recipient, path, caption, replyToMessageID string) (*SendMessageResult, error) {
	if !c.WA.IsConnected() {
		return &SendMessageResult{Success: false, Message: "not connected"}, fmt.Errorf("not connected")
	}

	jid, err := parseRecipient(recipient)
	if err != nil {
		return &SendMessageResult{Success: false, Message: "invalid recipient"}, err
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return &SendMessageResult{Success: false, Message: "read error"}, err
	}

	mediaType, mime := classify(path)
	up, err := c.WA.Upload(context.Background(), b, mediaType)
	if err != nil {
		return &SendMessageResult{Success: false, Message: "upload failed"}, err
	}

	m := &waE2E.Message{}
	base := filepath.Base(path)

	var quotedCtx *waE2E.ContextInfo
	if replyToMessageID != "" {
		quotedCtx, err = c.buildQuotedMessage(replyToMessageID, jid.String())
		if err != nil {
			return &SendMessageResult{Success: false, Message: "failed to build quote"}, err
		}
	}

	switch mediaType {
	case whatsmeow.MediaImage:
		m.ImageMessage = &waE2E.ImageMessage{
			Caption:       protoString(caption),
			Mimetype:      protoString(mime),
			URL:           &up.URL,
			DirectPath:    &up.DirectPath,
			MediaKey:      up.MediaKey,
			FileEncSHA256: up.FileEncSHA256,
			FileSHA256:    up.FileSHA256,
			FileLength:    &up.FileLength,
			ContextInfo:   quotedCtx,
		}
	case whatsmeow.MediaVideo:
		m.VideoMessage = &waE2E.VideoMessage{
			Caption:       protoString(caption),
			Mimetype:      protoString(mime),
			URL:           &up.URL,
			DirectPath:    &up.DirectPath,
			MediaKey:      up.MediaKey,
			FileEncSHA256: up.FileEncSHA256,
			FileSHA256:    up.FileSHA256,
			FileLength:    &up.FileLength,
			ContextInfo:   quotedCtx,
		}
	case whatsmeow.MediaDocument:
		m.DocumentMessage = &waE2E.DocumentMessage{
			Title:         protoString(base),
			Caption:       protoString(caption),
			Mimetype:      protoString(mime),
			URL:           &up.URL,
			DirectPath:    &up.DirectPath,
			MediaKey:      up.MediaKey,
			FileEncSHA256: up.FileEncSHA256,
			FileSHA256:    up.FileSHA256,
			FileLength:    &up.FileLength,
			ContextInfo:   quotedCtx,
		}
	case whatsmeow.MediaAudio:
		if !isOgg(path) {
			cpath, err := ConvertToOpusOgg(path)
			if err != nil {
				return &SendMessageResult{Success: false, Message: "conversion failed"}, err
			}
			defer func() { _ = os.Remove(cpath) }()

			b2, err := os.ReadFile(cpath)
			if err != nil {
				return &SendMessageResult{Success: false, Message: "read converted"}, err
			}

			up2, err := c.WA.Upload(context.Background(), b2, whatsmeow.MediaAudio)
			if err != nil {
				return &SendMessageResult{Success: false, Message: "upload converted"}, err
			}

			dur, waveform, _ := AnalyzeOggOpus(b2)
			m.AudioMessage = &waE2E.AudioMessage{
				Mimetype:      protoString("audio/ogg; codecs=opus"),
				URL:           &up2.URL,
				DirectPath:    &up2.DirectPath,
				MediaKey:      up2.MediaKey,
				FileEncSHA256: up2.FileEncSHA256,
				FileSHA256:    up2.FileSHA256,
				FileLength:    &up2.FileLength,
				Seconds:       protoUint32(dur),
				PTT:           protoBool(true),
				Waveform:      waveform,
				ContextInfo:   quotedCtx,
			}
		} else {
			dur, waveform, _ := AnalyzeOggOpus(b)
			m.AudioMessage = &waE2E.AudioMessage{
				Mimetype:      protoString(mime),
				URL:           &up.URL,
				DirectPath:    &up.DirectPath,
				MediaKey:      up.MediaKey,
				FileEncSHA256: up.FileEncSHA256,
				FileSHA256:    up.FileSHA256,
				FileLength:    &up.FileLength,
				Seconds:       protoUint32(dur),
				PTT:           protoBool(true),
				Waveform:      waveform,
				ContextInfo:   quotedCtx,
			}
		}
	}

	resp, err := c.WA.SendMessage(context.Background(), jid, m)
	if err != nil {
		return &SendMessageResult{Success: false, Message: err.Error()}, err
	}

	return &SendMessageResult{
		Success:   true,
		Message:   fmt.Sprintf("sent media to %s", recipient),
		MessageID: resp.ID,
		ChatJID:   jid.String(),
		Timestamp: resp.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

// ForwardMessage forwards a message to a recipient.
func (c *Client) ForwardMessage(recipient, messageID, fromChatJID string) (*SendMessageResult, error) {
	if !c.WA.IsConnected() {
		return &SendMessageResult{Success: false, Message: "not connected"}, fmt.Errorf("not connected")
	}

	toJID, err := parseRecipient(recipient)
	if err != nil {
		return &SendMessageResult{Success: false, Message: "invalid recipient"}, err
	}

	// Query original message content
	var content, mediaType string
	row := c.Store.Messages.QueryRow(`
		SELECT content, COALESCE(media_type, '') FROM messages WHERE id = ? AND chat_jid = ?
	`, messageID, fromChatJID)
	if err := row.Scan(&content, &mediaType); err != nil {
		return &SendMessageResult{Success: false, Message: "message not found"}, err
	}

	// For now, only forward text messages
	if mediaType != "" {
		return &SendMessageResult{Success: false, Message: "media forwarding not supported"}, fmt.Errorf("media forwarding not yet supported")
	}

	msg := &waE2E.Message{
		Conversation: protoString(content),
	}

	resp, err := c.WA.SendMessage(context.Background(), toJID, msg)
	if err != nil {
		return &SendMessageResult{Success: false, Message: err.Error()}, err
	}

	return &SendMessageResult{
		Success:   true,
		Message:   fmt.Sprintf("forwarded to %s", recipient),
		MessageID: resp.ID,
		ChatJID:   toJID.String(),
		Timestamp: resp.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

// SendReaction sends a reaction to a message.
func (c *Client) SendReaction(chatJID, messageID, emoji string, remove bool) (*SendMessageResult, error) {
	if !c.WA.IsConnected() {
		return &SendMessageResult{Success: false, Message: "not connected"}, fmt.Errorf("not connected")
	}

	jid, err := parseRecipient(chatJID)
	if err != nil {
		return &SendMessageResult{Success: false, Message: "invalid chat JID"}, err
	}

	// Get the sender of the original message for the reaction target
	var sender string
	var isFromMe bool
	row := c.Store.Messages.QueryRow(`SELECT sender, is_from_me FROM messages WHERE id = ? AND chat_jid = ?`, messageID, chatJID)
	if err := row.Scan(&sender, &isFromMe); err != nil {
		return &SendMessageResult{Success: false, Message: "message not found"}, err
	}

	reactionText := emoji
	if remove {
		reactionText = ""
	}

	msg := &waE2E.Message{
		ReactionMessage: &waE2E.ReactionMessage{
			Key: &waCommon.MessageKey{
				RemoteJID: protoString(chatJID),
				FromMe:    protoBool(isFromMe),
				ID:        protoString(messageID),
			},
			Text: protoString(reactionText),
		},
	}

	resp, err := c.WA.SendMessage(context.Background(), jid, msg)
	if err != nil {
		return &SendMessageResult{Success: false, Message: err.Error()}, err
	}

	action := "reacted"
	if remove {
		action = "removed reaction"
	}

	return &SendMessageResult{
		Success:   true,
		Message:   fmt.Sprintf("%s to message %s", action, messageID),
		MessageID: resp.ID,
		ChatJID:   jid.String(),
		Timestamp: resp.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

// DownloadMedia looks up media from DB and downloads via whatsmeow.
func (c *Client) DownloadMedia(messageID, chatJID string) (*DownloadMediaResult, error) {
	var mediaType, filename, url string
	var mediaKey, fileSHA256, fileEncSHA256 []byte
	var fileLength uint64

	row := c.Store.Messages.QueryRow("SELECT media_type, filename, url, media_key, file_sha256, file_enc_sha256, file_length FROM messages WHERE id = ? AND chat_jid = ?", messageID, chatJID)
	if err := row.Scan(&mediaType, &filename, &url, &mediaKey, &fileSHA256, &fileEncSHA256, &fileLength); err != nil {
		return &DownloadMediaResult{Success: false}, err
	}

	if mediaType == "" || url == "" || len(mediaKey) == 0 || len(fileSHA256) == 0 || len(fileEncSHA256) == 0 || fileLength == 0 {
		return &DownloadMediaResult{Success: false}, fmt.Errorf("incomplete media info")
	}

	dp := extractDirectPathFromURL(url)
	dm := &downloadable{
		URL:           url,
		DirectPath:    dp,
		MediaKey:      mediaKey,
		FileLength:    fileLength,
		FileSHA256:    fileSHA256,
		FileEncSHA256: fileEncSHA256,
		MediaType:     classifyToWA(mediaType),
	}

	data, err := c.WA.Download(context.Background(), dm)
	if err != nil {
		return &DownloadMediaResult{Success: false}, err
	}

	outDir := filepath.Join(c.BaseDir, strings.ReplaceAll(chatJID, ":", "_"))
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return &DownloadMediaResult{Success: false}, err
	}

	out := filepath.Join(outDir, filename)
	if err := os.WriteFile(out, data, fs.FileMode(0644)); err != nil {
		return &DownloadMediaResult{Success: false}, err
	}

	abs, _ := filepath.Abs(out)
	return &DownloadMediaResult{
		Success:   true,
		MediaType: mediaType,
		Filename:  filename,
		Path:      abs,
	}, nil
}

// protoString returns a pointer to a string (for protobuf).
func protoString(s string) *string { return &s }

// protoBool returns a pointer to a bool (for protobuf).
func protoBool(b bool) *bool { return &b }

// protoUint32 returns a pointer to a uint32 (for protobuf).
func protoUint32(u uint32) *uint32 { return &u }

// parseRecipient parses a recipient string (phone or JID) into a types.JID.
func parseRecipient(recipient string) (types.JID, error) {
	if strings.Contains(recipient, "@") {
		return types.ParseJID(recipient)
	}
	return types.JID{User: recipient, Server: "s.whatsapp.net"}, nil
}

// buildQuotedMessage fetches the message being replied to and constructs a ContextInfo.
func (c *Client) buildQuotedMessage(messageID, chatJID string) (*waE2E.ContextInfo, error) {
	var sender, content string
	var isFromMe bool
	var mediaType *string

	row := c.Store.Messages.QueryRow(`
		SELECT sender, content, is_from_me, media_type
		FROM messages
		WHERE id = ? AND chat_jid = ?
	`, messageID, chatJID)

	err := row.Scan(&sender, &content, &isFromMe, &mediaType)
	if err != nil {
		return nil, fmt.Errorf("failed to find quoted message: %w", err)
	}

	participantJID := ""
	if strings.HasSuffix(chatJID, "@g.us") {
		if !strings.Contains(sender, "@") {
			participantJID = sender + "@s.whatsapp.net"
		} else {
			participantJID = sender
		}
	}

	quotedMsg := &waE2E.Message{}
	if mediaType != nil && *mediaType != "" {
		quotedMsg.Conversation = protoString(getMediaEmoji(*mediaType))
	} else {
		quotedMsg.Conversation = protoString(content)
	}

	ctx := &waE2E.ContextInfo{
		StanzaID:      protoString(messageID),
		QuotedMessage: quotedMsg,
	}

	if participantJID != "" {
		ctx.Participant = protoString(participantJID)
	}

	return ctx, nil
}

// getMediaEmoji returns an emoji representation for media types.
func getMediaEmoji(mediaType string) string {
	switch mediaType {
	case "image":
		return "Photo"
	case "video":
		return "Video"
	case "audio":
		return "Audio"
	case "document":
		return "Document"
	default:
		return "Media"
	}
}

// classify determines WhatsApp media type and MIME type from file extension.
func classify(path string) (whatsmeow.MediaType, string) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return whatsmeow.MediaImage, "image/jpeg"
	case ".png":
		return whatsmeow.MediaImage, "image/png"
	case ".gif":
		return whatsmeow.MediaImage, "image/gif"
	case ".webp":
		return whatsmeow.MediaImage, "image/webp"
	case ".mp4":
		return whatsmeow.MediaVideo, "video/mp4"
	case ".avi":
		return whatsmeow.MediaVideo, "video/avi"
	case ".mov":
		return whatsmeow.MediaVideo, "video/quicktime"
	case ".ogg":
		return whatsmeow.MediaAudio, "audio/ogg; codecs=opus"
	default:
		return whatsmeow.MediaDocument, "application/octet-stream"
	}
}

// isOgg checks if a file is an Ogg file.
func isOgg(path string) bool {
	return strings.ToLower(filepath.Ext(path)) == ".ogg"
}
