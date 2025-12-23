package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

var forwardFrom string

var forwardCmd = &cobra.Command{
	Use:   "forward <jid> <msg-id>",
	Short: "Forward a message",
	Long: `Forward a message to a chat.

Requires --from to specify the source chat JID.

Examples:
  whatsapp forward 1234567890@s.whatsapp.net ABC123 --from 9876543210@s.whatsapp.net`,
	Args: cobra.ExactArgs(2),
	RunE: runForward,
}

func init() {
	rootCmd.AddCommand(forwardCmd)
	forwardCmd.Flags().StringVar(&forwardFrom, "from", "", "Source chat JID (required)")
	_ = forwardCmd.MarkFlagRequired("from")
}

func runForward(cmd *cobra.Command, args []string) error {
	toJID := args[0]
	messageID := args[1]

	return WithConnection(func(db *store.DB, client *whatsapp.Client) error {
		result, err := client.ForwardMessage(toJID, messageID, forwardFrom)
		if err != nil {
			return fmt.Errorf("forward failed: %w", err)
		}

		return OutputResult(store.SendResult{
			MessageID: result.MessageID,
			ChatJID:   result.ChatJID,
			Timestamp: result.Timestamp,
		}, fmt.Sprintf("Forwarded message %s", result.MessageID))
	})
}
