package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/eddmann/whatsapp-cli/internal/store"
)

var (
	chatsQuery  string
	chatsGroups bool
	chatsLimit  int
)

var chatsCmd = &cobra.Command{
	Use:   "chats",
	Short: "List all chats",
	Long: `List all chats from the local database.

Use --query to filter by name, --groups for groups only.
Returns JIDs that can be used with other commands.`,
	RunE: runChats,
}

func init() {
	rootCmd.AddCommand(chatsCmd)
	chatsCmd.Flags().StringVar(&chatsQuery, "query", "", "Filter by chat name")
	chatsCmd.Flags().BoolVar(&chatsGroups, "groups", false, "Show groups only")
	chatsCmd.Flags().IntVar(&chatsLimit, "limit", 50, "Maximum number of chats")
}

func runChats(cmd *cobra.Command, args []string) error {
	return WithDB(func(db *store.DB) error {
		chats, err := db.ListChats(store.ListChatsOptions{
			Query:      chatsQuery,
			OnlyGroups: chatsGroups,
			Limit:      chatsLimit,
		})
		if err != nil {
			return fmt.Errorf("failed to list chats: %w", err)
		}
		return Output(chats)
	})
}
