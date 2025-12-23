package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/eddmann/whatsapp-cli/internal/store"
)

var (
	messagesLimit     int
	messagesBefore    string
	messagesAfter     string
	messagesTimeframe string
	messagesType      string
)

var messagesCmd = &cobra.Command{
	Use:   "messages <jid>",
	Short: "List messages from a chat",
	Long: `List messages from a specific chat by JID.

Use 'whatsapp chats' to find the JID first.

Timeframe presets: last_hour, today, yesterday, last_3_days, this_week, last_week, this_month`,
	Args: cobra.ExactArgs(1),
	RunE: runMessages,
}

func init() {
	rootCmd.AddCommand(messagesCmd)
	messagesCmd.Flags().IntVar(&messagesLimit, "limit", 50, "Maximum number of messages")
	messagesCmd.Flags().StringVar(&messagesBefore, "before", "", "Messages before timestamp (RFC3339)")
	messagesCmd.Flags().StringVar(&messagesAfter, "after", "", "Messages after timestamp (RFC3339)")
	messagesCmd.Flags().StringVar(&messagesTimeframe, "timeframe", "", "Timeframe preset (today, yesterday, this_week, etc.)")
	messagesCmd.Flags().StringVar(&messagesType, "type", "", "Filter by type (text, image, video, audio, document)")
}

func runMessages(cmd *cobra.Command, args []string) error {
	jid := args[0]

	// Parse timeframe if provided
	after, before := messagesAfter, messagesBefore
	if messagesTimeframe != "" {
		var err error
		after, before, err = ParseTimeframe(messagesTimeframe)
		if err != nil {
			return err
		}
	}

	return WithDB(func(db *store.DB) error {
		messages, err := db.ListMessages(store.ListMessagesOptions{
			ChatJID: jid,
			After:   after,
			Before:  before,
			Type:    messagesType,
			Limit:   messagesLimit,
		})
		if err != nil {
			return fmt.Errorf("failed to list messages: %w", err)
		}
		return Output(messages)
	})
}
