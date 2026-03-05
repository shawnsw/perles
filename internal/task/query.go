package task

// DependencyEdge represents one edge in a dependency graph.
type DependencyEdge struct {
	TargetID string
	Type     string // "parent-child", "blocks", "discovered-from"
}

// DependencyGraph holds the full dependency graph for all issues.
// Forward edges: issue_id -> depends_on_id
// Reverse edges: depends_on_id -> issue_id
type DependencyGraph struct {
	Forward map[string][]DependencyEdge
	Reverse map[string][]DependencyEdge
}

// QueryHelpers provides optional utility functions that backends may implement.
// If a backend does not support structured queries, these methods should return
// sensible defaults or errors.
type QueryHelpers interface {
	// BuildIDQuery constructs a query to fetch issues by their IDs.
	BuildIDQuery(ids []string) string

	// IsStructuredQuery returns true if the input looks like a structured query
	// (e.g. BQL) vs a plain text search.
	IsStructuredQuery(input string) bool

	// ValidateQuery validates a query string, returning an error if invalid.
	ValidateQuery(query string) error
}

// SyntaxHighlighter provides query syntax highlighting for the UI.
// Backends that support structured query languages implement this to
// provide syntax highlighting in the search input.
type SyntaxHighlighter interface {
	// NewSyntaxLexer returns a syntax lexer for highlighting query input.
	// The returned value should implement the vimtextarea.SyntaxLexer interface.
	NewSyntaxLexer() any
}
