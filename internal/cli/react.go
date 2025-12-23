package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

var (
	reactChat   string
	reactRemove bool
)

var reactCmd = &cobra.Command{
	Use:   "react <msg-id> <emoji>",
	Short: "React to a message",
	Long: `Add or remove a reaction to a message.

Requires --chat to specify the chat JID.

Examples:
  whatsapp react ABC123 "thumbsup" --chat 1234567890@s.whatsapp.net
  whatsapp react ABC123 "" --chat 1234567890@s.whatsapp.net --remove`,
	Args: func(cmd *cobra.Command, args []string) error {
		remove, _ := cmd.Flags().GetBool("remove")
		if remove {
			if len(args) < 1 {
				return fmt.Errorf("requires at least 1 arg (msg-id)")
			}
			return nil
		}
		if len(args) < 2 {
			return fmt.Errorf("requires 2 args (msg-id and emoji)")
		}
		return nil
	},
	RunE: runReact,
}

func init() {
	rootCmd.AddCommand(reactCmd)
	reactCmd.Flags().StringVar(&reactChat, "chat", "", "Chat JID (required)")
	reactCmd.Flags().BoolVar(&reactRemove, "remove", false, "Remove reaction instead of adding")
	_ = reactCmd.MarkFlagRequired("chat")
}

func runReact(cmd *cobra.Command, args []string) error {
	messageID := args[0]
	emoji := ""
	if len(args) > 1 {
		emoji = args[1]
	}

	return WithConnection(func(db *store.DB, client *whatsapp.Client) error {
		result, err := client.SendReaction(reactChat, messageID, emoji, reactRemove)
		if err != nil {
			return fmt.Errorf("react failed: %w", err)
		}

		return OutputResult(store.SendResult{
			MessageID: result.MessageID,
			ChatJID:   result.ChatJID,
			Timestamp: result.Timestamp,
		}, fmt.Sprintf("Reacted to message %s", result.MessageID))
	})
}
