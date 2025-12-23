package store

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the messages database connection.
type DB struct {
	Messages *sql.DB
}

// Open opens the messages database at the given path.
func Open(dbPath string) (*DB, error) {
	dir := dbPath[:strings.LastIndex(dbPath, "/")]
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create db dir: %w", err)
	}

	connStr := fmt.Sprintf("file:%s?_foreign_keys=on", dbPath)
	mdb, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open messages db: %w", err)
	}

	// Configure for SQLite single-writer limitation
	mdb.SetMaxOpenConns(1)

	if err := migrate(mdb); err != nil {
		_ = mdb.Close()
		return nil, err
	}

	return &DB{Messages: mdb}, nil
}

// Close closes all database connections.
func (d *DB) Close() error {
	if d == nil {
		return nil
	}
	if d.Messages != nil {
		return d.Messages.Close()
	}
	return nil
}

// CloseQuietly closes all database connections, ignoring any errors.
func (d *DB) CloseQuietly() {
	_ = d.Close()
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS chats (
			jid TEXT PRIMARY KEY,
			name TEXT,
			last_message_time TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS messages (
			id TEXT,
			chat_jid TEXT,
			sender TEXT,
			sender_name TEXT,
			content TEXT,
			timestamp TIMESTAMP,
			is_from_me BOOLEAN,
			media_type TEXT,
			filename TEXT,
			url TEXT,
			media_key BLOB,
			file_sha256 BLOB,
			file_enc_sha256 BLOB,
			file_length INTEGER,
			PRIMARY KEY (id, chat_jid),
			FOREIGN KEY (chat_jid) REFERENCES chats(jid)
		);

		CREATE TABLE IF NOT EXISTS lid_mappings (
			lid TEXT PRIMARY KEY,
			phone TEXT,
			name TEXT,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Create FTS5 virtual table for full-text search
	if _, err := db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
		content,
		content='messages',
		content_rowid='rowid'
	);`); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "fts5") || strings.Contains(strings.ToLower(err.Error()), "no such module") {
			return fmt.Errorf("SQLite FTS5 is not available. Build with: CGO_ENABLED=1 go build -tags sqlite_fts5")
		}
		return err
	}

	// Create triggers to keep FTS index in sync
	if _, err := db.Exec(`CREATE TRIGGER IF NOT EXISTS messages_ai AFTER INSERT ON messages BEGIN
		INSERT INTO messages_fts(rowid, content) VALUES (new.rowid, new.content);
	END;`); err != nil {
		return err
	}

	if _, err := db.Exec(`CREATE TRIGGER IF NOT EXISTS messages_ad AFTER DELETE ON messages BEGIN
		INSERT INTO messages_fts(messages_fts, rowid) VALUES('delete', old.rowid);
	END;`); err != nil {
		return err
	}

	if _, err := db.Exec(`CREATE TRIGGER IF NOT EXISTS messages_au AFTER UPDATE ON messages BEGIN
		INSERT INTO messages_fts(messages_fts, rowid) VALUES('delete', old.rowid);
		INSERT INTO messages_fts(rowid, content) VALUES (new.rowid, new.content);
	END;`); err != nil {
		return err
	}

	// Verify FTS table exists
	var tbl string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='messages_fts'`).Scan(&tbl); err != nil {
		return fmt.Errorf("messages_fts not present after migration: %w", err)
	}

	// Rebuild index to sync with existing messages
	_, _ = db.Exec(`INSERT INTO messages_fts(messages_fts) VALUES('rebuild')`)

	// Add sender_name column if it doesn't exist (for existing databases)
	_, _ = db.Exec(`ALTER TABLE messages ADD COLUMN sender_name TEXT`)

	return nil
}

// CountChats returns the total number of chats matching the query.
func (d *DB) CountChats(query string) (int, error) {
	var count int
	var err error

	if query == "" {
		err = d.Messages.QueryRow("SELECT COUNT(*) FROM chats").Scan(&count)
	} else {
		pattern := "%" + strings.ToLower(query) + "%"
		err = d.Messages.QueryRow("SELECT COUNT(*) FROM chats WHERE LOWER(name) LIKE ? OR jid LIKE ?", pattern, pattern).Scan(&count)
	}

	return count, err
}

// CountMessages returns the total number of messages.
func (d *DB) CountMessages() (int, error) {
	var count int
	err := d.Messages.QueryRow("SELECT COUNT(*) FROM messages").Scan(&count)
	return count, err
}

// StoreLIDMapping stores a LID -> phone/name mapping.
func (d *DB) StoreLIDMapping(lid, phone, name string) error {
	_, err := d.Messages.Exec(`
		INSERT INTO lid_mappings (lid, phone, name, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(lid) DO UPDATE SET
			phone = COALESCE(NULLIF(excluded.phone, ''), phone),
			name = COALESCE(NULLIF(excluded.name, ''), name),
			updated_at = CURRENT_TIMESTAMP
	`, lid, phone, name)
	return err
}

// GetLIDMapping retrieves a LID mapping.
func (d *DB) GetLIDMapping(lid string) (phone, name string, found bool) {
	var p, n sql.NullString
	err := d.Messages.QueryRow("SELECT phone, name FROM lid_mappings WHERE lid = ?", lid).Scan(&p, &n)
	if err != nil {
		return "", "", false
	}
	return p.String, n.String, true
}

// ResolveSenderName tries to resolve a sender identifier to a display name.
func (d *DB) ResolveSenderName(sender string) string {
	// First check lid_mappings
	if _, n, found := d.GetLIDMapping(sender); found && n != "" {
		return n
	}

	// Then check if there's a chat with this JID that has a name
	var chatName sql.NullString
	jid := sender
	if !strings.Contains(sender, "@") {
		jid = sender + "@s.whatsapp.net"
	}
	_ = d.Messages.QueryRow("SELECT name FROM chats WHERE jid = ?", jid).Scan(&chatName)
	if chatName.Valid && chatName.String != "" {
		return chatName.String
	}

	return ""
}
