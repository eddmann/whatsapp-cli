package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

var (
	syncFollow        bool
	syncDownloadMedia bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync messages from WhatsApp",
	Long: `Connect to WhatsApp and sync new messages to the local database.

By default, performs a one-time sync and exits.
Use --follow to run continuously and capture messages in real-time.`,
	RunE: runSync,
}

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.Flags().BoolVar(&syncFollow, "follow", false, "Run continuously, syncing messages in real-time")
	syncCmd.Flags().BoolVar(&syncDownloadMedia, "download-media", false, "Automatically download media files")
}

func runSync(cmd *cobra.Command, args []string) error {
	if err := EnsureDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	db, err := store.Open(GetMessagesDBPath())
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.CloseQuietly()

	client, err := whatsapp.New(db, GetStoreDir(), IsVerbose(), nil)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if !client.IsAuthenticated() {
		return fmt.Errorf("not authenticated. Run 'whatsapp auth login' first")
	}

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		signal.Stop(sigChan)
		fmt.Fprintln(os.Stderr, "\nInterrupted, disconnecting...")
		cancel()
	}()

	// Connect
	if err := client.Connect(); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	if syncFollow {
		fmt.Fprintln(os.Stderr, "Connected. Syncing messages continuously. Press Ctrl+C to stop.")

		// Run until interrupted
		<-ctx.Done()
	} else {
		fmt.Fprintln(os.Stderr, "Connected. Performing one-time sync...")

		// Wait for sync completion, timeout, or interrupt
		syncTimeout := 2 * time.Minute
		select {
		case <-ctx.Done():
			// User interrupted
		case <-client.SyncComplete:
			fmt.Fprintln(os.Stderr, "History sync complete.")
		case <-time.After(syncTimeout):
			fmt.Fprintln(os.Stderr, "Sync timeout reached.")
		}
	}

	client.Disconnect()

	// Update last sync time
	_ = db.SetLastSyncTime(time.Now())

	// Output stats
	chatCount, _ := db.CountChats("")
	msgCount, _ := db.CountMessages()

	return OutputResult(map[string]any{
		"chats":    chatCount,
		"messages": msgCount,
	}, fmt.Sprintf("Synced %d chats, %d messages", chatCount, msgCount))
}
