package infrastructure

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

func TestSQLiteClient_GetComments_UUIDIDs(t *testing.T) {
	beadsDir := t.TempDir()
	dbPath := filepath.Join(beadsDir, "beads.db")

	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
		CREATE TABLE comments (
			id TEXT PRIMARY KEY,
			issue_id TEXT NOT NULL,
			author TEXT NOT NULL,
			text TEXT NOT NULL,
			created_at DATETIME NOT NULL
		);

		INSERT INTO comments (id, issue_id, author, text, created_at) VALUES
			('019d8af6-1b65-7096-b664-47dccd9493d5', 'issue-1', 'alice', 'First', '2026-04-14T07:47:58Z'),
			('019d8af6-1b65-7096-b664-47dccd9493d6', 'issue-1', 'bob', 'Second', '2026-04-14T07:48:58Z');
	`)
	require.NoError(t, err)

	client, err := NewSQLiteClient(beadsDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	comments, err := client.GetComments("issue-1")
	require.NoError(t, err)
	require.Len(t, comments, 2)
	require.Equal(t, "019d8af6-1b65-7096-b664-47dccd9493d5", comments[0].ID)
	require.Equal(t, "alice", comments[0].Author)
	require.Equal(t, "First", comments[0].Text)
	require.Equal(t, time.Date(2026, time.April, 14, 7, 47, 58, 0, time.UTC), comments[0].CreatedAt)
	require.Equal(t, "019d8af6-1b65-7096-b664-47dccd9493d6", comments[1].ID)
}
