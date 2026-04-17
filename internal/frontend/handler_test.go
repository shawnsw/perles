package frontend

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/zjrosen/perles/internal/orchestration/controlplane"
	"github.com/zjrosen/perles/internal/orchestration/controlplane/mocks"
	"github.com/zjrosen/perles/internal/orchestration/fabric"
	fabricpersist "github.com/zjrosen/perles/internal/orchestration/fabric/persistence"
	"github.com/zjrosen/perles/internal/orchestration/fabric/repository"
	"github.com/zjrosen/perles/internal/orchestration/session"
	v2 "github.com/zjrosen/perles/internal/orchestration/v2"
	v2repo "github.com/zjrosen/perles/internal/orchestration/v2/repository"
)

// createTestMux creates an http.ServeMux with the handler routes registered
// in the same order as production (API first, SPA last).
func createTestMux(h *Handler) *http.ServeMux {
	mux := http.NewServeMux()
	h.RegisterAPIRoutes(mux)
	h.RegisterSPAHandler(mux)
	return mux
}

// makeLoadSessionBody creates a properly JSON-encoded request body for LoadSession.
// This handles Windows paths correctly by escaping backslashes.
func makeLoadSessionBody(t *testing.T, path string) string {
	t.Helper()
	body, err := json.Marshal(LoadSessionRequest{Path: path})
	require.NoError(t, err)
	return string(body)
}

func makeJSONBody(t *testing.T, value any) string {
	t.Helper()
	body, err := json.Marshal(value)
	require.NoError(t, err)
	return string(body)
}

// === Health Endpoint Tests ===

func TestHandler_Health(t *testing.T) {
	h := NewHandler("/tmp/sessions", createTestFS(), nil)
	mux := createTestMux(h)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp HealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)
}

// === ListSessions Endpoint Tests ===

func TestHandler_ListSessions_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	h := NewHandler(tmpDir, createTestFS(), nil)
	mux := createTestMux(h)

	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp SessionListResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, tmpDir, resp.BasePath)
	assert.Empty(t, resp.Apps, "Expected empty apps array for empty directory")
}

func TestHandler_ListSessions_MissingDirectory(t *testing.T) {
	h := NewHandler("/nonexistent/path/sessions", createTestFS(), nil)
	mux := createTestMux(h)

	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp SessionListResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Empty(t, resp.Apps, "Expected empty apps array for missing directory")
}

func TestHandler_ListSessions_HierarchicalStructure(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test session structure
	// app1/2026-01-15/session-123/metadata.json
	// app1/2026-01-14/session-456/metadata.json
	// app2/2026-01-15/session-789/metadata.json
	createTestSession(t, tmpDir, "app1", "2026-01-15", "session-123", session.StatusRunning, "claude")
	createTestSession(t, tmpDir, "app1", "2026-01-14", "session-456", session.StatusCompleted, "amp")
	createTestSession(t, tmpDir, "app2", "2026-01-15", "session-789", session.StatusFailed, "claude")

	h := NewHandler(tmpDir, createTestFS(), nil)
	mux := createTestMux(h)

	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp SessionListResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Verify apps are present and sorted alphabetically
	require.Len(t, resp.Apps, 2)
	assert.Equal(t, "app1", resp.Apps[0].Name)
	assert.Equal(t, "app2", resp.Apps[1].Name)

	// Verify app1 has 2 dates sorted descending
	require.Len(t, resp.Apps[0].Dates, 2)
	assert.Equal(t, "2026-01-15", resp.Apps[0].Dates[0].Date)
	assert.Equal(t, "2026-01-14", resp.Apps[0].Dates[1].Date)

	// Verify sessions
	require.Len(t, resp.Apps[0].Dates[0].Sessions, 1)
	assert.Equal(t, "session-123", resp.Apps[0].Dates[0].Sessions[0].ID)
	assert.Equal(t, "running", resp.Apps[0].Dates[0].Sessions[0].Status)
	assert.Equal(t, "claude", resp.Apps[0].Dates[0].Sessions[0].ClientType)
}

// === LoadSession Endpoint Tests ===

