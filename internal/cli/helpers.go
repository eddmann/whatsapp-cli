package cli

import (
	"fmt"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

// WithDB opens the database and runs the provided function.
func WithDB(fn func(*store.DB) error) error {
	db, err := store.Open(GetMessagesDBPath())
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.CloseQuietly()

	return fn(db)
}

// WithConnection opens the database, creates a WhatsApp client, verifies authentication,
// connects to WhatsApp, and runs the provided function.
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

	return fn(db, client)
}
