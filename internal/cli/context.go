package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

var (
	contextChats    int
	contextMessages int
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Get aggregated context for LLM use",
	Long: `Returns aggregated context including connection status and recent chat activity.

Useful for providing context to LLMs about your WhatsApp state.`,
	RunE: runContext,
}

func init() {
	rootCmd.AddCommand(contextCmd)
	contextCmd.Flags().IntVar(&contextChats, "chats", 5, "Number of recent chats to include")
	contextCmd.Flags().IntVar(&contextMessages, "messages", 10, "Messages per chat")
}

func runContext(cmd *cobra.Command, args []string) error {
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

	// Get connection status
	status := store.ConnectionStatus{
		Connected: false,
		LoggedIn:  false,
	}

	if client.IsAuthenticated() {
		if err := client.Connect(); err == nil {
			status.Connected = client.IsConnected()
			status.LoggedIn = client.IsLoggedIn()

			user, device := client.GetDeviceID()
			if user != "" {
				status.Device = &store.DeviceInfo{
					User:   user,
					Device: device,
				}
			}
			client.Disconnect()
		}
	}

	// Get database stats
	chatCount, _ := db.CountChats("")
	msgCount, _ := db.CountMessages()
	status.Database = &store.DBStats{
		Chats:    chatCount,
		Messages: msgCount,
	}

	// Get recent chats with messages
	chats, err := db.ListChats(store.ListChatsOptions{Limit: contextChats})
	if err != nil {
		return fmt.Errorf("failed to list chats: %w", err)
	}

	var recentChats []store.ChatWithRecent
	for _, chat := range chats {
		messages, err := db.ListMessages(store.ListMessagesOptions{
			ChatJID: chat.JID,
			Limit:   contextMessages,
		})
		if err != nil {
			continue
		}

		recentChats = append(recentChats, store.ChatWithRecent{
			Chat:           chat,
			RecentMessages: messages,
		})
	}

	result := store.ContextResult{
		Connection:  &status,
		RecentChats: recentChats,
	}

	return Output(result)
}
