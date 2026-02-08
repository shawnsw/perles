// Package frontend provides HTTP handlers for serving the embedded SPA frontend.
package frontend

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zjrosen/perles/internal/log"
	"github.com/zjrosen/perles/internal/orchestration/controlplane"
	"github.com/zjrosen/perles/internal/orchestration/fabric"
	"github.com/zjrosen/perles/internal/orchestration/fabric/domain"
	"github.com/zjrosen/perles/internal/orchestration/session"
)

// maxLineSize is the buffer size for reading JSONL lines (1MB).
const maxLineSize = 1024 * 1024

// maxContentLength is the maximum allowed content length for fabric messages (10,000 chars).
const maxContentLength = 10000

// Handler provides HTTP endpoints for the session viewer frontend.
type Handler struct {
	sessionBaseDir string
	spaHandler     http.Handler
	controlPlane   controlplane.ControlPlane
}

// NewHandler creates a new Handler with the given session base directory, SPA filesystem,
// and optional ControlPlane for workflow operations.
// If sessionBaseDir is empty, session.DefaultBaseDir() is used.
// The spaFS should be pre-processed with fs.Sub() to strip any prefix (e.g., "dist/").
// The controlPlane parameter can be nil if workflow lookup is not needed.
func NewHandler(sessionBaseDir string, spaFS fs.FS, cp controlplane.ControlPlane) *Handler {
	if sessionBaseDir == "" {
		sessionBaseDir = session.DefaultBaseDir()
	}
	return &Handler{
		sessionBaseDir: sessionBaseDir,
		spaHandler:     NewSPAHandler(spaFS),
		controlPlane:   cp,
	}
}

// RegisterAPIRoutes registers the API routes on the provided mux.
// Call this BEFORE registering the SPA handler.
func (h *Handler) RegisterAPIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", h.Health)
	mux.HandleFunc("GET /api/sessions", h.ListSessions)
	mux.HandleFunc("POST /api/load-session", h.LoadSession)

	// Fabric messaging endpoints
	mux.HandleFunc("POST /api/fabric/send-message", h.SendMessage)
	mux.HandleFunc("POST /api/fabric/reply", h.Reply)
	mux.HandleFunc("POST /api/fabric/react", h.React)
	mux.HandleFunc("GET /api/fabric/agents", h.ListAgents)

	// File content endpoint for artifacts
	mux.HandleFunc("GET /api/file", h.ReadFile)
}

// RegisterSPAHandler registers the SPA catch-all handler on the provided mux.
// IMPORTANT: This MUST be registered LAST after all other routes.
// The SPA handler catches all unmatched paths and serves index.html for client-side routing.
func (h *Handler) RegisterSPAHandler(mux *http.ServeMux) {
	mux.Handle("/", h.spaHandler)
}

// Health returns a simple health check response.
// GET /api/health
func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	h.writeJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}

// ListSessions returns all sessions organized hierarchically by application and date.
// GET /api/sessions
func (h *Handler) ListSessions(w http.ResponseWriter, _ *http.Request) {
	resp := SessionListResponse{
		BasePath: h.sessionBaseDir,
		Apps:     []AppSessions{},
	}

	// Check if base directory exists
	if _, err := os.Stat(h.sessionBaseDir); os.IsNotExist(err) {
		// Return empty list for missing directory
		h.writeJSON(w, http.StatusOK, resp)
		return
	}

	// List application directories
	appEntries, err := os.ReadDir(h.sessionBaseDir)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "read_error", "Failed to read sessions directory", err.Error())
		return
	}

	for _, appEntry := range appEntries {
		if !appEntry.IsDir() {
			continue
		}

		// Skip if not a valid application directory (must contain sessions.json or date directories)
		appDir := filepath.Join(h.sessionBaseDir, appEntry.Name())
		appSessions, err := h.loadAppSessions(appEntry.Name(), appDir)
		if err != nil {
			log.Warn(log.CatOrch, "Failed to load app sessions", "app", appEntry.Name(), "error", err)
			continue // Skip apps with errors
		}

		if len(appSessions.Dates) > 0 {
			resp.Apps = append(resp.Apps, appSessions)
		}
	}

	// Sort apps alphabetically
	sort.Slice(resp.Apps, func(i, j int) bool {
		return resp.Apps[i].Name < resp.Apps[j].Name
	})

	h.writeJSON(w, http.StatusOK, resp)
}

