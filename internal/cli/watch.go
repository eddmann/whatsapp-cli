package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"

	"go.mau.fi/whatsmeow/types/events"
)

// Terminal exit codes. A supervisor watching this process should stop
// (or long-backoff) retrying on these: retrying cannot fix them.
const (
	watchExitLoggedOut      = 3 // session invalidated; needs QR re-pair
	watchExitTemporaryBan   = 4 // server-side temporary ban
	watchExitClientOutdated = 5 // protocol too old; rebuild with whatsmeow bump
	watchExitStreamReplaced = 6 // another connection took over this device session
)

var watchJIDs []string

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Stream live events as JSON lines on stdout",
	Long: `Connect to WhatsApp and stream real-time events as JSON lines (JSONL) on stdout.

Designed to run as a long-lived companion process against its own --store
directory (a separate linked device): one JSON object per line for message,
connected, disconnected, keepalive_timeout, offline_sync_completed and
terminal events. It never sends messages and never mutates any other store.

Auto-reconnects on unclean disconnects. Terminal states exit non-zero:
3 = logged out, 4 = temporary ban, 5 = client outdated, 6 = stream replaced.`,
	RunE: runWatch,
}

func init() {
	rootCmd.AddCommand(watchCmd)
	watchCmd.Flags().StringArrayVar(&watchJIDs, "jid", nil, "Only emit messages for these chat JIDs (repeatable; default: all)")
}

type watchEvent struct {
	Event   string                       `json:"event"`
	TS      time.Time                    `json:"ts"`
	Message *whatsapp.MessageDescription `json:"message,omitempty"`
	Reason  string                       `json:"reason,omitempty"`
	Count   int                          `json:"count,omitempty"`
}

func emitWatchEvent(ev watchEvent) {
	if ev.TS.IsZero() {
		ev.TS = time.Now()
	}
	// One line per event; write errors (e.g. closed pipe) are fatal for a stream process.
	if err := json.NewEncoder(os.Stdout).Encode(ev); err != nil {
		fmt.Fprintf(os.Stderr, "watch: stdout write failed: %v\n", err)
		os.Exit(1)
	}
}

func runWatch(cmd *cobra.Command, args []string) error {
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
		return fmt.Errorf("not authenticated. Run 'whatsapp auth login' first (with the same --store)")
	}

	client.WA.EnableAutoReconnect = true

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		signal.Stop(sigChan)
		fmt.Fprintln(os.Stderr, "\nInterrupted, disconnecting...")
		cancel()
	}()

	terminal := make(chan int, 1)

	jidSet := make(map[string]bool, len(watchJIDs))
	for _, j := range watchJIDs {
		jidSet[j] = true
	}

	client.WA.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			if len(jidSet) > 0 && !jidSet[v.Info.Chat.String()] {
				return
			}
			desc := client.DescribeMessage(v)
			emitWatchEvent(watchEvent{Event: "message", TS: time.Now(), Message: &desc})
		case *events.Connected:
			emitWatchEvent(watchEvent{Event: "connected"})
		case *events.Disconnected:
			emitWatchEvent(watchEvent{Event: "disconnected", Reason: "socket closed; auto-reconnect pending"})
		case *events.KeepAliveTimeout:
			emitWatchEvent(watchEvent{Event: "keepalive_timeout", Reason: fmt.Sprintf("error count %d", v.ErrorCount)})
		case *events.OfflineSyncCompleted:
			emitWatchEvent(watchEvent{Event: "offline_sync_completed", Count: v.Count})
		case *events.StreamReplaced:
			emitWatchEvent(watchEvent{Event: "stream_replaced", Reason: "another connection took over this device session"})
			select {
			case terminal <- watchExitStreamReplaced:
			default:
			}
		case *events.LoggedOut:
			emitWatchEvent(watchEvent{Event: "logged_out", Reason: fmt.Sprintf("on_connect=%v reason=%v", v.OnConnect, v.Reason)})
			select {
			case terminal <- watchExitLoggedOut:
			default:
			}
		case *events.TemporaryBan:
			emitWatchEvent(watchEvent{Event: "temporary_ban", Reason: v.String()})
			select {
			case terminal <- watchExitTemporaryBan:
			default:
			}
		case *events.ClientOutdated:
			emitWatchEvent(watchEvent{Event: "client_outdated", Reason: "rebuild with a newer whatsmeow"})
			select {
			case terminal <- watchExitClientOutdated:
			default:
			}
		}
	})

	if err := client.Connect(); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	emitWatchEvent(watchEvent{Event: "started"})

	select {
	case <-ctx.Done():
	case code := <-terminal:
		client.Disconnect()
		fmt.Fprintf(os.Stderr, "watch: terminal state, exiting %d\n", code)
		os.Exit(code)
	}

	client.Disconnect()
	return nil
}
