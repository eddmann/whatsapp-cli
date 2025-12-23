package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

var doctorConnect bool

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
