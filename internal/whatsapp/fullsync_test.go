package whatsapp

import (
	"testing"

	"go.mau.fi/whatsmeow/store"
)

func TestRequestFullHistorySyncSetsDeviceProps(t *testing.T) {
	prevRequire := store.DeviceProps.RequireFullSync
	prevDays := store.DeviceProps.HistorySyncConfig.FullSyncDaysLimit
	t.Cleanup(func() {
		store.DeviceProps.RequireFullSync = prevRequire
		store.DeviceProps.HistorySyncConfig.FullSyncDaysLimit = prevDays
	})

	RequestFullHistorySync()

	if !store.DeviceProps.GetRequireFullSync() {
		t.Fatalf("expected RequireFullSync to be true")
	}
	if got := store.DeviceProps.GetHistorySyncConfig().GetFullSyncDaysLimit(); got != fullSyncDaysLimit {
		t.Fatalf("expected FullSyncDaysLimit %d, got %d", fullSyncDaysLimit, got)
	}
}
