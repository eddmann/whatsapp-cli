package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

var backfillCount int

var backfillCmd = &cobra.Command{
	Use:   "backfill <jid>",
	Short: "Request older messages for a chat",
	Long: `Request WhatsApp to send historical messages for a specific chat.

Note: WhatsApp controls how much history is available and may not send
all requested messages. Run 'whatsapp sync' after to receive them.`,
	Args: cobra.ExactArgs(1),
	RunE: runBackfill,
}

func init() {
	rootCmd.AddCommand(backfillCmd)
	backfillCmd.Flags().IntVar(&backfillCount, "count", 50, "Number of messages to request")
}

func runBackfill(cmd *cobra.Command, args []string) error {
	jid := args[0]

	return WithConnection(func(db *store.DB, client *whatsapp.Client) error {
		if err := client.RequestBackfill(jid, backfillCount); err != nil {
			return fmt.Errorf("backfill request failed: %w", err)
		}

		return OutputResult(map[string]any{
			"jid":   jid,
			"count": backfillCount,
		}, fmt.Sprintf("Requested %d messages for %s", backfillCount, jid))
	})
}