func TestHandler_LoadSession_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a session with all file types
	sessionDir := createTestSession(t, tmpDir, "app1", "2026-01-15", "session-123", session.StatusRunning, "claude")

	// Add coordinator messages
	coordDir := filepath.Join(sessionDir, "coordinator")
	require.NoError(t, os.MkdirAll(coordDir, 0750))
	writeJSONL(t, filepath.Join(coordDir, "messages.jsonl"), []map[string]any{
		{"role": "assistant", "content": "Hello"},
		{"role": "user", "content": "Hi"},
	})
	writeJSONL(t, filepath.Join(coordDir, "raw.jsonl"), []map[string]any{
		{"type": "text", "text": "raw data"},
	})

	// Add worker data
	workerDir := filepath.Join(sessionDir, "workers", "worker-1")
	require.NoError(t, os.MkdirAll(workerDir, 0750))
	writeJSONL(t, filepath.Join(workerDir, "messages.jsonl"), []map[string]any{
		{"role": "assistant", "content": "Worker output"},
	})
	require.NoError(t, os.WriteFile(filepath.Join(workerDir, "accountability_summary.md"), []byte("# Summary\nDone."), 0600))

	// Add root-level JSONL files
	writeJSONL(t, filepath.Join(sessionDir, "messages.jsonl"), []map[string]any{
		{"id": "msg-1", "content": "Inter-agent message"},
	})
	writeJSONL(t, filepath.Join(sessionDir, "mcp_requests.jsonl"), []map[string]any{
		{"tool": "bash", "input": "ls"},
	})
	writeJSONL(t, filepath.Join(sessionDir, "commands.jsonl"), []map[string]any{
		{"command": "spawn_worker"},
	})

	h := NewHandler(tmpDir, createTestFS(), nil)
	mux := createTestMux(h)

	req := httptest.NewRequest(http.MethodPost, "/api/load-session", bytes.NewBufferString(makeLoadSessionBody(t, sessionDir)))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp LoadSessionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Verify path
	assert.Equal(t, sessionDir, resp.Path)

	// Verify metadata
	require.NotNil(t, resp.Metadata)
	assert.Equal(t, "session-123", resp.Metadata.SessionID)
	assert.Equal(t, "running", resp.Metadata.Status)
	assert.Equal(t, "claude", resp.Metadata.ClientType)

	// Verify coordinator data
	assert.Len(t, resp.Coordinator.Messages, 2)
	assert.Len(t, resp.Coordinator.Raw, 1)

	// Verify worker data
	require.Contains(t, resp.Workers, "worker-1")
	assert.Len(t, resp.Workers["worker-1"].Messages, 1)
	require.NotNil(t, resp.Workers["worker-1"].AccountabilitySummary)
	assert.Contains(t, *resp.Workers["worker-1"].AccountabilitySummary, "# Summary")

	// Verify root-level JSONL
	assert.Len(t, resp.Messages, 1)
	assert.Len(t, resp.MCPRequests, 1)
	assert.Len(t, resp.Commands, 1)
}

func TestHandler_LoadSession_MissingFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a minimal session with only metadata
	sessionDir := createTestSession(t, tmpDir, "app1", "2026-01-15", "session-123", session.StatusRunning, "claude")

	h := NewHandler(tmpDir, createTestFS(), nil)
	mux := createTestMux(h)

	req := httptest.NewRequest(http.MethodPost, "/api/load-session", bytes.NewBufferString(makeLoadSessionBody(t, sessionDir)))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp LoadSessionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Should return empty arrays, not errors
	assert.Empty(t, resp.Coordinator.Messages)
	assert.Empty(t, resp.Coordinator.Raw)
	assert.Empty(t, resp.Workers)
	assert.Empty(t, resp.Messages)
	assert.Empty(t, resp.MCPRequests)
	assert.Empty(t, resp.Commands)
}

func TestHandler_LoadSession_CorruptedJSONL(t *testing.T) {
	tmpDir := t.TempDir()

	sessionDir := createTestSession(t, tmpDir, "app1", "2026-01-15", "session-123", session.StatusRunning, "claude")

	// Create coordinator dir and write corrupted JSONL
	coordDir := filepath.Join(sessionDir, "coordinator")
	require.NoError(t, os.MkdirAll(coordDir, 0750))

	// Mix of valid and invalid JSON lines
	content := `{"role": "assistant", "content": "Valid"}
not valid json
{"role": "user", "content": "Also valid"}
{incomplete json
`
	require.NoError(t, os.WriteFile(filepath.Join(coordDir, "messages.jsonl"), []byte(content), 0600))

	h := NewHandler(tmpDir, createTestFS(), nil)
	mux := createTestMux(h)

	req := httptest.NewRequest(http.MethodPost, "/api/load-session", bytes.NewBufferString(makeLoadSessionBody(t, sessionDir)))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp LoadSessionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Should have 2 valid messages (skipped the corrupted lines)
	assert.Len(t, resp.Coordinator.Messages, 2)
}

