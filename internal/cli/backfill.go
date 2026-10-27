package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

var (
	backfillCount int
	backfillPages int
	backfillWait  time.Duration
	backfillAfter string
)

var backfillCmd = &cobra.Command{
	Use:   "backfill <jid>",
	Short: "Request older messages for a chat",
	Long: `Request WhatsApp to send historical messages for a specific chat.

Note: WhatsApp controls how much history is available. This command anchors
requests before the oldest message already stored locally and waits for the
on-demand history sync response before optionally requesting another page.

Use --after to fill a gap in the middle of a chat's history: requests then
anchor at the oldest stored message after that date and walk back towards it.`,
	Args: cobra.ExactArgs(1),
	RunE: runBackfill,
}

func init() {
	rootCmd.AddCommand(backfillCmd)
	backfillCmd.Flags().IntVar(&backfillCount, "count", 50, "Number of messages to request per page")
	backfillCmd.Flags().IntVar(&backfillPages, "pages", 1, "Maximum number of on-demand history pages to request")
	backfillCmd.Flags().DurationVar(&backfillWait, "wait", 30*time.Second, "How long to wait for each on-demand history sync response")
	backfillCmd.Flags().StringVar(&backfillAfter, "after", "", "Fill a history gap: anchor at the oldest message after this date (YYYY-MM-DD or RFC3339)")
}

// parseBackfillAfter accepts a date (YYYY-MM-DD, local time) or an RFC3339 timestamp.
func parseBackfillAfter(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	if t, err := time.ParseInLocation(time.DateOnly, value, time.Local); err == nil {
		return t, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --after %q: use YYYY-MM-DD or RFC3339", value)
	}
	return t, nil
}

func runBackfill(cmd *cobra.Command, args []string) error {
	jid := args[0]
	after, err := parseBackfillAfter(backfillAfter)
	if err != nil {
		return err
	}

	return WithConnection(func(_ *store.DB, client *whatsapp.Client) error {
		result, err := client.RequestBackfill(cmd.Context(), whatsapp.BackfillOptions{
			JID:   jid,
			Count: backfillCount,
			Pages: backfillPages,
			Wait:  backfillWait,
			After: after,
		})
		if err != nil {
			return fmt.Errorf("backfill request failed: %w", err)
		}

		return OutputResult(result, fmt.Sprintf(
			"Requested %d page(s) of %d messages for %s; synced %d message(s)",
			result.PagesRequested,
			result.Count,
			result.ChatJID,
			result.MessagesSynced,
		))
	})
}
