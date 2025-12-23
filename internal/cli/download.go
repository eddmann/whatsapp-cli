package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

var downloadChat string

var downloadCmd = &cobra.Command{
	Use:   "download <msg-id>",
	Short: "Download media from a message",
	Long: `Download media (image, video, audio, document) from a message.

Requires --chat to specify the chat JID.
Files are saved to the store directory under the chat's folder.`,
	Args: cobra.ExactArgs(1),
	RunE: runDownload,
}

func init() {
	rootCmd.AddCommand(downloadCmd)
	downloadCmd.Flags().StringVar(&downloadChat, "chat", "", "Chat JID (required)")
	_ = downloadCmd.MarkFlagRequired("chat")
}

func runDownload(cmd *cobra.Command, args []string) error {
	messageID := args[0]

	return WithConnection(func(db *store.DB, client *whatsapp.Client) error {
		result, err := client.DownloadMedia(messageID, downloadChat)
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}

		return OutputResult(store.DownloadResult{
			Filename: result.Filename,
			Path:     result.Path,
		}, fmt.Sprintf("Downloaded %s to %s", result.Filename, result.Path))
	})
}
