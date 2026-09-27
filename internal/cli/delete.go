package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

var deleteChat string

var deleteCmd = &cobra.Command{
	Use:   "delete <msg-id>",
	Short: "Delete a message for everyone",
	Long: `Revoke (delete for everyone) a message.

Requires --chat to specify the chat JID. Messages you sent can always be
revoked; deleting another participant's message requires group admin.

The local store row is not removed — the placeholder arrives via sync.

Examples:
  whatsapp delete ABC123 --chat 1234567890@s.whatsapp.net`,
	Args: cobra.ExactArgs(1),
	RunE: runDelete,
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().StringVar(&deleteChat, "chat", "", "Chat JID (required)")
	_ = deleteCmd.MarkFlagRequired("chat")
}

func runDelete(cmd *cobra.Command, args []string) error {
	messageID := args[0]

	return WithConnection(func(db *store.DB, client *whatsapp.Client) error {
		result, err := client.RevokeMessage(deleteChat, messageID)
		if err != nil {
			return fmt.Errorf("delete failed: %w", err)
		}

		return OutputResult(store.SendResult{
			MessageID: result.MessageID,
			ChatJID:   result.ChatJID,
			Timestamp: result.Timestamp,
		}, fmt.Sprintf("Deleted message %s", messageID))
	})
}
