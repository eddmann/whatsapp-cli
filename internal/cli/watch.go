package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/spf13/cobra"
	"go.mau.fi/whatsmeow/types"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

var watchCmd = &cobra.Command{
	Use:   "watch [jid]",
	Short: "Stream incoming messages as JSON lines",
	Long: `Connect to WhatsApp and print every real-time message as one JSON object per
line, in the same shape as 'messages -f jsonl'. Messages are also stored, exactly
as 'sync --follow' does. Give a chat JID to print only that chat.

Chat presence (someone typing, or recording a voice note) is printed as a
separate line shaped {"activity": {...}} with state "composing" or "paused" and
media "audio" while recording. Messages keep their existing shape.

WhatsApp tends to deliver presence only to a client that is online. --presence
marks this session available for as long as watch runs, which shows the account
as online to contacts, and sets it unavailable again on exit. Without the flag
nothing visible about the account changes, and presence arrives only when the
server sends it anyway (for example while the phone app is open).

Only one process can hold the WhatsApp connection. While watch is running, read
with --no-auto-sync so a read does not open a second connection.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runWatch,
}

var watchPresence bool

func init() {
	watchCmd.Flags().BoolVar(&watchPresence, "presence", false,
		"mark this session online while watching so typing and recording updates arrive (visible to contacts)")
	rootCmd.AddCommand(watchCmd)
}

func runWatch(cmd *cobra.Command, args []string) error {
	filter := ""
	if len(args) == 1 {
		filter = args[0]
	}

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
		return fmt.Errorf("not authenticated. Run 'whatsapp auth login' first")
	}

	onMessage, onActivity := newWatchEmitters(os.Stdout, filter)
	client.OnMessage = onMessage
	client.OnActivity = onActivity

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		signal.Stop(sigChan)
		cancel()
	}()

	if err := client.Connect(); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	if watchPresence {
		if err := client.WA.SendPresence(ctx, types.PresenceAvailable); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not mark session available: %v\n", err)
		} else {
			fmt.Fprintln(os.Stderr, "Session marked available (--presence); contacts can see the account online.")
		}
	}
	// Subscribing to a person's presence is a server request, not something the
	// other side sees. It only applies to one-to-one chats; group chatstates are
	// delivered to members without a subscription.
	if jid, err := types.ParseJID(filter); err == nil && filter != "" && jid.Server != types.GroupServer {
		if err := client.WA.SubscribePresence(ctx, jid); err != nil {
			fmt.Fprintf(os.Stderr, "warning: presence subscription failed: %v\n", err)
		}
	}
	if filter != "" {
		fmt.Fprintf(os.Stderr, "Watching %s for new messages (Ctrl-C to stop)…\n", filter)
	} else {
		fmt.Fprintln(os.Stderr, "Watching all chats for new messages (Ctrl-C to stop)…")
	}

	<-ctx.Done()
	if watchPresence {
		_ = client.WA.SendPresence(context.Background(), types.PresenceUnavailable)
	}
	client.Disconnect()
	return nil
}

// newWatchEmitter returns the OnMessage callback for watch. Kept for callers
// and tests that only need messages.
func newWatchEmitter(w io.Writer, filter string) func(store.Message) {
	onMessage, _ := newWatchEmitters(w, filter)
	return onMessage
}

// newWatchEmitters returns the OnMessage and OnActivity callbacks for watch.
// Both write one JSON line per event from the filtered chat (every chat when
// filter is empty) and share one lock, because whatsmeow may deliver events
// concurrently. Activity lines are wrapped as {"activity": {...}} so a reader
// can tell them from messages; our own presence is dropped.
func newWatchEmitters(w io.Writer, filter string) (func(store.Message), func(whatsapp.Activity)) {
	var mu sync.Mutex
	enc := json.NewEncoder(w)
	onMessage := func(m store.Message) {
		if filter != "" && m.ChatJID != filter {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		_ = enc.Encode(m)
	}
	onActivity := func(a whatsapp.Activity) {
		if a.IsFromMe || (filter != "" && a.ChatJID != filter && a.ChatAlt != filter) {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		_ = enc.Encode(struct {
			Activity whatsapp.Activity `json:"activity"`
		}{a})
	}
	return onMessage, onActivity
}
