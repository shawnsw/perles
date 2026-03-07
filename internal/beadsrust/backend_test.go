package beadsrust

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewBeadsRustBackend_MissingDB(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "beadsrust-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	_, err = NewBeadsRustBackend(tmpDir, tmpDir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "beads_rust")
}

func TestNewBeadsRustBackend_Services(t *testing.T) {
	backend := newTestBackend(t)
	services := backend.Services()

	// Verify all services are wired.
	require.NotNil(t, services.TaskExecutor)
	require.NotNil(t, services.QueryExecutor)
	require.NotNil(t, services.QueryHelpers)
	require.NotNil(t, services.SyntaxHighlighter, "beads_rust has BQL syntax highlighting")

	// Verify capabilities.
	require.True(t, services.Capabilities.SupportsQuery)
	require.Equal(t, "BQL", services.Capabilities.QueryLanguageName)
	require.True(t, services.Capabilities.SupportsDependencies)
	require.True(t, services.Capabilities.SupportsTree)
	require.True(t, services.Capabilities.SupportsComments)
	require.True(t, services.Capabilities.SupportsLabels)
	require.True(t, services.Capabilities.SupportsPriority)
	require.True(t, services.Capabilities.SupportsDesignField)
	require.True(t, services.Capabilities.SupportsNotesField)

	// Verify watcher config.
	require.Equal(t, []string{"beads.db", "beads.db-wal"}, services.WatcherConfig.RelevantFiles)
	require.Equal(t, 100*time.Millisecond, services.WatcherConfig.DebounceDuration)

	// Verify DB path points to the actual file.
	require.Contains(t, services.DBPath, "beads.db")
}

func TestBeadsRustBackend_FlushCaches(t *testing.T) {
	backend := newTestBackend(t)

	// FlushCaches should not error.
	require.NoError(t, backend.FlushCaches(context.Background()))

	// Execute a query to populate caches.
	services := backend.Services()
	_, err := services.QueryExecutor.Execute("status = open")
	require.NoError(t, err)

	// Flush again after populated caches.
	require.NoError(t, backend.FlushCaches(context.Background()))

	// Query should still work after cache flush.
	issues, err := services.QueryExecutor.Execute("status = open")
	require.NoError(t, err)
	require.Len(t, issues, 5) // 5 open issues in seed data.
}

func TestBeadsRustBackend_Close(t *testing.T) {
	backend := newTestBackend(t)

	// Close should close the DB connection without error.
	require.NoError(t, backend.Close())
}

func TestBeadsRustBackend_QueryExecutor_Integration(t *testing.T) {
	backend := newTestBackend(t)
	services := backend.Services()

	// Various queries through the full stack.
	tests := []struct {
		name  string
		query string
		check func(t *testing.T, issues []interface{})
	}{
		{"open issues", "status = open", nil},
		{"priority P0", "priority = P0", nil},
		{"type bug", "type = bug", nil},
		{"compound", "type = task and status = open", nil},
		{"order by", "status = open order by priority asc", nil},
		{"ready", "ready = true", nil},
		{"blocked", "blocked = true", nil},
		{"label", "label = bug", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := services.QueryExecutor.Execute(tt.query)
			require.NoError(t, err)
		})
	}
}

func TestBeadsRustBackend_QueryHelpers(t *testing.T) {
	backend := newTestBackend(t)
	services := backend.Services()
	helpers := services.QueryHelpers

	// IsStructuredQuery should recognize BQL.
	require.True(t, helpers.IsStructuredQuery("status = open"))
	require.True(t, helpers.IsStructuredQuery("type = bug and priority = P0"))
	require.False(t, helpers.IsStructuredQuery("just a search string"))

	// ValidateQuery should accept valid queries.
	require.NoError(t, helpers.ValidateQuery("status = open"))
	require.NoError(t, helpers.ValidateQuery("type = bug and priority = P0"))
	require.NoError(t, helpers.ValidateQuery("owner = bob"))

	// ValidateQuery should reject invalid fields.
	require.Error(t, helpers.ValidateQuery("hook_bead = test"))

	// BuildIDQuery should produce a valid query.
	idQuery := helpers.BuildIDQuery([]string{"TEST-1", "TEST-2"})
	require.Contains(t, idQuery, "TEST-1")
	require.Contains(t, idQuery, "TEST-2")
}