func TestHandler_LoadSession_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	h := NewHandler(tmpDir, createTestFS(), nil)
	mux := createTestMux(h)

	tests := []struct {
		name string
		path string
	}{
		{"double dot", tmpDir + "/../etc/passwd"},
		{"embedded double dot", tmpDir + "/app/../../../etc/passwd"},
		{"triple dot attempt", tmpDir + "/..."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/load-session", bytes.NewBufferString(makeLoadSessionBody(t, tc.path)))
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			require.Equal(t, http.StatusBadRequest, w.Code)

			var resp APIError
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, "invalid_path", resp.Code)
		})
	}
}

func TestHandler_LoadSession_PathOutsideBase(t *testing.T) {
	tmpDir := t.TempDir()
	h := NewHandler(tmpDir, createTestFS(), nil)
	mux := createTestMux(h)

	// Try to access a path outside the session base directory
	body := `{"path": "/etc/passwd"}`
	req := httptest.NewRequest(http.MethodPost, "/api/load-session", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp APIError
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "invalid_path", resp.Code)
	assert.Contains(t, resp.Error, "not under session directory")
}

func TestHandler_LoadSession_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	h := NewHandler(tmpDir, createTestFS(), nil)
	mux := createTestMux(h)

	req := httptest.NewRequest(http.MethodPost, "/api/load-session", bytes.NewBufferString("not json"))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp APIError
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "invalid_json", resp.Code)
}

func TestHandler_LoadSession_SiblingDirectoryAttack(t *testing.T) {
	// Regression test for sibling directory path attack
	// If baseDir is /tmp/sessions, an attacker might try /tmp/sessionsx
	tmpDir := t.TempDir()
	h := NewHandler(tmpDir, createTestFS(), nil)
	mux := createTestMux(h)

	// Try to access a sibling directory with a matching prefix
	siblingPath := tmpDir + "x/evil"
	req := httptest.NewRequest(http.MethodPost, "/api/load-session", bytes.NewBufferString(makeLoadSessionBody(t, siblingPath)))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp APIError
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "invalid_path", resp.Code)
	assert.Contains(t, resp.Error, "not under session directory")
}

// === Helper Functions ===

func createTestFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html></html>")},
	}
}

func createTestSession(t *testing.T, baseDir, app, date, sessionID string, status session.Status, clientType string) string {
	t.Helper()

	sessionDir := filepath.Join(baseDir, app, date, sessionID)
	require.NoError(t, os.MkdirAll(sessionDir, 0750))

	// Create metadata
	meta := &session.Metadata{
		SessionID:       sessionID,
		StartTime:       time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
		Status:          status,
		SessionDir:      sessionDir,
		ClientType:      clientType,
		ApplicationName: app,
		DatePartition:   date,
		Workers:         []session.WorkerMetadata{},
	}
	require.NoError(t, meta.Save(sessionDir))

	return sessionDir
}

func writeJSONL(t *testing.T, path string, items []map[string]any) {
	t.Helper()

	var content []byte
	for _, item := range items {
		data, err := json.Marshal(item)
		require.NoError(t, err)
		content = append(content, data...)
		content = append(content, '\n')
	}

	require.NoError(t, os.WriteFile(path, content, 0600))
}

// === Unit Tests for Helper Functions ===

func TestIsValidDateFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"2026-01-15", true},
		{"2020-12-31", true},
		{"2026-1-15", false},   // Missing leading zero
		{"26-01-15", false},    // Wrong year format
		{"2026/01/15", false},  // Wrong separator
		{"2026-01-151", false}, // Too long
		{"2026-01-1", false},   // Too short
		{"abcd-ef-gh", false},  // Non-numeric
		{"", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, isValidDateFormat(tc.input))
		})
	}
}

