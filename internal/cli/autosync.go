package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

// autoSyncThreshold is the age a sync may reach before a read triggers a new one.
//
// It is zero, which means every read syncs first. That is deliberate and it is the
// whole point of this file. It used to be 24 hours, and the effect was that any read
// taken between syncs served a store that was silently behind, with nothing in the
// output able to say so. On 26 September 2026 a chat read at 16:01 missed a voice
// message that had arrived at 15:49, reported the chat as current, and the gap was
// found only because a human said "you need to sync I think".
//
// A stale read that announces itself is an inconvenience. A stale read that looks
// complete is a wrong answer, and this tool is used to decide what to reply to people.
// Correctness wins over the round trip; a warm sync costs about 9 seconds.
//
// Escape hatch: --no-auto-sync. Required while `whatsapp watch` holds the connection,
// because only one process can hold it and every read now wants one.
const autoSyncThreshold = 0
const autoSyncTimeout = 30 * time.Second

// shouldAutoSync reports whether this command should sync before touching the store.
// With the threshold at zero this is true for every read unless --no-auto-sync was
// passed. The last-sync time is still consulted so the "last sync: ..." line can say
// how far behind the store actually was.
func shouldAutoSync(db *store.DB) bool {
	if NoAutoSync() {
		return false
	}

	lastSync, err := db.GetLastSyncTime()
	if err != nil {
		// Cannot tell how old the store is, so assume the worst and sync.
		return true
	}

	if lastSync.IsZero() {
		return true
	}

	return time.Since(lastSync) >= autoSyncThreshold
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
		// Do not record a sync time here. A sync that timed out fetched an unknown
		// amount, and writing the clock forward on it is how a failed sync used to
		// buy itself another full threshold of silence. The warning is the output.
		fmt.Fprintln(os.Stderr, "Sync timeout: the store may be behind. Re-run, or pass --no-auto-sync to read it as-is.")
		return nil
	}

	if err := db.SetLastSyncTime(time.Now()); err != nil {
		return fmt.Errorf("failed to update sync time: %w", err)
	}

	return nil
}
