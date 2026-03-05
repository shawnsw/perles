// Package watcher provides file system watching with debouncing for backend data stores.
package watcher

import (
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/zjrosen/perles/internal/log"
	"github.com/zjrosen/perles/internal/pubsub"

	"github.com/fsnotify/fsnotify"
)

// WatcherEventType identifies the kind of watcher event.
type WatcherEventType string

const (
	// DBChanged is emitted when the database file changes (after debounce).
	DBChanged WatcherEventType = "db_changed"
	// WatcherError is emitted when the watcher encounters an error (immediate, not debounced).
	WatcherError WatcherEventType = "error"
)

// WatcherEvent represents an event from the database watcher.
type WatcherEvent struct {
	Type  WatcherEventType
	Error error // Non-nil for WatcherError events
}

// Watcher monitors a backend data store for changes and publishes events via broker.
type Watcher struct {
	fsWatcher     *fsnotify.Watcher
	dbPath        string
	relevantFiles []string // Base filenames that trigger a refresh (e.g. "beads.db", "beads.db-wal")
	debounce      time.Duration
	done          chan struct{}
	broker        *pubsub.Broker[WatcherEvent]
}

// Config holds watcher configuration options.
type Config struct {
	// DBPath is the path to the data store file/directory. The watcher watches
	// the parent directory of this path for filesystem events.
	DBPath string

	// DebounceDur is how long to wait after the last filesystem event before
	// publishing a DBChanged notification. This coalesces rapid writes.
	DebounceDur time.Duration

	// RelevantFiles lists the base filenames in the watched directory that
	// should trigger a refresh. For example: ["beads.db", "beads.db-wal"]
	// for SQLite, or ["last-touched"] for Dolt sentinel-based watching.
	// If empty, all write/create events in the watched directory trigger a refresh.
	RelevantFiles []string
}

// New creates a new database watcher.
func New(cfg Config) (*Watcher, error) {
	log.Debug(log.CatWatcher, "Creating watcher", "dbPath", cfg.DBPath, "debounce", cfg.DebounceDur)
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		log.ErrorErr(log.CatWatcher, "Failed to create fsnotify watcher", err)
		return nil, fmt.Errorf("creating fsnotify watcher: %w", err)
	}

	return &Watcher{
		fsWatcher:     fsw,
		dbPath:        cfg.DBPath,
		relevantFiles: cfg.RelevantFiles,
		debounce:      cfg.DebounceDur,
		done:          make(chan struct{}),
		broker:        pubsub.NewBroker[WatcherEvent](),
	}, nil
}

// watchDir returns the directory to watch.
// The watcher monitors the parent directory of dbPath for filesystem events.
func (w *Watcher) watchDir() string {
	return filepath.Dir(w.dbPath)
}

// Start begins watching the database directory.
// Subscribe to watcher events using Broker().Subscribe(ctx) instead of the old channel return.
func (w *Watcher) Start() error {
	dir := w.watchDir()
	if err := w.fsWatcher.Add(dir); err != nil {
		log.ErrorErr(log.CatWatcher, "Failed to watch directory", err, "dir", dir)
		return fmt.Errorf("watching directory %s: %w", dir, err)
	}

	log.Info(log.CatWatcher, "Started watching", "dir", dir)
	go w.loop()

	return nil
}

// Stop terminates the watcher and releases resources.
// CRITICAL SHUTDOWN SEQUENCE: broker.Close() must be called BEFORE fsWatcher.Close().
// This ensures subscribers receive clean channel close notifications before the underlying
// fsnotify watcher is destroyed. Reversing this order could leave subscribers hanging.
func (w *Watcher) Stop() error {
	log.Debug(log.CatWatcher, "Stopping watcher")
	close(w.done)
	w.broker.Close() // Close broker first to notify subscribers
	return w.fsWatcher.Close()
}

// Broker returns the pub/sub broker for subscribing to watcher events.
// The broker is created in New(), so it is always valid even before Start() is called.
func (w *Watcher) Broker() *pubsub.Broker[WatcherEvent] {
	return w.broker
}

// loop processes file system events with debouncing.
func (w *Watcher) loop() {
	var (
		timer   *time.Timer
		pending bool
	)

	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}

			// Only react to writes on database files
			if !w.isRelevantEvent(event) {
				continue
			}

			log.Debug(log.CatWatcher, "File event received", "file", event.Name, "op", event.Op.String())

			// Reset or start debounce timer
			if timer == nil {
				log.Debug(log.CatWatcher, "Starting debounce timer", "duration", w.debounce)
				timer = time.NewTimer(w.debounce)
				pending = true
			} else {
				if !timer.Stop() {
					// Drain the timer channel if it already fired
					select {
					case <-timer.C:
					default:
					}
				}
				log.Debug(log.CatWatcher, "Resetting debounce timer", "duration", w.debounce)
				timer.Reset(w.debounce)
				pending = true
			}

		case <-func() <-chan time.Time {
			if timer != nil {
				return timer.C
			}
			return nil
		}():
			if pending {
				log.Debug(log.CatWatcher, "Debounce complete, triggering refresh")
				// Publish DBChanged event to broker (non-blocking by design)
				w.broker.Publish(pubsub.UpdatedEvent, WatcherEvent{
					Type: DBChanged,
				})
				pending = false
			}

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			// Log error AND publish error event (immediate, not debounced)
			log.ErrorErr(log.CatWatcher, "File watcher error", err)
			w.broker.Publish(pubsub.UpdatedEvent, WatcherEvent{
				Type:  WatcherError,
				Error: err,
			})

		case <-w.done:
			if timer != nil {
				timer.Stop()
			}
			return
		}
	}
}

// isRelevantEvent checks if the event should trigger a refresh.
// Only write/create operations on files listed in RelevantFiles are considered.
// If RelevantFiles is empty, all write/create events trigger a refresh.
func (w *Watcher) isRelevantEvent(event fsnotify.Event) bool {
	// Only care about write or create operations
	if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
		return false
	}

	// If no relevant files specified, all writes trigger refresh
	if len(w.relevantFiles) == 0 {
		return true
	}

	return slices.Contains(w.relevantFiles, filepath.Base(event.Name))
}
