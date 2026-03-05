package task

import (
	"context"
	"time"
)

// TaskReader provides read access to individual issues and their comments.
type TaskReader interface {
	ShowIssue(issueID string) (*Issue, error)
	GetComments(issueID string) ([]Comment, error)
}

// TaskWriter provides write access to issues.
type TaskWriter interface {
	UpdateStatus(issueID string, status Status) error
	UpdatePriority(issueID string, priority Priority) error
	UpdateType(issueID string, issueType IssueType) error
	UpdateTitle(issueID, title string) error
	UpdateDescription(issueID, description string) error
	UpdateNotes(issueID, notes string) error
	CloseIssue(issueID, reason string) error
	ReopenIssue(issueID string) error
	SetLabels(issueID string, labels []string) error
	AddComment(issueID, author, text string) error
	CreateEpic(title, description string, labels []string) (CreateResult, error)
	CreateTask(title, description, parentID, assignee string, labels []string) (CreateResult, error)
	DeleteIssues(issueIDs []string) error
	AddDependency(taskID, dependsOnID string) error
	UpdateIssue(issueID string, opts UpdateOptions) error
}

// TaskExecutor combines read and write operations on issues.
type TaskExecutor interface {
	TaskReader
	TaskWriter
}

// QueryExecutor executes queries and returns matching issues.
// Each backend defines its own query language/strategy.
// The query string format is backend-dependent (BQL for beads, etc.).
type QueryExecutor interface {
	Execute(query string) ([]Issue, error)
}

// BackendCapabilities describes what a backend supports.
// This lets the UI gracefully degrade for backends that lack certain features.
type BackendCapabilities struct {
	SupportsQuery        bool   // Can execute structured queries
	QueryLanguageName    string // "BQL", "JQL", etc. — shown in UI
	SupportsDependencies bool   // BlockedBy/Blocks tracking
	SupportsTree         bool   // Parent/child hierarchy
	SupportsComments     bool
	SupportsLabels       bool
	SupportsPriority     bool
	SupportsDesignField  bool
	SupportsNotesField   bool
}

// WatcherConfig holds backend-specific file watcher configuration.
type WatcherConfig struct {
	// RelevantFiles lists the base filenames that should trigger a refresh.
	// For example: ["beads.db", "beads.db-wal"] for SQLite backends.
	RelevantFiles []string

	// DebounceDuration is how long to wait after the last filesystem event
	// before publishing a change notification. Zero means use default (100ms).
	DebounceDuration time.Duration
}

// Backend is the top-level interface for a task-tracking backend.
// Each backend (beads, github, linear, etc.) implements this interface.
// The composition root creates a Backend via a factory function and passes
// it to the application layer, which uses Services() for task operations
// and FlushCaches() when the underlying data store changes on disk.
type Backend interface {
	// Services returns the task-layer services produced by this backend.
	Services() BackendServices

	// CheckCompatibility verifies that the backend's data store is compatible
	// with this version of perles. Returns *VersionIncompatibleError if the
	// data store version is too old, nil if compatible.
	CheckCompatibility() error

	// FlushCaches invalidates all internal caches so subsequent queries
	// hit the data store instead of returning stale results.
	FlushCaches(ctx context.Context) error

	// Close releases all backend resources (database connections, etc.).
	Close() error
}

// BackendServices holds all task-layer services produced by a backend.
// Each backend implementation (beads, etc.) constructs these internally
// and exposes them via a Services() method.
type BackendServices struct {
	TaskExecutor      TaskExecutor
	QueryExecutor     QueryExecutor
	QueryHelpers      QueryHelpers        // nil if backend has no structured query language
	SyntaxHighlighter SyntaxHighlighter   // nil if backend has no query syntax highlighting
	Capabilities      BackendCapabilities // what the backend supports
	WatcherConfig     WatcherConfig       // file watcher configuration
	DBPath            string              // path to data store file (for watcher)
}
