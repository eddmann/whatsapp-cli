package cli

import (
	"fmt"
	"os"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

// WithDB opens the database and runs the provided function.
// Performs auto-sync if last sync was over 24 hours ago.
func WithDB(fn func(*store.DB) error) error {
	if err := EnsureDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	db, err := store.Open(GetMessagesDBPath())
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.CloseQuietly()

	// Auto-sync if needed
	if err := maybeAutoSync(db); err != nil {
		fmt.Fprintf(os.Stderr, "Auto-sync warning: %v\n", err)
	}

	return fn(db)
}

// WithConnection opens the database, creates a WhatsApp client, verifies authentication,
// connects to WhatsApp, and runs the provided function.
// Performs auto-sync if last sync was over 24 hours ago.
func WithConnection(fn func(*store.DB, *whatsapp.Client) error) error {
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

	if err := client.Connect(); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer client.Disconnect()

	// Auto-sync if needed (using existing connection)
	if err := maybeAutoSyncWithClient(client, db); err != nil {
		fmt.Fprintf(os.Stderr, "Auto-sync warning: %v\n", err)
	}

	return fn(db, client)
}