// loadAppSessions loads all sessions for an application, organized by date.
func (h *Handler) loadAppSessions(appName, appDir string) (AppSessions, error) {
	result := AppSessions{
		Name:  appName,
		Dates: []DateGroup{},
	}

	dateEntries, err := os.ReadDir(appDir)
	if err != nil {
		return result, fmt.Errorf("reading app directory: %w", err)
	}

	for _, dateEntry := range dateEntries {
		if !dateEntry.IsDir() {
			continue
		}

		// Validate date format (YYYY-MM-DD)
		dateName := dateEntry.Name()
		if !isValidDateFormat(dateName) {
			continue
		}

		dateDir := filepath.Join(appDir, dateName)
		dateGroup, err := h.loadDateGroup(dateName, dateDir)
		if err != nil {
			log.Warn(log.CatOrch, "Failed to load date group", "app", appName, "date", dateName, "error", err)
			continue // Skip dates with errors
		}

		if len(dateGroup.Sessions) > 0 {
			result.Dates = append(result.Dates, dateGroup)
		}
	}

	// Sort dates descending (most recent first)
	sort.Slice(result.Dates, func(i, j int) bool {
		return result.Dates[i].Date > result.Dates[j].Date
	})

	return result, nil
}

// loadDateGroup loads all sessions for a specific date.
func (h *Handler) loadDateGroup(dateName, dateDir string) (DateGroup, error) {
	result := DateGroup{
		Date:     dateName,
		Sessions: []SessionSummary{},
	}

	sessionEntries, err := os.ReadDir(dateDir)
	if err != nil {
		return result, fmt.Errorf("reading date directory: %w", err)
	}

	for _, sessionEntry := range sessionEntries {
		if !sessionEntry.IsDir() {
			continue
		}

		sessionDir := filepath.Join(dateDir, sessionEntry.Name())
		summary, err := h.loadSessionSummary(sessionEntry.Name(), sessionDir)
		if err != nil {
			log.Debug(log.CatOrch, "Failed to load session summary", "session", sessionEntry.Name(), "error", err)
			continue // Skip sessions with errors
		}

		result.Sessions = append(result.Sessions, summary)
	}

	// Sort sessions by start time descending (most recent first)
	sort.Slice(result.Sessions, func(i, j int) bool {
		// Sessions with StartTime come first, sorted descending
		if result.Sessions[i].StartTime == nil && result.Sessions[j].StartTime == nil {
			return result.Sessions[i].ID > result.Sessions[j].ID
		}
		if result.Sessions[i].StartTime == nil {
			return false
		}
		if result.Sessions[j].StartTime == nil {
			return true
		}
		return *result.Sessions[i].StartTime > *result.Sessions[j].StartTime
	})

	return result, nil
}

// loadSessionSummary loads a lightweight summary from a session's metadata.json.
func (h *Handler) loadSessionSummary(sessionID, sessionDir string) (SessionSummary, error) {
	summary := SessionSummary{
		ID:   sessionID,
		Path: sessionDir,
	}

	// Load metadata
	meta, err := session.Load(sessionDir)
	if err != nil {
		return summary, fmt.Errorf("loading metadata: %w", err)
	}

	// Populate summary from metadata
	if !meta.StartTime.IsZero() {
		startTime := meta.StartTime.Format("2006-01-02T15:04:05Z07:00")
		summary.StartTime = &startTime
	}
	summary.Status = string(meta.Status)
	summary.WorkerCount = len(meta.Workers)
	summary.ClientType = meta.ClientType

	return summary, nil
}