func TestLoadRawJSONL_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	// Create empty file
	require.NoError(t, os.WriteFile(path, []byte(""), 0600))

	result := loadRawJSONL(path)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestLoadRawJSONL_NonExistent(t *testing.T) {
	result := loadRawJSONL("/nonexistent/path.jsonl")
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestLoadRawJSONL_ValidContent(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	content := `{"key": "value1"}
{"key": "value2"}
{"key": "value3"}
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))

	result := loadRawJSONL(path)
	require.Len(t, result, 3)

	// Verify content
	var item1 map[string]string
	require.NoError(t, json.Unmarshal(result[0], &item1))
	assert.Equal(t, "value1", item1["key"])
}

// === SPA Handler Integration Test ===

func TestHandler_SPAFallback(t *testing.T) {
	testFS := fstest.MapFS{
		"index.html":        &fstest.MapFile{Data: []byte("<html>SPA</html>")},
		"assets/app.js":     &fstest.MapFile{Data: []byte("console.log('app')")},
		"assets/styles.css": &fstest.MapFile{Data: []byte("body {}")},
	}

	h := NewHandler("/tmp/sessions", testFS, nil)
	mux := createTestMux(h)

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{"root path serves index.html", "/", http.StatusOK, "<html>SPA</html>"},
		{"existing file served", "/assets/app.js", http.StatusOK, "console.log('app')"},
		{"non-existent path serves index.html", "/some/route", http.StatusOK, "<html>SPA</html>"},
		{"api paths return 404", "/api/nonexistent", http.StatusNotFound, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)
			if tc.expectedBody != "" {
				assert.Contains(t, w.Body.String(), tc.expectedBody)
			}
		})
	}
}

// === Fabric Messaging Endpoint Tests ===

// newTestFabricService creates a test fabric service with initialized channels.
func newTestFabricService(t *testing.T) *fabric.Service {
	t.Helper()
	threadRepo := repository.NewMemoryThreadRepository()
	depRepo := repository.NewMemoryDependencyRepository()
	subRepo := repository.NewMemorySubscriptionRepository()
	ackRepo := repository.NewMemoryAckRepository(depRepo, threadRepo, subRepo)
	participantRepo := repository.NewMemoryParticipantRepository()

	svc := fabric.NewService(threadRepo, depRepo, subRepo, ackRepo, participantRepo)
	err := svc.InitSession("coordinator")
	require.NoError(t, err)
	return svc
}

// newTestWorkflowWithFabric creates a test workflow instance with fabric service and process repo.
func newTestWorkflowWithFabric(t *testing.T, state controlplane.WorkflowState) *controlplane.WorkflowInstance {
	t.Helper()
	fabricSvc := newTestFabricService(t)
	processRepo := v2repo.NewMemoryProcessRepository()

	// Add coordinator process
	_ = processRepo.Save(&v2repo.Process{
		ID:     "coordinator",
		Role:   v2repo.RoleCoordinator,
		Status: v2repo.StatusReady,
	})

	// Add a worker process
	_ = processRepo.Save(&v2repo.Process{
		ID:     "worker-1",
		Role:   v2repo.RoleWorker,
		Status: v2repo.StatusReady,
	})

	return &controlplane.WorkflowInstance{
		ID:    "test-workflow",
		Name:  "Test Workflow",
		State: state,
		Infrastructure: &v2.Infrastructure{
			Core: v2.CoreComponents{
				FabricService: fabricSvc,
			},
			Repositories: v2.RepositoryComponents{
				ProcessRepo: processRepo,
			},
		},
	}
}

func seedPersistedFabricSession(t *testing.T) (string, string) {
	t.Helper()

	sessionBase := t.TempDir()
	sessionDir := filepath.Join(sessionBase, "app", "2026-04-16", "session-123")
	require.NoError(t, os.MkdirAll(sessionDir, 0750))

	threadRepo := repository.NewMemoryThreadRepository()
	depRepo := repository.NewMemoryDependencyRepository()
	subRepo := repository.NewMemorySubscriptionRepository()
	ackRepo := repository.NewMemoryAckRepository(depRepo, threadRepo, subRepo)
	participantRepo := repository.NewMemoryParticipantRepository()

	svc := fabric.NewService(threadRepo, depRepo, subRepo, ackRepo, participantRepo)
	logger, err := fabricpersist.NewEventLogger(sessionDir)
	require.NoError(t, err)
	svc.SetEventHandler(logger.HandleEvent)

	require.NoError(t, svc.InitSession("coordinator"))

	msg, err := svc.SendMessage(fabric.SendMessageInput{
		ChannelSlug: "tasks",
		Content:     "Seed task thread",
		CreatedBy:   "coordinator",
	})
	require.NoError(t, err)
	require.NoError(t, logger.Close())

	return sessionBase, msg.ID
}

// === SendMessage Tests ===

func TestHandler_SendMessage_Success(t *testing.T) {
	workflow := newTestWorkflowWithFabric(t, controlplane.WorkflowRunning)

	mockCP := mocks.NewMockControlPlane(t)
	mockCP.EXPECT().Get(mock.Anything, controlplane.WorkflowID("test-workflow")).Return(workflow, nil)

	h := NewHandler("/tmp/sessions", createTestFS(), mockCP)
	mux := createTestMux(h)

	body := `{"workflowId":"test-workflow","channelSlug":"tasks","content":"Hello from user"}`
	req := httptest.NewRequest(http.MethodPost, "/api/fabric/send-message", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var resp SendMessageResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.MessageID)
}

func TestHandler_SendMessage_EmptyContent(t *testing.T) {
	mockCP := mocks.NewMockControlPlane(t)

	h := NewHandler("/tmp/sessions", createTestFS(), mockCP)
	mux := createTestMux(h)

	body := `{"workflowId":"test-workflow","channelSlug":"tasks","content":"   "}`
	req := httptest.NewRequest(http.MethodPost, "/api/fabric/send-message", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp APIError
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "validation_error", resp.Code)
	assert.Contains(t, resp.Error, "empty")
}

func TestHandler_SendMessage_ContentTooLong(t *testing.T) {
	mockCP := mocks.NewMockControlPlane(t)

	h := NewHandler("/tmp/sessions", createTestFS(), mockCP)
	mux := createTestMux(h)

	// Create content that exceeds 10,000 characters
	longContent := strings.Repeat("a", 10001)
	body := `{"workflowId":"test-workflow","channelSlug":"tasks","content":"` + longContent + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/fabric/send-message", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp APIError
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "validation_error", resp.Code)
	assert.Contains(t, resp.Error, "maximum length")
}

func TestHandler_SendMessage_WorkflowNotFound(t *testing.T) {
	mockCP := mocks.NewMockControlPlane(t)
	mockCP.EXPECT().Get(mock.Anything, controlplane.WorkflowID("nonexistent")).Return(nil, controlplane.ErrWorkflowNotFound)

	h := NewHandler("/tmp/sessions", createTestFS(), mockCP)
	mux := createTestMux(h)

	body := `{"workflowId":"nonexistent","channelSlug":"tasks","content":"Hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/fabric/send-message", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)

	var resp APIError
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "not_found", resp.Code)
}

func TestHandler_SendMessage_ArchivedSessionFallback(t *testing.T) {
	sessionBase, _ := seedPersistedFabricSession(t)
	sessionDir := filepath.Join(sessionBase, "app", "2026-04-16", "session-123")

	h := NewHandler(sessionBase, createTestFS(), nil)
	mux := createTestMux(h)

	body := makeJSONBody(t, SendMessageRequest{
		SessionPath: sessionDir,
		ChannelSlug: "tasks",
		Content:     "Archived comment",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/fabric/send-message", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	events, err := fabricpersist.LoadPersistedEvents(sessionDir)
	require.NoError(t, err)
	require.NotEmpty(t, events)
	lastEvent := events[len(events)-1]
	assert.Equal(t, fabric.EventMessagePosted, lastEvent.Event.Type)
	if assert.NotNil(t, lastEvent.Event.Thread) {
		assert.Equal(t, "Archived comment", lastEvent.Event.Thread.Content)
		assert.Equal(t, "user", lastEvent.Event.Thread.CreatedBy)
	}
}

// === Reply Tests ===

func TestHandler_Reply_Success(t *testing.T) {
	workflow := newTestWorkflowWithFabric(t, controlplane.WorkflowRunning)

	// Send a message first to get a thread ID
	msg, err := workflow.Infrastructure.Core.FabricService.SendMessage(fabric.SendMessageInput{
		ChannelSlug: "tasks",
		Content:     "Initial message",
		CreatedBy:   "coordinator",
	})
	require.NoError(t, err)

	mockCP := mocks.NewMockControlPlane(t)
	mockCP.EXPECT().Get(mock.Anything, controlplane.WorkflowID("test-workflow")).Return(workflow, nil)

	h := NewHandler("/tmp/sessions", createTestFS(), mockCP)
	mux := createTestMux(h)

	body := `{"workflowId":"test-workflow","threadId":"` + msg.ID + `","content":"Reply from user"}`
	req := httptest.NewRequest(http.MethodPost, "/api/fabric/reply", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var resp SendMessageResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.MessageID)
}

func TestHandler_Reply_ThreadNotFound(t *testing.T) {
	workflow := newTestWorkflowWithFabric(t, controlplane.WorkflowRunning)

	mockCP := mocks.NewMockControlPlane(t)
	mockCP.EXPECT().Get(mock.Anything, controlplane.WorkflowID("test-workflow")).Return(workflow, nil)

	h := NewHandler("/tmp/sessions", createTestFS(), mockCP)
	mux := createTestMux(h)

	body := `{"workflowId":"test-workflow","threadId":"nonexistent-thread","content":"Reply"}`
	req := httptest.NewRequest(http.MethodPost, "/api/fabric/reply", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)

	var resp APIError
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "not_found", resp.Code)
	assert.Contains(t, resp.Error, "Thread not found")
}

func TestHandler_Reply_ArchivedSessionFallback(t *testing.T) {
	sessionBase, threadID := seedPersistedFabricSession(t)
	sessionDir := filepath.Join(sessionBase, "app", "2026-04-16", "session-123")

	h := NewHandler(sessionBase, createTestFS(), nil)
	mux := createTestMux(h)

	body := makeJSONBody(t, ReplyRequest{
		SessionPath: sessionDir,
		ThreadID:    threadID,
		Content:     "Archived reply",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/fabric/reply", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	events, err := fabricpersist.LoadPersistedEvents(sessionDir)
	require.NoError(t, err)
	require.NotEmpty(t, events)
	lastEvent := events[len(events)-1]
	assert.Equal(t, fabric.EventReplyPosted, lastEvent.Event.Type)
	assert.Equal(t, threadID, lastEvent.Event.ParentID)
	if assert.NotNil(t, lastEvent.Event.Thread) {
		assert.Equal(t, "Archived reply", lastEvent.Event.Thread.Content)
		assert.Equal(t, "user", lastEvent.Event.Thread.CreatedBy)
	}
}

// === ListAgents Tests ===

func TestHandler_ListAgents_Success(t *testing.T) {
	workflow := newTestWorkflowWithFabric(t, controlplane.WorkflowRunning)

	mockCP := mocks.NewMockControlPlane(t)
	mockCP.EXPECT().Get(mock.Anything, controlplane.WorkflowID("test-workflow")).Return(workflow, nil)

	h := NewHandler("/tmp/sessions", createTestFS(), mockCP)
	mux := createTestMux(h)

	req := httptest.NewRequest(http.MethodGet, "/api/fabric/agents?workflowId=test-workflow", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp AgentsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.True(t, resp.IsActive)
	assert.Len(t, resp.Agents, 2) // coordinator + worker-1

	// Find coordinator and worker
	var foundCoord, foundWorker bool
	for _, agent := range resp.Agents {
		if agent.ID == "coordinator" && agent.Role == "coordinator" {
			foundCoord = true
		}
		if agent.ID == "worker-1" && agent.Role == "worker" {
			foundWorker = true
		}
	}
	assert.True(t, foundCoord, "should have coordinator")
	assert.True(t, foundWorker, "should have worker-1")
}

func TestHandler_ListAgents_InactiveWorkflow(t *testing.T) {
	workflow := newTestWorkflowWithFabric(t, controlplane.WorkflowCompleted)

	mockCP := mocks.NewMockControlPlane(t)
	mockCP.EXPECT().Get(mock.Anything, controlplane.WorkflowID("test-workflow")).Return(workflow, nil)

	h := NewHandler("/tmp/sessions", createTestFS(), mockCP)
	mux := createTestMux(h)

	req := httptest.NewRequest(http.MethodGet, "/api/fabric/agents?workflowId=test-workflow", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp AgentsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.False(t, resp.IsActive)
	assert.Len(t, resp.Agents, 2) // Agents are still returned, just marked inactive
}

func TestHandler_ListAgents_MissingWorkflowId(t *testing.T) {
	mockCP := mocks.NewMockControlPlane(t)

	h := NewHandler("/tmp/sessions", createTestFS(), mockCP)
	mux := createTestMux(h)

	req := httptest.NewRequest(http.MethodGet, "/api/fabric/agents", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp APIError
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "validation_error", resp.Code)
}

func TestHandler_ListAgents_WorkflowNotFound(t *testing.T) {
	mockCP := mocks.NewMockControlPlane(t)
	mockCP.EXPECT().Get(mock.Anything, controlplane.WorkflowID("nonexistent")).Return(nil, controlplane.ErrWorkflowNotFound)

	h := NewHandler("/tmp/sessions", createTestFS(), mockCP)
	mux := createTestMux(h)

	req := httptest.NewRequest(http.MethodGet, "/api/fabric/agents?workflowId=nonexistent", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)

	var resp APIError
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "not_found", resp.Code)
}

func TestHandler_ListAgents_NoControlPlane(t *testing.T) {
	// Handler with nil ControlPlane
	h := NewHandler("/tmp/sessions", createTestFS(), nil)
	mux := createTestMux(h)

	req := httptest.NewRequest(http.MethodGet, "/api/fabric/agents?workflowId=test-workflow", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp AgentsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.False(t, resp.IsActive)
	assert.Empty(t, resp.Agents)
}

func TestHandler_ListAgents_FallsBackToSessionPath(t *testing.T) {
	sessionBase, _ := seedPersistedFabricSession(t)
	sessionDir := filepath.Join(sessionBase, "app", "2026-04-16", "session-123")

	h := NewHandler(sessionBase, createTestFS(), nil)
	mux := createTestMux(h)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/fabric/agents?sessionPath="+sessionDir,
		nil,
	)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp AgentsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.False(t, resp.IsActive)
	assert.NotEmpty(t, resp.Agents)
	assert.Equal(t, "coordinator", resp.Agents[0].ID)
}

// === ReadFile Endpoint Tests ===

func TestHandler_ReadFile_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file within the sessions directory
	testFile := filepath.Join(tmpDir, "app1", "2026-01-15", "session-123", "artifact.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(testFile), 0750))
	require.NoError(t, os.WriteFile(testFile, []byte("# Hello World\n\nThis is a test."), 0600))

	h := NewHandler(tmpDir, createTestFS(), nil)
	mux := createTestMux(h)

	req := httptest.NewRequest(http.MethodGet, "/api/file?path="+testFile, nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "# Hello World\n\nThis is a test.", w.Body.String())
}

func TestHandler_ReadFile_MissingPath(t *testing.T) {
	h := NewHandler("/tmp/sessions", createTestFS(), nil)
	mux := createTestMux(h)

	req := httptest.NewRequest(http.MethodGet, "/api/file", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "path parameter required")
}

func TestHandler_ReadFile_OutsideSessionDir_Allowed(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file outside the sessions directory - should still be readable
	// since fabric_attach can reference files anywhere on the filesystem
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "project-file.txt")
	require.NoError(t, os.WriteFile(outsideFile, []byte("project data"), 0600))

	h := NewHandler(tmpDir, createTestFS(), nil)
	mux := createTestMux(h)

	req := httptest.NewRequest(http.MethodGet, "/api/file?path="+outsideFile, nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "project data", w.Body.String())
}

func TestHandler_ReadFile_RelativePath_Denied(t *testing.T) {
	h := NewHandler(t.TempDir(), createTestFS(), nil)
	mux := createTestMux(h)

	req := httptest.NewRequest(http.MethodGet, "/api/file?path=relative/path.txt", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "access denied")
}

func TestHandler_ReadFile_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	h := NewHandler(tmpDir, createTestFS(), nil)
	mux := createTestMux(h)

	nonexistentFile := filepath.Join(tmpDir, "does-not-exist.txt")
	req := httptest.NewRequest(http.MethodGet, "/api/file?path="+nonexistentFile, nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "file not found")
}

func TestHandler_ReadFile_PathTraversalCleaned(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file and access it via a path with .. segments
	// filepath.Clean resolves these to the canonical path
	testFile := filepath.Join(tmpDir, "file.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("content"), 0600))

	h := NewHandler(tmpDir, createTestFS(), nil)
	mux := createTestMux(h)

	// Access via path with .. that resolves to the same file
	traversalPath := filepath.Join(tmpDir, "subdir", "..", "file.txt")
	req := httptest.NewRequest(http.MethodGet, "/api/file?path="+traversalPath, nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	// filepath.Clean resolves the .. so the file is found
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "content", w.Body.String())
}

// Suppress unused import warning for context
var _ = context.Background
