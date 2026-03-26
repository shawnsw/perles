package beadsrust

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBRQueryHelpers_BuildIDQuery(t *testing.T) {
	h := NewBRQueryHelpers()

	// BQL format: id = "xxx" for single, id in ("a", "b") for multiple.
	require.Equal(t, `id = "abc"`, h.BuildIDQuery([]string{"abc"}))
	require.Equal(t, `id in ("abc", "def")`, h.BuildIDQuery([]string{"abc", "def"}))
	require.Equal(t, "", h.BuildIDQuery(nil))
	require.Equal(t, "", h.BuildIDQuery([]string{}))
}

func TestBRQueryHelpers_IsStructuredQuery(t *testing.T) {
	h := NewBRQueryHelpers()

	// BQL operators are now recognized.
	require.True(t, h.IsStructuredQuery("status = open"))
	require.True(t, h.IsStructuredQuery("type = bug and priority = P0"))
	require.True(t, h.IsStructuredQuery("label in (urgent, security)"))
	require.True(t, h.IsStructuredQuery("title ~ auth order by priority asc"))

	// Plain text search is not structured.
	require.False(t, h.IsStructuredQuery("search term"))
	require.False(t, h.IsStructuredQuery(""))
	require.False(t, h.IsStructuredQuery("auth bug"))
}

func TestBRQueryHelpers_ValidateQuery(t *testing.T) {
	h := NewBRQueryHelpers()

	// Valid BQL queries.
	require.NoError(t, h.ValidateQuery("status = open"))
	require.NoError(t, h.ValidateQuery("type = bug and priority = P0"))
	require.NoError(t, h.ValidateQuery("blocked = true"))
	require.NoError(t, h.ValidateQuery("owner ~ zack"))
	require.NoError(t, h.ValidateQuery("created > -7d"))
	require.NoError(t, h.ValidateQuery("label in (urgent, security)"))
	require.NoError(t, h.ValidateQuery("pinned = true"))
	require.NoError(t, h.ValidateQuery("sender ~ bot"))
	require.NoError(t, h.ValidateQuery("title ~ auth order by priority asc"))

	// Valid: ready is supported via inline subquery.
	require.NoError(t, h.ValidateQuery("ready = true"))

	// Invalid: removed agent-as-bead fields.
	require.Error(t, h.ValidateQuery("hook_bead = test"))
	require.Error(t, h.ValidateQuery("role_bead = test"))
	require.Error(t, h.ValidateQuery("agent_state = test"))
	require.Error(t, h.ValidateQuery("role_type = test"))

	// Invalid: unknown field.
	require.Error(t, h.ValidateQuery("nonexistent_field = foo"))
}

func TestBRSyntaxHighlighter(t *testing.T) {
	h := NewBRSyntaxHighlighter()
	lexer := h.NewSyntaxLexer()
	require.NotNil(t, lexer)
}