// LoadSession loads all session data for the viewer.
// POST /api/load-session
func (h *Handler) LoadSession(w http.ResponseWriter, r *http.Request) {
	var req LoadSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body", err.Error())
		return
	}

	// Validate path
	if err := h.validateSessionPath(req.Path); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_path", err.Error(), "")
		return
	}

	// Build response
	resp := LoadSessionResponse{
		Path:        req.Path,
		Fabric:      []json.RawMessage{},
		MCPRequests: []json.RawMessage{},
		Commands:    []json.RawMessage{},
		Messages:    []json.RawMessage{},
		Coordinator: CoordinatorData{
			Messages: []json.RawMessage{},
			Raw:      []json.RawMessage{},
		},
		Workers: make(map[string]WorkerData),
	}

	// Load metadata
	meta, err := session.Load(req.Path)
	if err != nil {
		// If metadata is missing, return a minimal response
		log.Warn(log.CatOrch, "Failed to load session metadata", "path", req.Path, "error", err)
	} else {
		resp.Metadata = h.convertMetadata(meta)
	}

	// Load root-level JSONL files
	resp.Messages = loadRawJSONL(filepath.Join(req.Path, "messages.jsonl"))
	resp.MCPRequests = loadRawJSONL(filepath.Join(req.Path, "mcp_requests.jsonl"))
	resp.Commands = loadRawJSONL(filepath.Join(req.Path, "commands.jsonl"))
	resp.Fabric = loadRawJSONL(filepath.Join(req.Path, "fabric.jsonl"))

	// Load coordinator data
	coordDir := filepath.Join(req.Path, "coordinator")
	resp.Coordinator.Messages = loadRawJSONL(filepath.Join(coordDir, "messages.jsonl"))
	resp.Coordinator.Raw = loadRawJSONL(filepath.Join(coordDir, "raw.jsonl"))

	// Load worker data
	workersDir := filepath.Join(req.Path, "workers")
	if entries, err := os.ReadDir(workersDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			workerID := entry.Name()
			workerDir := filepath.Join(workersDir, workerID)

			workerData := WorkerData{
				Messages: loadRawJSONL(filepath.Join(workerDir, "messages.jsonl")),
				Raw:      loadRawJSONL(filepath.Join(workerDir, "raw.jsonl")),
			}

			// Load accountability summary if it exists
			summaryPath := filepath.Join(workerDir, "accountability_summary.md")
			if data, err := os.ReadFile(summaryPath); err == nil { //nolint:gosec // G304: path is validated by validateSessionPath before calling this handler
				summaryStr := string(data)
				workerData.AccountabilitySummary = &summaryStr
			}

			resp.Workers[workerID] = workerData
		}
	}

	// Load observer data (if observer directory exists)
	observerDir := filepath.Join(req.Path, "observer")
	if _, err := os.Stat(observerDir); err == nil {
		observerMessages := loadRawJSONL(filepath.Join(observerDir, "messages.jsonl"))
		observerNotes := ""
		if data, err := os.ReadFile(filepath.Join(observerDir, "observer_notes.md")); err == nil { //nolint:gosec // G304: path is validated by validateSessionPath before calling this handler
			observerNotes = string(data)
		}
		// Only include observer data if there are messages or notes
		if len(observerMessages) > 0 || observerNotes != "" {
			resp.Observer = &ObserverData{
				Messages: observerMessages,
				Notes:    observerNotes,
			}
		}
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// validateSessionPath validates that a path is safe to access.
// It rejects path traversal attempts and paths outside sessionBaseDir.
func (h *Handler) validateSessionPath(path string) error {
	// Clean the path
	clean := filepath.Clean(path)

	// Reject paths containing ".."
	if strings.Contains(clean, "..") {
		return fmt.Errorf("path traversal not allowed")
	}

	// Ensure path is under sessionBaseDir
	// Use path separator boundary check to prevent sibling directory access
	// (e.g., /home/user/sessionsx when baseDir is /home/user/sessions)
	baseDir := filepath.Clean(h.sessionBaseDir)
	baseDirWithSep := baseDir + string(filepath.Separator)
	if clean != baseDir && !strings.HasPrefix(clean, baseDirWithSep) {
		return fmt.Errorf("path not under session directory")
	}

	return nil
}

// convertMetadata converts session.Metadata to the frontend SessionMetadata type.
func (h *Handler) convertMetadata(meta *session.Metadata) *SessionMetadata {
	workers := make([]WorkerMeta, 0, len(meta.Workers))
	for _, w := range meta.Workers {
		wm := WorkerMeta{
			ID:                 w.ID,
			SpawnedAt:          w.SpawnedAt.Format("2006-01-02T15:04:05Z07:00"),
			HeadlessSessionRef: w.HeadlessSessionRef,
			WorkDir:            w.WorkDir,
		}
		// Include worker token usage if available
		if w.TokenUsage.ContextTokens > 0 || w.TokenUsage.TotalOutputTokens > 0 || w.TokenUsage.TotalCostUSD > 0 {
			wm.TokenUsage = &TokenUsageSummary{
				TotalInputTokens:  w.TokenUsage.ContextTokens,
				TotalOutputTokens: w.TokenUsage.TotalOutputTokens,
				TotalCostUSD:      w.TokenUsage.TotalCostUSD,
			}
		}
		workers = append(workers, wm)
	}

	return &SessionMetadata{
		SessionID:             meta.SessionID,
		StartTime:             meta.StartTime.Format("2006-01-02T15:04:05Z07:00"),
		Status:                string(meta.Status),
		SessionDir:            meta.SessionDir,
		CoordinatorSessionRef: meta.CoordinatorSessionRef,
		Resumable:             meta.Resumable,
		Workers:               workers,
		ClientType:            meta.ClientType,
		TokenUsage: TokenUsageSummary{
			// Map context_tokens to total_input_tokens for frontend compatibility
			TotalInputTokens:  meta.TokenUsage.ContextTokens,
			TotalOutputTokens: meta.TokenUsage.TotalOutputTokens,
			TotalCostUSD:      meta.TokenUsage.TotalCostUSD,
		},
		ApplicationName: meta.ApplicationName,
		WorkDir:         meta.WorkDir,
		DatePartition:   meta.DatePartition,
		WorkflowID:      meta.WorkflowID,
	}
}

// loadRawJSONL loads a JSONL file and returns its contents as raw JSON messages.
// Returns an empty slice if the file doesn't exist or is empty.
// Malformed lines are skipped gracefully.
func loadRawJSONL(path string) []json.RawMessage {
	file, err := os.Open(path) //nolint:gosec // G304: path is validated by validateSessionPath before calling this function
	if err != nil {
		if os.IsNotExist(err) {
			return []json.RawMessage{}
		}
		log.Warn(log.CatOrch, "Failed to open JSONL file", "path", path, "error", err)
		return []json.RawMessage{}
	}
	defer func() { _ = file.Close() }()

	var messages []json.RawMessage
	scanner := bufio.NewScanner(file)

	// Increase buffer size for potentially long lines
	buf := make([]byte, maxLineSize)
	scanner.Buffer(buf, maxLineSize)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue // Skip empty lines
		}

		// Validate JSON before adding
		if !json.Valid(line) {
			log.Debug(log.CatOrch, "Skipping invalid JSON line", "path", path)
			continue
		}

		// Make a copy since scanner reuses buffer
		msg := make(json.RawMessage, len(line))
		copy(msg, line)
		messages = append(messages, msg)
	}

	if err := scanner.Err(); err != nil {
		log.Warn(log.CatOrch, "Error scanning JSONL file", "path", path, "error", err)
	}

	// Ensure we return an empty slice, not nil
	if messages == nil {
		messages = []json.RawMessage{}
	}

	return messages
}

