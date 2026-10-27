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

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication commands",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with WhatsApp via QR code and sync messages",
	Long: `Display a QR code in the terminal. Scan it with WhatsApp on your phone
(Settings → Linked Devices → Link a Device) to authenticate.

After authentication, an initial sync will start to download your message history.

By default WhatsApp only shares recent history with a newly linked device. Use
--full-history to ask the phone for its full message history instead (as
WhatsApp Desktop does). This only applies when linking: run 'whatsapp auth
logout' first if already authenticated. Login then stays connected until the
phone stops sending history (5 minutes idle, 2 hours at most); keep WhatsApp
open on the phone meanwhile.`,
	RunE: runAuthLogin,
}

var authLoginFullHistory bool

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Disconnect and clear session",
	RunE:  runAuthLogout,
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show connection status and database stats",
	RunE:  runAuthStatus,
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)

	authLoginCmd.Flags().BoolVar(&authLoginFullHistory, "full-history", false, "Request full message history from the phone when linking")
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	if err := EnsureDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Open messages database
	db, err := store.Open(GetMessagesDBPath())
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.CloseQuietly()

	// Create WhatsApp client
	client, err := whatsapp.New(db, GetStoreDir(), IsVerbose(), nil)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if authLoginFullHistory {
		if client.IsAuthenticated() {
			return fmt.Errorf("--full-history only applies when linking a new device; run 'whatsapp auth logout' first")
		}
		whatsapp.RequestFullHistorySync()
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
		client.Disconnect()
		cancel()
	}()

	// Connect with QR code
	fmt.Fprintln(os.Stderr, "Scan this QR code with WhatsApp (Settings → Linked Devices → Link a Device):")
	fmt.Fprintln(os.Stderr, "")

	if err := client.ConnectWithQR(ctx); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Authenticated! Starting initial sync...")
	fmt.Fprintln(os.Stderr, "This may take a few minutes depending on your message history.")
	fmt.Fprintln(os.Stderr, "Press Ctrl+C to stop (you can resume sync later with 'whatsapp sync').")
	fmt.Fprintln(os.Stderr, "")

	// Wait for initial sync (with timeout)
	syncTimeout := 5 * time.Minute
	select {
	case <-ctx.Done():
		// User interrupted
	case <-client.SyncComplete:
		fmt.Fprintln(os.Stderr, "Initial sync complete.")
	case <-time.After(syncTimeout):
		fmt.Fprintln(os.Stderr, "Initial sync timeout reached. You can continue syncing with 'whatsapp sync'.")
	}

	// The phone sends full history in later chunks and gives up ("couldn't
	// finish syncing") if the device disconnects, so stay connected until it
	// goes quiet.
	if authLoginFullHistory && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "Waiting for full history from your phone (keep WhatsApp open on Wi-Fi). Stops after %s without new history.\n", fullHistoryIdleTimeout)
		waitForHistoryIdle(ctx, client.HistoryActivity, fullHistoryIdleTimeout, fullHistoryMaxWait)
		fmt.Fprintln(os.Stderr, "Full history sync finished or went quiet. Run 'whatsapp sync --follow' if more is expected.")
	}

	client.Disconnect()

	// Output result
	user, device := client.GetDeviceID()
	return OutputResult(map[string]any{
		"user":   user,
		"device": device,
	}, fmt.Sprintf("Authenticated as %s (device %d)", user, device))
}

const (
	fullHistoryIdleTimeout = 5 * time.Minute
	fullHistoryMaxWait     = 2 * time.Hour
)

// waitForHistoryIdle blocks until no activity arrives for idle, maxWait
// elapses, or ctx is cancelled.
func waitForHistoryIdle(ctx context.Context, activity <-chan struct{}, idle, maxWait time.Duration) {
	idleTimer := time.NewTimer(idle)
	defer idleTimer.Stop()
	deadline := time.NewTimer(maxWait)
	defer deadline.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-deadline.C:
			return
		case <-idleTimer.C:
			return
		case <-activity:
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(idle)
		}
	}
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
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
		if GetFormat() == FormatHuman {
			fmt.Println("Already logged out")
		}
		return nil
	}

	// Connect briefly to logout properly
	if err := client.Connect(); err == nil {
		if err := client.Logout(); err != nil {
			OutputWarning("Logout failed: %v", err)
		}
	}

	client.Disconnect()

	// Remove session database
	sessionPath := GetStoreDir() + "/session.db"
	if err := os.Remove(sessionPath); err != nil && !os.IsNotExist(err) {
		OutputWarning("Failed to remove session file: %v", err)
	}

	if GetFormat() == FormatHuman {
		fmt.Println("Logged out")
	}
	return nil
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	status := store.ConnectionStatus{
		Connected: false,
		LoggedIn:  false,
	}

	db, err := store.Open(GetMessagesDBPath())
	if err != nil {
		// Database doesn't exist yet
		return Output(status)
	}
	defer db.CloseQuietly()

	client, err := whatsapp.New(db, GetStoreDir(), IsVerbose(), nil)
	if err != nil {
		return Output(status)
	}

	if client.IsAuthenticated() {
		// Try to connect to check actual status
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

	return Output(status)
}
