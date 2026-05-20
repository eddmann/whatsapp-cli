package whatsapp

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"go.mau.fi/whatsmeow/types"
)

const backfillSendTimeout = 30 * time.Second

type BackfillOptions struct {
	JID   string
	Count int
	Pages int
	Wait  time.Duration
}

type BackfillResult struct {
	ChatJID         string    `json:"jid"`
	Count           int       `json:"count"`
	PagesRequested  int       `json:"pages_requested"`
	MessagesSynced  int       `json:"messages_synced"`
	MoreAvailable   bool      `json:"more_available"`
	AnchorMessageID string    `json:"anchor_message_id"`
	AnchorTimestamp time.Time `json:"anchor_timestamp"`
}

type HistorySyncResult struct {
	MessagesSynced int
	MoreAvailable  bool
}

type pendingBackfillRequest struct {
	chatJID  string
	response chan HistorySyncResult
}

func (c *Client) RequestBackfill(ctx context.Context, opts BackfillOptions) (*BackfillResult, error) {
	if opts.Count <= 0 {
		return nil, fmt.Errorf("count must be greater than zero")
	}
	if opts.Pages <= 0 {
		return nil, fmt.Errorf("pages must be greater than zero")
	}
	if opts.Wait <= 0 {
		return nil, fmt.Errorf("wait duration must be greater than zero")
	}
	if c.Store == nil {
		return nil, fmt.Errorf("message store is required")
	}

	jid, err := parseJID(opts.JID)
	if err != nil {
		return nil, err
	}

	result := &BackfillResult{
		ChatJID: jid.String(),
		Count:   opts.Count,
	}

	for page := 0; page < opts.Pages; page++ {
		oldest, err := c.Store.OldestMessageForChat(jid.String())
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("no stored messages found for %s; sync the chat before requesting older history", jid.String())
			}
			return nil, fmt.Errorf("failed to find oldest message for %s: %w", jid.String(), err)
		}

		anchor, err := buildBackfillAnchor(oldest)
		if err != nil {
			return nil, err
		}
		result.AnchorMessageID = anchor.ID
		result.AnchorTimestamp = anchor.Timestamp

		response, clearPending, err := c.beginBackfillRequest(jid.String())
		if err != nil {
			return nil, err
		}

		sendCtx, cancelSend := context.WithTimeout(ctx, backfillSendTimeout)
		c.Logger.Info("requesting backfill", "jid", jid.String(), "count", opts.Count, "page", page+1, "before_message_id", anchor.ID, "before_timestamp", anchor.Timestamp)
		_, sendErr := c.WA.SendPeerMessage(sendCtx, c.WA.BuildHistorySyncRequest(anchor, opts.Count))
		cancelSend()
		if sendErr != nil {
			clearPending()
			return nil, fmt.Errorf("failed to send backfill request: %w", sendErr)
		}
		result.PagesRequested++

		waitTimer := time.NewTimer(opts.Wait)
		select {
		case syncResult := <-response:
			clearPending()
			stopTimer(waitTimer)
			result.MessagesSynced += syncResult.MessagesSynced
			result.MoreAvailable = syncResult.MoreAvailable
			if syncResult.MessagesSynced == 0 || !syncResult.MoreAvailable {
				return result, nil
			}
		case <-waitTimer.C:
			clearPending()
			return result, fmt.Errorf("timed out waiting for on-demand history sync after %s", opts.Wait)
		case <-ctx.Done():
			clearPending()
			stopTimer(waitTimer)
			return result, ctx.Err()
		}
	}

	return result, nil
}

func buildBackfillAnchor(message store.Message) (*types.MessageInfo, error) {
	if message.ID == "" {
		return nil, fmt.Errorf("oldest stored message has no ID")
	}

	chat, err := parseJID(message.ChatJID)
	if err != nil {
		return nil, fmt.Errorf("invalid chat JID %q: %w", message.ChatJID, err)
	}

	return &types.MessageInfo{
		MessageSource: types.MessageSource{
			Chat:     chat,
			IsFromMe: message.IsFromMe,
			IsGroup:  chat.Server == types.GroupServer,
		},
		ID:        message.ID,
		Timestamp: message.Timestamp,
	}, nil
}

func (c *Client) beginBackfillRequest(chatJID string) (<-chan HistorySyncResult, func(), error) {
	request := &pendingBackfillRequest{
		chatJID:  chatJID,
		response: make(chan HistorySyncResult, 1),
	}

	c.backfillMu.Lock()
	defer c.backfillMu.Unlock()

	if c.pendingBackfill != nil {
		return nil, nil, fmt.Errorf("another backfill request is already waiting for a response")
	}
	c.pendingBackfill = request

	clear := func() {
		c.backfillMu.Lock()
		if c.pendingBackfill == request {
			c.pendingBackfill = nil
		}
		c.backfillMu.Unlock()
	}

	return request.response, clear, nil
}

func (c *Client) backfillStorageChatJID(responseJID string, isOnDemand bool, conversationCount int) string {
	if !isOnDemand || conversationCount != 1 {
		return responseJID
	}

	c.backfillMu.Lock()
	defer c.backfillMu.Unlock()

	if c.pendingBackfill == nil {
		return responseJID
	}
	return c.pendingBackfill.chatJID
}

func (c *Client) completeBackfillRequest(result HistorySyncResult) {
	c.backfillMu.Lock()
	request := c.pendingBackfill
	c.backfillMu.Unlock()

	if request == nil {
		return
	}

	select {
	case request.response <- result:
	default:
	}
}

func stopTimer(timer *time.Timer) {
	if timer.Stop() {
		return
	}

	select {
	case <-timer.C:
	default:
	}
}