// isValidDateFormat checks if a string looks like a date in YYYY-MM-DD format.
func isValidDateFormat(s string) bool {
	if len(s) != 10 {
		return false
	}
	// Check format: YYYY-MM-DD
	for i, c := range s {
		if i == 4 || i == 7 {
			if c != '-' {
				return false
			}
		} else {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}

// writeJSON writes a JSON response with the given status code.
func (h *Handler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Error(log.CatOrch, "Failed to encode JSON response", "error", err)
	}
}

// writeError writes an error response in the standard APIError format.
func (h *Handler) writeError(w http.ResponseWriter, status int, code, message, details string) {
	h.writeJSON(w, status, APIError{
		Error:   message,
		Code:    code,
		Details: details,
	})
}

// === Fabric Messaging Handlers ===

// SendMessage creates a new thread in a channel.
// POST /api/fabric/send-message
func (h *Handler) SendMessage(w http.ResponseWriter, r *http.Request) {
	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body", err.Error())
		return
	}

	// Validate content
	if err := h.validateContent(req.Content); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error(), "")
		return
	}

	// Get workflow and fabric service
	fabricSvc, err := h.getFabricService(r.Context(), req.WorkflowID)
	if err != nil {
		if errors.Is(err, controlplane.ErrWorkflowNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Workflow not found", req.WorkflowID)
			return
		}
		h.writeError(w, http.StatusServiceUnavailable, "service_unavailable", "Fabric service unavailable", err.Error())
		return
	}

	// Send message
	msg, err := fabricSvc.SendMessage(fabric.SendMessageInput{
		ChannelSlug: req.ChannelSlug,
		Content:     req.Content,
		Kind:        domain.KindInfo,
		CreatedBy:   "user",
		Mentions:    req.Mentions,
	})
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "send_failed", "Failed to send message", err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, SendMessageResponse{
		Success:   true,
		MessageID: msg.ID,
	})
}

