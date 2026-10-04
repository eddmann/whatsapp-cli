package cli

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

var doctorConnect bool

// checkFTS5 runs a real MATCH query against messages_fts rather than trusting
// that the table existing in sqlite_master means the fts5 module is usable.
// CREATE VIRTUAL TABLE ... IF NOT EXISTS makes store.Open succeed even on a
// binary built without -tags sqlite_fts5, as long as messages_fts already
// exists from an earlier, correctly built binary — the failure then only
// surfaces later, silently, on every message INSERT via the messages_ai
// trigger. This is what actually caught that: a real query, not a schema check.
func checkFTS5(messages *sql.DB) (ok bool, errMsg string) {
	_, err := messages.Query(`SELECT rowid FROM messages_fts WHERE messages_fts MATCH 'zzz_doctor_fts5_probe' LIMIT 1`)
	if err != nil {
		return false, err.Error()
	}
	return true, ""
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run diagnostics",
	Long: `Run diagnostics to check the health of your WhatsApp CLI setup.

Checks:
- Config directory exists and is writable
- Database is accessible
- Session exists
- Connection to WhatsApp (with --connect)`,
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().BoolVar(&doctorConnect, "connect", false, "Also test connection to WhatsApp")
}

func runDoctor(cmd *cobra.Command, args []string) error {
	checks := []map[string]any{}

	// Check config directory
	configDir := GetConfigDir()
	configExists := true
	configWritable := true

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		configExists = false
		configWritable = false
	} else {
		// Try to write a test file
		testPath := configDir + "/.test"
		if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
			configWritable = false
		} else {
			_ = os.Remove(testPath)
		}
	}

	checks = append(checks, map[string]any{
		"name":   "Config Directory",
		"path":   configDir,
		"exists": configExists,
		"ok":     configWritable,
	})

	// Check store directory
	storeDir := GetStoreDir()
	storeExists := true
	if _, err := os.Stat(storeDir); os.IsNotExist(err) {
		storeExists = false
	}

	checks = append(checks, map[string]any{
		"name":   "Store Directory",
		"path":   storeDir,
		"exists": storeExists,
		"ok":     storeExists,
	})

	// Check messages database
	dbPath := GetMessagesDBPath()
	dbOK := false
	dbStats := map[string]int{}

	db, err := store.Open(dbPath)
	if err == nil {
		dbOK = true
		chats, _ := db.CountChats("")
		msgs, _ := db.CountMessages()
		dbStats["chats"] = chats
		dbStats["messages"] = msgs
		db.CloseQuietly()
	}

	checks = append(checks, map[string]any{
		"name":  "Messages Database",
		"path":  dbPath,
		"ok":    dbOK,
		"stats": dbStats,
	})

	// Check FTS5 is actually usable, not just that the schema exists.
	//
	// `messages_fts` is created with CREATE VIRTUAL TABLE ... IF NOT EXISTS,
	// so once it exists in sqlite_master, Open() succeeds and this whole
	// check block above stays green even on a binary built without
	// -tags sqlite_fts5 — the failure only shows up later, silently, on
	// every message INSERT via the messages_ai trigger, which is exactly
	// how a 2026-09 build silently dropped messages during sync/backfill
	// for hours at a time with no error surfaced anywhere else. Run a real
	// MATCH query so this check fails loudly instead.
	fts5OK, fts5Err := false, ""
	if dbOK {
		if db, err := store.Open(dbPath); err == nil {
			fts5OK, fts5Err = checkFTS5(db.Messages)
			db.CloseQuietly()
		}
	}

	fts5Check := map[string]any{
		"name": "Full-text search (FTS5)",
		"ok":   fts5OK,
	}
	if fts5Err != "" {
		fts5Check["error"] = fts5Err
		fts5Check["fix"] = "Rebuild with: CGO_ENABLED=1 go build -tags sqlite_fts5 -o ~/.local/bin/whatsapp ./cmd/whatsapp (or: make build). Check `which -a whatsapp` afterward — a plain `go build` or a stale Homebrew binary ahead on PATH silently reintroduces this."
	}
	checks = append(checks, fts5Check)

	// Check session
	sessionPath := GetSessionDBPath()
	sessionExists := false
	if _, err := os.Stat(sessionPath); err == nil {
		sessionExists = true
	}

	checks = append(checks, map[string]any{
		"name":   "Session Database",
		"path":   sessionPath,
		"exists": sessionExists,
		"ok":     sessionExists,
	})

	// Check authentication
	authenticated := false
	if db, err := store.Open(GetMessagesDBPath()); err == nil {
		if client, err := whatsapp.New(db, GetStoreDir(), false, nil); err == nil {
			authenticated = client.IsAuthenticated()
		}
		db.CloseQuietly()
	}

	checks = append(checks, map[string]any{
		"name": "Authenticated",
		"ok":   authenticated,
	})

	// Optional: test connection
	if doctorConnect && authenticated {
		connected := false
		loggedIn := false

		if db, err := store.Open(GetMessagesDBPath()); err == nil {
			if client, err := whatsapp.New(db, GetStoreDir(), IsVerbose(), nil); err == nil {
				if err := client.Connect(); err == nil {
					connected = client.IsConnected()
					loggedIn = client.IsLoggedIn()
					client.Disconnect()
				}
			}
			db.CloseQuietly()
		}

		checks = append(checks, map[string]any{
			"name":      "Connection Test",
			"connected": connected,
			"logged_in": loggedIn,
			"ok":        connected && loggedIn,
		})
	}

	// Summarize
	allOK := true
	for _, check := range checks {
		if ok, exists := check["ok"].(bool); exists && !ok {
			allOK = false
			break
		}
	}

	result := map[string]any{
		"checks":  checks,
		"healthy": allOK,
	}

	if IsJSON() {
		return Output(result)
	}

	{
		// Print human-readable summary
		fmt.Println("WhatsApp CLI Diagnostics")
		fmt.Println("========================")
		fmt.Println()

		for _, check := range checks {
			name := check["name"].(string)
			ok := false
			if v, exists := check["ok"].(bool); exists {
				ok = v
			}
			status := "FAIL"
			if ok {
				status = "OK"
			}
			fmt.Printf("[%s] %s\n", status, name)
			if path, exists := check["path"].(string); exists {
				fmt.Printf("      Path: %s\n", path)
			}
		}

		fmt.Println()
		if allOK {
			fmt.Println("All checks passed!")
		} else {
			fmt.Println("Some checks failed. Run 'whatsapp auth login' if not authenticated.")
		}
	}

	return nil
}
