package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/eddmann/whatsapp-cli/internal/store"
)

var exportOutput string

var exportCmd = &cobra.Command{
	Use:   "export <jid>",
	Short: "Export chat history",
	Long: `Export chat history to a JSON file.

Exports all messages from the local database for the specified chat.`,
	Args: cobra.ExactArgs(1),
	RunE: runExport,
}

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file (default: stdout)")
}

func runExport(cmd *cobra.Command, args []string) error {
	jid := args[0]

	return WithDB(func(db *store.DB) error {
		messages, err := db.ListMessages(store.ListMessagesOptions{
			ChatJID: jid,
			Limit:   0, // No limit
		})
		if err != nil {
			return fmt.Errorf("failed to list messages: %w", err)
		}

		chatName := db.GetChatName(jid)

		exportData := map[string]any{
			"jid":           jid,
			"name":          chatName,
			"message_count": len(messages),
			"messages":      messages,
		}

		data, err := json.MarshalIndent(exportData, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal: %w", err)
		}

		if exportOutput != "" {
			if err := os.WriteFile(exportOutput, data, 0644); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
			return OutputResult(map[string]any{
				"jid":           jid,
				"message_count": len(messages),
				"output":        exportOutput,
			}, fmt.Sprintf("Exported %d messages to %s", len(messages), exportOutput))
		}

		fmt.Println(string(data))
		return nil
	})
}
