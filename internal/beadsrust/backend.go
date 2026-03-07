package beadsrust

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/zjrosen/perles/internal/beads/bql"
	"github.com/zjrosen/perles/internal/cachemanager"
	"github.com/zjrosen/perles/internal/task"
)

// Compile-time check that BeadsRustBackend implements task.Backend.
var _ task.Backend = (*BeadsRustBackend)(nil)

// BeadsRustBackend implements task.Backend for the beads_rust (br) issue tracker.
// It uses direct SQLite access for reads (fast) and the br CLI for writes
// (to maintain consistency with br's audit trail and event system).
// Queries are processed through the BQL engine with beads_rust-specific validation.
type BeadsRustBackend struct {
	reader        *SQLiteReader
	services      task.BackendServices
	bqlCache      cachemanager.Flushable
	depGraphCache cachemanager.Flushable
}

// NewBeadsRustBackend creates a new beads_rust backend.
// dataDir is the path to the .beads directory containing beads.db.
// workDir is the working directory for br CLI execution.
func NewBeadsRustBackend(dataDir, workDir string) (*BeadsRustBackend, error) {
	// Open SQLite database for reads.
	reader, err := NewSQLiteReader(dataDir)
	if err != nil {
		return nil, fmt.Errorf("beads_rust backend: %w", err)
	}

	// Create CLI executor for writes.
	writer := NewBRExecutor(workDir, dataDir)

	// Compose the task executor (reads from DB, writes via CLI).
	taskExec := newTaskExecutor(reader, writer)

	// Create BQL caches.
	bqlCache := cachemanager.NewInMemoryCacheManager[string, []task.Issue](
		"beads_rust-bql-cache",
		cachemanager.DefaultExpiration,
		cachemanager.DefaultCleanupInterval,
	)
	depGraphCache := cachemanager.NewInMemoryCacheManager[string, *bql.DependencyGraph](
		"beads_rust-dep-cache",
		cachemanager.DefaultExpiration,
		cachemanager.DefaultCleanupInterval,
	)

	// Create BQL executor (SQL-based via shared BQL engine).
	bqlExec := NewBQLExecutor(reader.DB(), bqlCache, depGraphCache)
	queryExec := NewBRQueryExecutor(bqlExec)

	b := &BeadsRustBackend{
		reader:        reader,
		bqlCache:      bqlCache,
		depGraphCache: depGraphCache,
		services: task.BackendServices{
			TaskExecutor:      taskExec,
			QueryExecutor:     queryExec,
			QueryHelpers:      NewBRQueryHelpers(),
			SyntaxHighlighter: NewBRSyntaxHighlighter(),
			Capabilities:      beadsRustCapabilities(),
			WatcherConfig: task.WatcherConfig{
				RelevantFiles:    []string{"beads.db", "beads.db-wal"},
				DebounceDuration: 100 * time.Millisecond,
			},
			DBPath: reader.DBPath(),
		},
	}

	return b, nil
}

// Services returns the backend service bundle.
func (b *BeadsRustBackend) Services() task.BackendServices {
	return b.services
}

// CheckCompatibility verifies that the br CLI is installed and accessible.
func (b *BeadsRustBackend) CheckCompatibility() error {
	_, err := exec.LookPath("br")
	if err != nil {
		return fmt.Errorf("br CLI not found in PATH: %w", err)
	}
	return nil
}

// FlushCaches invalidates the BQL and dependency-graph caches so that
// subsequent queries hit the database instead of returning stale results.
func (b *BeadsRustBackend) FlushCaches(ctx context.Context) error {
	if err := b.bqlCache.Flush(ctx); err != nil {
		return fmt.Errorf("flushing BQL cache: %w", err)
	}
	if err := b.depGraphCache.Flush(ctx); err != nil {
		return fmt.Errorf("flushing dep graph cache: %w", err)
	}
	return nil
}

// Close closes the SQLite database connection.
func (b *BeadsRustBackend) Close() error {
	return b.reader.Close()
}

// beadsRustCapabilities returns the capabilities of the beads_rust backend.
func beadsRustCapabilities() task.BackendCapabilities {
	return task.BackendCapabilities{
		SupportsQuery:        true,
		QueryLanguageName:    "BQL",
		SupportsDependencies: true,
		SupportsTree:         true,
		SupportsComments:     true,
		SupportsLabels:       true,
		SupportsPriority:     true,
		SupportsDesignField:  true,
		SupportsNotesField:   true,
	}
}
