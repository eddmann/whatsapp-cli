package cli

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/eddmann/whatsapp-cli/internal/store"
)

func TestCheckFTS5(t *testing.T) {
	t.Run("usable on a correctly built binary", func(t *testing.T) {
		db, err := store.Open(filepath.Join(t.TempDir(), "messages.db"))
		if err != nil {
			t.Fatalf("store.Open: %v", err)
		}
		defer db.CloseQuietly()

		ok, errMsg := checkFTS5(db.Messages)
		if !ok {
			t.Fatalf("expected ok, got error: %q", errMsg)
		}
		if errMsg != "" {
			t.Fatalf("expected no error message, got %q", errMsg)
		}
	})

	// This cannot reproduce the exact upstream failure (a binary built
	// without -tags sqlite_fts5, since this test itself only runs under
	// that tag), but it does verify checkFTS5 queries the real table and
	// surfaces a real error rather than always reporting ok — a plain,
	// un-migrated database is the simplest stand-in for "messages_fts
	// doesn't actually work here."
	t.Run("fails when messages_fts does not exist", func(t *testing.T) {
		raw, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "empty.db"))
		if err != nil {
			t.Fatalf("sql.Open: %v", err)
		}
		defer func() { _ = raw.Close() }()

		ok, errMsg := checkFTS5(raw)
		if ok {
			t.Fatal("expected not ok against a database with no messages_fts table")
		}
		if errMsg == "" {
			t.Fatal("expected a non-empty error message")
		}
	})
}