// Reply adds a reply to an existing thread.
// POST /api/fabric/reply
func (h *Handler) Reply(w http.ResponseWriter, r *http.Request) {
	var req ReplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body", err.Error())
		return
	}

	// Validate content
	if err := h.validateContent(req.Content); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error(), "")
		return
	}

	// Get workflow and fabric service
	fabricSvc, err := h.getFabricService(r.Context(), req.WorkflowID)
	if err != nil {
		if errors.Is(err, controlplane.ErrWorkflowNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Workflow not found", req.WorkflowID)
			return
		}
		h.writeError(w, http.StatusServiceUnavailable, "service_unavailable", "Fabric service unavailable", err.Error())
		return
	}

	// Reply to message
	reply, err := fabricSvc.Reply(fabric.ReplyInput{
		MessageID: req.ThreadID,
		Content:   req.Content,
		Kind:      domain.KindResponse,
		CreatedBy: "user",
		Mentions:  req.Mentions,
	})
	if err != nil {
		// Check if this is a "not found" error for the thread
		if strings.Contains(err.Error(), "get parent message") {
			h.writeError(w, http.StatusNotFound, "not_found", "Thread not found", req.ThreadID)
			return
		}
		h.writeError(w, http.StatusInternalServerError, "reply_failed", "Failed to reply", err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, SendMessageResponse{
		Success:   true,
		MessageID: reply.ID,
	})
}

// React adds or removes a reaction on a message.
// POST /api/fabric/react
func (h *Handler) React(w http.ResponseWriter, r *http.Request) {
	var req ReactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body", err.Error())
		return
	}

	// Validate required fields
	if req.MessageID == "" {
		h.writeError(w, http.StatusBadRequest, "validation_error", "messageId is required", "")
		return
	}
	if req.Emoji == "" {
		h.writeError(w, http.StatusBadRequest, "validation_error", "emoji is required", "")
		return
	}

	// Get workflow and fabric service
	fabricSvc, err := h.getFabricService(r.Context(), req.WorkflowID)
	if err != nil {
		if errors.Is(err, controlplane.ErrWorkflowNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Workflow not found", req.WorkflowID)
			return
		}
		h.writeError(w, http.StatusServiceUnavailable, "service_unavailable", "Fabric service unavailable", err.Error())
		return
	}

	// Add or remove reaction
	if req.Remove {
		if err := fabricSvc.RemoveReaction(req.MessageID, "user", req.Emoji); err != nil {
			h.writeError(w, http.StatusInternalServerError, "react_failed", "Failed to remove reaction", err.Error())
			return
		}
	} else {
		if _, err := fabricSvc.AddReaction(req.MessageID, "user", req.Emoji); err != nil {
			h.writeError(w, http.StatusInternalServerError, "react_failed", "Failed to add reaction", err.Error())
			return
		}
	}

	h.writeJSON(w, http.StatusOK, ReactResponse{
		Success: true,
	})
}

