package beadsrust

import (
	"database/sql"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSQLiteReader_NewSQLiteReader_MissingDB(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "beadsrust-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	_, err = NewSQLiteReader(tmpDir)
	require.Error(t, err)
}

func TestSQLiteReader_ShowIssue(t *testing.T) {
	reader := newTestReader(t)

	issue, err := reader.ShowIssue("TEST-1")
	require.NoError(t, err)
	require.Equal(t, "TEST-1", issue.ID)
	require.Equal(t, "Open bug", issue.TitleText)
	require.Equal(t, "open", string(issue.Status))
	require.Equal(t, 0, int(issue.Priority))
	require.Equal(t, "bug", string(issue.Type))
	require.Equal(t, "alice", issue.Assignee)
	require.Equal(t, "A critical bug", issue.DescriptionText)
	require.False(t, issue.CreatedAt.IsZero())
	require.False(t, issue.UpdatedAt.IsZero())

	// Labels should be loaded.
	require.Contains(t, issue.Labels, "bug")
	require.Contains(t, issue.Labels, "urgent")
}

func TestSQLiteReader_ShowIssue_NotFound(t *testing.T) {
	reader := newTestReader(t)
	_, err := reader.ShowIssue("nonexistent-id-xyz")
	require.Error(t, err)
	require.Contains(t, err.Error(), "issue not found")
}

func TestSQLiteReader_ShowIssue_WithDependencies(t *testing.T) {
	reader := newTestReader(t)

	// TEST-3 is blocked by TEST-1.
	issue, err := reader.ShowIssue("TEST-3")
	require.NoError(t, err)
	require.Equal(t, "TEST-3", issue.ID)
	require.Contains(t, issue.BlockedBy, "TEST-1")
}

func TestSQLiteReader_ShowIssue_ClosedIssue(t *testing.T) {
	reader := newTestReader(t)

	issue, err := reader.ShowIssue("TEST-4")
	require.NoError(t, err)
	require.Equal(t, "closed", string(issue.Status))
	require.False(t, issue.ClosedAt.IsZero(), "closed issue should have ClosedAt set")
}

func TestSQLiteReader_GetComments(t *testing.T) {
	reader := newTestReader(t)

	// TEST-1 has 2 comments.
	comments, err := reader.GetComments("TEST-1")
	require.NoError(t, err)
	require.Len(t, comments, 2)
	require.Equal(t, "comment-1", comments[0].ID)
	require.Equal(t, "alice", comments[0].Author)
	require.Equal(t, "Investigating this bug", comments[0].Text)
	require.Equal(t, "comment-2", comments[1].ID)
	require.Equal(t, "bob", comments[1].Author)
	require.Equal(t, "Found the root cause", comments[1].Text)

	// TEST-2 has 1 comment.
	comments, err = reader.GetComments("TEST-2")
	require.NoError(t, err)
	require.Len(t, comments, 1)
	require.Equal(t, "comment-3", comments[0].ID)
	require.Equal(t, "carol", comments[0].Author)
}

func TestSQLiteReader_GetComments_EmptyForNonexistent(t *testing.T) {
	reader := newTestReader(t)
	comments, err := reader.GetComments("nonexistent-id-xyz")
	require.NoError(t, err)
	require.Empty(t, comments)
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		input string
		want  bool // whether time should be non-zero
	}{
		{"2026-03-06T12:00:00Z", true},
		{"2026-03-06T12:00:00.123456Z", true},
		{"2026-03-06 12:00:00", true},
		{"2026-03-06T12:00:00+00:00", true},
		{"2026-03-06T12:00:00.123456-05:00", true},
		{"", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseTime(tt.input)
			if tt.want {
				require.False(t, result.IsZero(), "expected non-zero time for %q", tt.input)
				require.Equal(t, 2026, result.Year())
			} else {
				require.True(t, result.IsZero(), "expected zero time for %q", tt.input)
			}
		})
	}
}

func TestNullStr(t *testing.T) {
	require.Equal(t, "", nullStr(sql.NullString{String: "", Valid: false}))
	require.Equal(t, "", nullStr(sql.NullString{String: "", Valid: true}))
	require.Equal(t, "hello", nullStr(sql.NullString{String: "hello", Valid: true}))
}

func TestInClause(t *testing.T) {
	placeholders, args := inClause([]string{"a", "b", "c"})
	require.Equal(t, "?,?,?", placeholders)
	require.Equal(t, []any{"a", "b", "c"}, args)
}
