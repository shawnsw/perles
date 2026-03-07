package beadsrust

import (
	"github.com/zjrosen/perles/internal/beads/bql"
	"github.com/zjrosen/perles/internal/task"
)

// Compile-time checks.
var (
	_ task.QueryExecutor     = (*BRQueryExecutor)(nil)
	_ task.QueryHelpers      = (*BRQueryHelpers)(nil)
	_ task.SyntaxHighlighter = (*BRSyntaxHighlighter)(nil)
)

// BRQueryExecutor implements task.QueryExecutor by delegating to the BQL executor.
type BRQueryExecutor struct {
	inner *BQLExecutor
}

// NewBRQueryExecutor creates a new BRQueryExecutor wrapping the given BQL executor.
func NewBRQueryExecutor(inner *BQLExecutor) *BRQueryExecutor {
	return &BRQueryExecutor{inner: inner}
}

// Execute runs a BQL query and returns matching issues.
func (e *BRQueryExecutor) Execute(query string) ([]task.Issue, error) {
	return e.inner.Execute(query)
}

// BRQueryHelpers implements task.QueryHelpers using BQL functions
// with beads_rust-specific field validation.
type BRQueryHelpers struct{}

// NewBRQueryHelpers creates a new BRQueryHelpers.
func NewBRQueryHelpers() *BRQueryHelpers {
	return &BRQueryHelpers{}
}

// BuildIDQuery constructs a BQL id-in query for the given IDs.
func (h *BRQueryHelpers) BuildIDQuery(ids []string) string {
	return bql.BuildIDQuery(ids)
}

// IsStructuredQuery returns true if the input looks like a BQL query.
func (h *BRQueryHelpers) IsStructuredQuery(input string) bool {
	return bql.IsBQLQuery(input)
}

// ValidateQuery validates a BQL query against the beads_rust field set.
func (h *BRQueryHelpers) ValidateQuery(query string) error {
	parser := bql.NewParser(query)
	parsed, err := parser.Parse()
	if err != nil {
		return err
	}
	return bql.ValidateWithFields(parsed, brValidFields)
}

// BRSyntaxHighlighter implements task.SyntaxHighlighter using BQL syntax highlighting.
type BRSyntaxHighlighter struct{}

// NewBRSyntaxHighlighter creates a new BRSyntaxHighlighter.
func NewBRSyntaxHighlighter() *BRSyntaxHighlighter {
	return &BRSyntaxHighlighter{}
}

// NewSyntaxLexer returns a BQL syntax lexer for vimtextarea highlighting.
func (h *BRSyntaxHighlighter) NewSyntaxLexer() any {
	return bql.NewSyntaxLexer()
}
