package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

const autoSyncThreshold = 24 * time.Hour
const autoSyncTimeout = 30 * time.Second

// shouldAutoSync checks if an auto-sync is needed based on last sync time.
func shouldAutoSync(db *store.DB) bool {
	if NoAutoSync() {
		return false
	}

	lastSync, err := db.GetLastSyncTime()
	if err != nil {
		return false
	}

	// Never synced or sync is stale
	if lastSync.IsZero() {
		return true
	}

	return time.Since(lastSync) > autoSyncThreshold
}

// formatTimeSince returns a human-readable duration since the given time.
func formatTimeSince(t time.Time) string {
	if t.IsZero() {
		return "never"
	}

	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

// maybeAutoSync performs a sync if needed (for WithDB commands that don't have a client).
// Creates a temporary connection, syncs, then disconnects.
func maybeAutoSync(db *store.DB) error {
	if !shouldAutoSync(db) {
		return nil
	}

	lastSync, _ := db.GetLastSyncTime()
	fmt.Fprintf(os.Stderr, "Auto-syncing (last sync: %s)...\n", formatTimeSince(lastSync))

	// Create a temporary client for syncing
	client, err := whatsapp.New(db, GetStoreDir(), IsVerbose(), nil)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Check if authenticated
	if !client.IsAuthenticated() {
		// Not authenticated - skip auto-sync silently
		return nil
	}

	// Connect
	if err := client.Connect(); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer client.Disconnect()

	// Perform the quick sync
	return performQuickSync(client, db)
}

// maybeAutoSyncWithClient performs a sync if needed, using an existing client connection.
func maybeAutoSyncWithClient(client *whatsapp.Client, db *store.DB) error {
	if !shouldAutoSync(db) {
		return nil
	}

	lastSync, _ := db.GetLastSyncTime()
	fmt.Fprintf(os.Stderr, "Auto-syncing (last sync: %s)...\n", formatTimeSince(lastSync))

	return performQuickSync(client, db)
}

// performQuickSync waits for sync events with a timeout.
func performQuickSync(client *whatsapp.Client, db *store.DB) error {
	select {
	case <-client.SyncComplete:
		fmt.Fprintln(os.Stderr, "Sync complete.")
	case <-time.After(autoSyncTimeout):
		fmt.Fprintln(os.Stderr, "Sync timeout (continuing with available data).")
	}

	// Update last sync time regardless of timeout
	if err := db.SetLastSyncTime(time.Now()); err != nil {
		return fmt.Errorf("failed to update sync time: %w", err)
	}

	return nil
}