// ListAgents returns available agents for a workflow.
// GET /api/fabric/agents?workflowId=...
func (h *Handler) ListAgents(w http.ResponseWriter, r *http.Request) {
	workflowID := r.URL.Query().Get("workflowId")
	if workflowID == "" {
		h.writeError(w, http.StatusBadRequest, "validation_error", "workflowId query parameter is required", "")
		return
	}

	// Check if ControlPlane is available
	if h.controlPlane == nil {
		h.writeJSON(w, http.StatusOK, AgentsResponse{
			Agents:   []Agent{},
			IsActive: false,
		})
		return
	}

	// Get workflow
	wf, err := h.controlPlane.Get(r.Context(), controlplane.WorkflowID(workflowID))
	if err != nil {
		if errors.Is(err, controlplane.ErrWorkflowNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Workflow not found", workflowID)
			return
		}
		h.writeError(w, http.StatusInternalServerError, "get_failed", "Failed to get workflow", err.Error())
		return
	}

	// Determine if workflow is active
	isActive := wf.State == controlplane.WorkflowRunning || wf.State == controlplane.WorkflowPaused

	// Get agents from process repository
	agents := []Agent{}
	if wf.Infrastructure != nil {
		// Add coordinator
		if coord, err := wf.Infrastructure.Repositories.ProcessRepo.GetCoordinator(); err == nil {
			agents = append(agents, Agent{
				ID:   coord.ID,
				Role: "coordinator",
			})
		}

		// Add workers
		for _, worker := range wf.Infrastructure.Repositories.ProcessRepo.Workers() {
			agents = append(agents, Agent{
				ID:   worker.ID,
				Role: "worker",
			})
		}
	}

	h.writeJSON(w, http.StatusOK, AgentsResponse{
		Agents:   agents,
		IsActive: isActive,
	})
}

// validateContent checks that content is valid for a fabric message.
func (h *Handler) validateContent(content string) error {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return fmt.Errorf("content cannot be empty")
	}
	if len(content) > maxContentLength {
		return fmt.Errorf("content exceeds maximum length of %d characters", maxContentLength)
	}
	return nil
}

// getFabricService retrieves the fabric service for a workflow.
func (h *Handler) getFabricService(ctx context.Context, workflowID string) (*fabric.Service, error) {
	if h.controlPlane == nil {
		return nil, fmt.Errorf("control plane not available")
	}

	wf, err := h.controlPlane.Get(ctx, controlplane.WorkflowID(workflowID))
	if err != nil {
		return nil, err
	}

	if wf.Infrastructure == nil {
		return nil, fmt.Errorf("workflow infrastructure not initialized")
	}

	if wf.Infrastructure.Core.FabricService == nil {
		return nil, fmt.Errorf("fabric service not available")
	}

	return wf.Infrastructure.Core.FabricService, nil
}

// ReadFile reads a file from the filesystem and returns its contents.
// GET /api/file?path=/path/to/file
// Security: Requires absolute paths, rejects path traversal attempts.
// This endpoint is used by the session viewer to display artifacts attached
// via fabric_attach, which may reference files anywhere on the filesystem
// (e.g., project working directories, session directories).
// Since this server only listens on localhost, the security boundary is
// the local user's filesystem access.
func (h *Handler) ReadFile(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "path parameter required", http.StatusBadRequest)
		return
	}

	// Security: Require absolute paths and reject path traversal
	clean := filepath.Clean(filePath)
	if !filepath.IsAbs(clean) {
		http.Error(w, "access denied: path must be absolute", http.StatusForbidden)
		return
	}
	if strings.Contains(clean, "..") {
		http.Error(w, "access denied: path traversal not allowed", http.StatusForbidden)
		return
	}

	// Read the file
	// G304: filePath is cleaned above and validated as absolute with no traversal
	content, err := os.ReadFile(clean) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		http.Error(w, "error reading file", http.StatusInternalServerError)
		return
	}

	// Set content type based on file extension
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(content)
}
