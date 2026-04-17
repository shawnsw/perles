//nolint:tagliatelle // JSON tags use camelCase to match frontend TypeScript types
package frontend

import "encoding/json"

// SessionListResponse is the response for GET /api/sessions.
// It returns sessions organized hierarchically by application and date.
type SessionListResponse struct {
	BasePath string        `json:"basePath"`
	Apps     []AppSessions `json:"apps"`
}

// AppSessions groups sessions by application name.
type AppSessions struct {
	Name  string      `json:"name"`
	Dates []DateGroup `json:"dates"`
}

// DateGroup groups sessions by date partition.
type DateGroup struct {
	Date     string           `json:"date"`
	Sessions []SessionSummary `json:"sessions"`
}

// SessionSummary provides a lightweight summary of a session for listing.
type SessionSummary struct {
	ID          string  `json:"id"`
	Path        string  `json:"path"`
	StartTime   *string `json:"startTime"`
	Status      string  `json:"status"`
	WorkerCount int     `json:"workerCount"`
	ClientType  string  `json:"clientType"`
}

// LoadSessionRequest is the request body for POST /api/load-session.
type LoadSessionRequest struct {
	Path string `json:"path"`
}

// LoadSessionResponse contains all session data for the viewer.
// Fields use json.RawMessage to preserve the original JSONL structure
// without needing to parse every event type.
type LoadSessionResponse struct {
	Path        string                `json:"path"`
	Metadata    *SessionMetadata      `json:"metadata"`
	Fabric      []json.RawMessage     `json:"fabric"`
	MCPRequests []json.RawMessage     `json:"mcpRequests"`
	Commands    []json.RawMessage     `json:"commands"`
	Messages    []json.RawMessage     `json:"messages"`
	Coordinator CoordinatorData       `json:"coordinator"`
	Workers     map[string]WorkerData `json:"workers"`
	Observer    *ObserverData         `json:"observer,omitempty"`
}

// SessionMetadata contains parsed session metadata.
// Field names use snake_case JSON tags to match the existing metadata.json format
// that the backend writes.
type SessionMetadata struct {
	SessionID             string            `json:"session_id"`
	StartTime             string            `json:"start_time"`
	Status                string            `json:"status"`
	SessionDir            string            `json:"session_dir"`
	CoordinatorSessionRef string            `json:"coordinator_session_ref"`
	Resumable             bool              `json:"resumable"`
	Workers               []WorkerMeta      `json:"workers"`
	ClientType            string            `json:"client_type"`
	TokenUsage            TokenUsageSummary `json:"token_usage"`
	ApplicationName       string            `json:"application_name"`
	WorkDir               string            `json:"work_dir"`
	DatePartition         string            `json:"date_partition"`
	WorkflowID            string            `json:"workflow_id,omitempty"`
}

// WorkerMeta contains worker metadata.
type WorkerMeta struct {
	ID                 string             `json:"id"`
	SpawnedAt          string             `json:"spawned_at"`
	HeadlessSessionRef string             `json:"headless_session_ref"`
	WorkDir            string             `json:"work_dir"`
	TokenUsage         *TokenUsageSummary `json:"token_usage,omitempty"`
}

// TokenUsageSummary aggregates token usage for display.
// Note: The frontend uses total_input_tokens while the backend uses context_tokens.
// The handler should map context_tokens to total_input_tokens for frontend compatibility.
type TokenUsageSummary struct {
	TotalInputTokens  int     `json:"total_input_tokens"`
	TotalOutputTokens int     `json:"total_output_tokens"`
	TotalCostUSD      float64 `json:"total_cost_usd"`
}

// CoordinatorData contains coordinator process data.
type CoordinatorData struct {
	Messages []json.RawMessage `json:"messages"`
	Raw      []json.RawMessage `json:"raw"`
}

// WorkerData contains worker process data.
type WorkerData struct {
	Messages              []json.RawMessage `json:"messages"`
	Raw                   []json.RawMessage `json:"raw"`
	AccountabilitySummary *string           `json:"accountabilitySummary,omitempty"`
}

// ObserverData contains observer process data.
type ObserverData struct {
	Messages []json.RawMessage `json:"messages"`
	Notes    string            `json:"notes"`
}

// APIError provides consistent error response format.
type APIError struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// HealthResponse is the response for GET /api/health.
type HealthResponse struct {
	Status string `json:"status"`
}

// === Fabric Messaging Types ===

// SendMessageRequest is the request body for POST /api/fabric/send-message.
type SendMessageRequest struct {
	WorkflowID  string   `json:"workflowId"`
	SessionPath string   `json:"sessionPath,omitempty"`
	ChannelSlug string   `json:"channelSlug"`
	Content     string   `json:"content"`
	Mentions    []string `json:"mentions,omitempty"`
}

// SendMessageResponse is the response for POST /api/fabric/send-message.
type SendMessageResponse struct {
	Success   bool   `json:"success"`
	MessageID string `json:"messageId,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ReplyRequest is the request body for POST /api/fabric/reply.
type ReplyRequest struct {
	WorkflowID  string   `json:"workflowId"`
	SessionPath string   `json:"sessionPath,omitempty"`
	ThreadID    string   `json:"threadId"`
	Content     string   `json:"content"`
	Mentions    []string `json:"mentions,omitempty"`
}

// Agent represents an agent (coordinator or worker) in a workflow.
type Agent struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}

// AgentsResponse is the response for GET /api/fabric/agents.
type AgentsResponse struct {
	Agents   []Agent `json:"agents"`
	IsActive bool    `json:"isActive"`
}

// ReactRequest is the request body for POST /api/fabric/react.
type ReactRequest struct {
	WorkflowID string `json:"workflowId"`
	MessageID  string `json:"messageId"`
	Emoji      string `json:"emoji"`
	Remove     bool   `json:"remove,omitempty"`
}

// ReactResponse is the response for POST /api/fabric/react.
type ReactResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}
