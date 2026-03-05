package adapter

import (
	"github.com/zjrosen/perles/internal/beads/bql"
	"github.com/zjrosen/perles/internal/task"
)

// Compile-time checks.
var (
	_ task.QueryExecutor = (*BeadsQueryExecutor)(nil)
	_ task.QueryHelpers  = (*BeadsQueryHelpers)(nil)
)

// BeadsQueryExecutor wraps a BQL executor to implement task.QueryExecutor.
type BeadsQueryExecutor struct {
	inner bql.BQLExecutor
}

// NewBeadsQueryExecutor creates a new BeadsQueryExecutor wrapping the given BQLExecutor.
func NewBeadsQueryExecutor(inner bql.BQLExecutor) *BeadsQueryExecutor {
	return &BeadsQueryExecutor{inner: inner}
}

func (e *BeadsQueryExecutor) Execute(query string) ([]task.Issue, error) {
	issues, err := e.inner.Execute(query)
	if err != nil {
		return nil, err
	}
	return ToTaskIssues(issues), nil
}

// BeadsQueryHelpers implements task.QueryHelpers using BQL functions.
type BeadsQueryHelpers struct{}

// NewBeadsQueryHelpers creates a new BeadsQueryHelpers.
func NewBeadsQueryHelpers() *BeadsQueryHelpers {
	return &BeadsQueryHelpers{}
}

func (h *BeadsQueryHelpers) BuildIDQuery(ids []string) string {
	return bql.BuildIDQuery(ids)
}

func (h *BeadsQueryHelpers) IsStructuredQuery(input string) bool {
	return bql.IsBQLQuery(input)
}

func (h *BeadsQueryHelpers) ValidateQuery(query string) error {
	parser := bql.NewParser(query)
	parsed, err := parser.Parse()
	if err != nil {
		return err
	}
	return bql.Validate(parsed)
}

// BeadsSyntaxHighlighter implements task.SyntaxHighlighter using BQL syntax highlighting.
type BeadsSyntaxHighlighter struct{}

// NewBeadsSyntaxHighlighter creates a new BeadsSyntaxHighlighter.
func NewBeadsSyntaxHighlighter() *BeadsSyntaxHighlighter {
	return &BeadsSyntaxHighlighter{}
}

func (h *BeadsSyntaxHighlighter) NewSyntaxLexer() any {
	return bql.NewSyntaxLexer()
}
