package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mdp/qrterminal/v3"
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

With --format jsonl, no QR art is drawn: each step is written to stdout as one
JSON object per line, for a program to render the QR code and follow pairing:

  {"event":"qr","code":"2@...","expires_in":60}        a QR code to show; the next replaces it
  {"event":"authenticated","user":"447700900123","device":12}
  {"event":"sync_complete"}, {"event":"sync_timeout"} or {"event":"interrupted"}
  {"event":"error","reason":"timeout","error":"..."}   pairing failed (exit status 1)`,
	RunE: runAuthLogin,
}

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

	// With --format jsonl, each step is a JSON line on stdout instead of terminal text
	var events *pairingEvents
	if GetFormat() == FormatJSONL {
		events = newPairingEvents(os.Stdout)
	}
	say := func(line string) {
		if events == nil {
			fmt.Fprintln(os.Stderr, line)
		}
	}

	// Connect with QR code
	onQR := func(code string, _ time.Duration) {
		qrterminal.GenerateHalfBlock(code, qrterminal.L, os.Stderr)
	}
	if events != nil {
		onQR = events.qr
	}
	say("Scan this QR code with WhatsApp (Settings → Linked Devices → Link a Device):")
	say("")

	if err := client.ConnectWithQR(ctx, onQR); err != nil {
		if events != nil {
			events.failed(err)
		}
		return fmt.Errorf("connection failed: %w", err)
	}

	user, device := client.GetDeviceID()
	if events != nil {
		events.authenticated(user, device)
	}

	say("")
	say("Authenticated! Starting initial sync...")
	say("This may take a few minutes depending on your message history.")
	say("Press Ctrl+C to stop (you can resume sync later with 'whatsapp sync').")
	say("")

	// Wait for initial sync (with timeout)
	syncTimeout := 5 * time.Minute
	select {
	case <-ctx.Done():
		// User interrupted
		if events != nil {
			events.interrupted()
		}
	case <-client.SyncComplete:
		say("Initial sync complete.")
		if events != nil {
			events.syncComplete()
		}
	case <-time.After(syncTimeout):
		say("Initial sync timeout reached. You can continue syncing with 'whatsapp sync'.")
		if events != nil {
			events.syncTimeout()
		}
	}

	client.Disconnect()

	// The event stream already carried the result
	if events != nil {
		return nil
	}

	// Output result
	return OutputResult(map[string]any{
		"user":   user,
		"device": device,
	}, fmt.Sprintf("Authenticated as %s (device %d)", user, device))
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
