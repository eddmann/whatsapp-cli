package cli

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/eddmann/whatsapp-cli/internal/store"
)

func openTestDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "messages.db"))
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(db.CloseQuietly)
	return db
}

// The defect this guards against: the threshold was 24 hours, so a read taken
// between syncs served a store that was silently behind and said nothing about it.
// On 26 September 2026 that lost a voice message sent twelve minutes earlier.
func TestShouldAutoSyncEvenWhenTheStoreWasJustSynced(t *testing.T) {
	if autoSyncThreshold != 0 {
		t.Fatalf("autoSyncThreshold must stay 0 so every read syncs first; got %v", autoSyncThreshold)
	}

	db := openTestDB(t)
	noAutoSync = false
	t.Cleanup(func() { noAutoSync = false })

	for _, age := range []time.Duration{0, time.Second, time.Minute, time.Hour} {
		if err := db.SetLastSyncTime(time.Now().Add(-age)); err != nil {
			t.Fatalf("set last sync time: %v", err)
		}
		if !shouldAutoSync(db) {
			t.Errorf("a store synced %v ago must still sync before a read", age)
		}
	}
}

func TestNoAutoSyncFlagStillSuppressesTheSync(t *testing.T) {
	db := openTestDB(t)
	if err := db.SetLastSyncTime(time.Now().Add(-48 * time.Hour)); err != nil {
		t.Fatalf("set last sync time: %v", err)
	}

	noAutoSync = true
	t.Cleanup(func() { noAutoSync = false })

	if shouldAutoSync(db) {
		t.Error("--no-auto-sync must suppress the sync; it is the only way to read " +
			"the store while `whatsapp watch` holds the connection")
	}
}

func TestNeverSyncedStoreSyncs(t *testing.T) {
	db := openTestDB(t)
	noAutoSync = false
	t.Cleanup(func() { noAutoSync = false })

	if !shouldAutoSync(db) {
		t.Error("a store that has never been synced must sync")
	}
}
