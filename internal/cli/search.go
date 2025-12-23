package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/eddmann/whatsapp-cli/internal/store"
)

var (
	searchChat      string
	searchFrom      string
	searchType      string
	searchTimeframe string
	searchLimit     int
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Full-text search messages",
	Long: `Search messages using full-text search.

Uses SQLite FTS5 for fast searching across all messages.

Timeframe presets: last_hour, today, yesterday, last_3_days, this_week, last_week, this_month`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().StringVar(&searchChat, "chat", "", "Limit to specific chat JID")
	searchCmd.Flags().StringVar(&searchFrom, "from", "", "Limit to specific sender JID")
	searchCmd.Flags().StringVar(&searchType, "type", "", "Filter by type (text, image, video, audio, document)")
	searchCmd.Flags().StringVar(&searchTimeframe, "timeframe", "", "Timeframe preset")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 50, "Maximum results")
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := args[0]

	// Parse timeframe if provided
	var after, before string
	if searchTimeframe != "" {
		var err error
		after, before, err = ParseTimeframe(searchTimeframe)
		if err != nil {
			return err
		}
	}

	return WithDB(func(db *store.DB) error {
		messages, err := db.SearchMessages(store.SearchMessagesOptions{
			Query:   query,
			ChatJID: searchChat,
			FromJID: searchFrom,
			Type:    searchType,
			After:   after,
			Before:  before,
			Limit:   searchLimit,
		})
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}
		return Output(messages)
	})
}
