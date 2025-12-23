package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
)

var customStoreDir string

// SetStoreDir sets a custom store directory
func SetStoreDir(dir string) {
	customStoreDir = dir
}

// GetConfigDir returns the config directory path
// Uses XDG_CONFIG_HOME if set, otherwise ~/.config/whatsapp-cli
func GetConfigDir() string {
	if customStoreDir != "" {
		return customStoreDir
	}

	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "whatsapp-cli")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ".whatsapp-cli"
	}

	return filepath.Join(home, ".config", "whatsapp-cli")
}

// GetStoreDir returns the store directory for databases
func GetStoreDir() string {
	return filepath.Join(GetConfigDir(), "store")
}

// GetSessionDBPath returns the path to the whatsmeow session database
func GetSessionDBPath() string {
	return filepath.Join(GetStoreDir(), "session.db")
}

// GetMessagesDBPath returns the path to the messages database
func GetMessagesDBPath() string {
	return filepath.Join(GetStoreDir(), "messages.db")
}

// GetMediaDir returns the directory for downloaded media
func GetMediaDir() string {
	return filepath.Join(GetStoreDir(), "media")
}

// GetAliasesPath returns the path to the aliases file
func GetAliasesPath() string {
	return filepath.Join(GetConfigDir(), "aliases.json")
}

// EnsureDirectories creates all necessary directories
func EnsureDirectories() error {
	dirs := []string{
		GetConfigDir(),
		GetStoreDir(),
		GetMediaDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}

	return nil
}

// Aliases manages local JID aliases
type Aliases map[string]string // alias -> JID

// LoadAliases loads aliases from disk
func LoadAliases() (Aliases, error) {
	path := GetAliasesPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(Aliases), nil
		}
		return nil, err
	}

	var aliases Aliases
	if err := json.Unmarshal(data, &aliases); err != nil {
		return nil, err
	}

	return aliases, nil
}

// Save saves aliases to disk
func (a Aliases) Save() error {
	path := GetAliasesPath()

	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// Set adds or updates an alias
func (a Aliases) Set(alias, jid string) {
	a[alias] = jid
}

// Remove removes an alias
func (a Aliases) Remove(alias string) {
	delete(a, alias)
}

// Get returns the JID for an alias, or the input if not found
func (a Aliases) Get(aliasOrJID string) string {
	if jid, ok := a[aliasOrJID]; ok {
		return jid
	}
	return aliasOrJID
}

// JIDAlias maps JIDs to their aliases (reversed from Aliases)
type JIDAlias map[string]string // JID -> alias

// ListAliases returns all aliases as JID -> alias map
func ListAliases() (JIDAlias, error) {
	aliases, err := LoadAliases()
	if err != nil {
		return nil, err
	}

	result := make(JIDAlias)
	for alias, jid := range aliases {
		result[jid] = alias
	}
	return result, nil
}

// SetAlias sets an alias for a JID
func SetAlias(jid, alias string) error {
	aliases, err := LoadAliases()
	if err != nil {
		return err
	}
	aliases.Set(alias, jid)
	return aliases.Save()
}

// RemoveAlias removes an alias for a JID
func RemoveAlias(jid string) error {
	aliases, err := LoadAliases()
	if err != nil {
		return err
	}

	// Find and remove the alias for this JID
	for alias, j := range aliases {
		if j == jid {
			aliases.Remove(alias)
			break
		}
	}
	return aliases.Save()
}
