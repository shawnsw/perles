package adapter

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	domain "github.com/zjrosen/perles/internal/beads/domain"
	infrastructure "github.com/zjrosen/perles/internal/beads/infrastructure"
)

type stubCommentReader struct {
	comments []domain.Comment
	err      error
	calls    int
}

func (s *stubCommentReader) GetComments(string) ([]domain.Comment, error) {
	s.calls++
	return s.comments, s.err
}

func TestBeadsTaskExecutor_GetComments_UsesPrimaryReader(t *testing.T) {
	primary := &stubCommentReader{
		comments: []domain.Comment{{
			Author:    "alice",
			Text:      "Primary comment",
			CreatedAt: time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC),
		}},
	}
	fallback := &stubCommentReader{
		comments: []domain.Comment{{
			Author:    "bob",
			Text:      "Fallback comment",
			CreatedAt: time.Date(2026, 4, 16, 12, 1, 0, 0, time.UTC),
		}},
	}

	exec := NewBeadsTaskExecutor(
		infrastructure.NewBDExecutor("", ""),
		WithCommentReader(primary),
		WithFallbackCommentReader(fallback),
	)

	comments, err := exec.GetComments("PROJ-1")
	require.NoError(t, err)
	require.Len(t, comments, 1)
	require.Equal(t, "Primary comment", comments[0].Text)
	require.Equal(t, 1, primary.calls)
	require.Zero(t, fallback.calls)
}

func TestBeadsTaskExecutor_GetComments_FallsBackWhenPrimaryFails(t *testing.T) {
	primary := &stubCommentReader{err: errors.New("no such table: comments")}
	fallback := &stubCommentReader{
		comments: []domain.Comment{{
			Author:    "alice",
			Text:      "Recovered via CLI",
			CreatedAt: time.Date(2026, 4, 16, 12, 2, 0, 0, time.UTC),
		}},
	}

	exec := NewBeadsTaskExecutor(
		infrastructure.NewBDExecutor("", ""),
		WithCommentReader(primary),
		WithFallbackCommentReader(fallback),
	)

	comments, err := exec.GetComments("PROJ-1")
	require.NoError(t, err)
	require.Len(t, comments, 1)
	require.Equal(t, "Recovered via CLI", comments[0].Text)
	require.Equal(t, 1, primary.calls)
	require.Equal(t, 1, fallback.calls)
}

func TestBeadsTaskExecutor_GetComments_UsesFallbackWhenPrimaryMissing(t *testing.T) {
	fallback := &stubCommentReader{
		comments: []domain.Comment{{
			Author:    "alice",
			Text:      "Fallback only",
			CreatedAt: time.Date(2026, 4, 16, 12, 3, 0, 0, time.UTC),
		}},
	}

	exec := NewBeadsTaskExecutor(
		infrastructure.NewBDExecutor("", ""),
		WithFallbackCommentReader(fallback),
	)

	comments, err := exec.GetComments("PROJ-1")
	require.NoError(t, err)
	require.Len(t, comments, 1)
	require.Equal(t, "Fallback only", comments[0].Text)
	require.Equal(t, 1, fallback.calls)
}
