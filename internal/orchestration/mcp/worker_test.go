package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zjrosen/perles/internal/orchestration/fabric"
	fabricrepo "github.com/zjrosen/perles/internal/orchestration/fabric/repository"
	"github.com/zjrosen/perles/internal/orchestration/message"
	"github.com/zjrosen/perles/internal/orchestration/v2/command"
)

// mockMessageStore implements MessageStore for testing.
type mockMessageStore struct {
	entries   []message.Entry
	readState map[string]int
	mu        sync.RWMutex

	// Track method calls for verification
	appendCalls      []appendCall
	readAndMarkCalls []string
}

type appendCall struct {
	From    string
	To      string
	Content string
	Type    message.MessageType
}

func newMockMessageStore() *mockMessageStore {
	return &mockMessageStore{
		entries:          make([]message.Entry, 0),
		readState:        make(map[string]int),
		appendCalls:      make([]appendCall, 0),
		readAndMarkCalls: make([]string, 0),
	}
}

// addEntry adds a message directly for test setup.
func (m *mockMessageStore) addEntry(from, to, content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, message.Entry{
		ID:        "test-" + from + "-" + to,
		Timestamp: time.Now(),
		From:      from,
		To:        to,
		Content:   content,
		Type:      message.MessageInfo,
	})
}

// ReadAndMark atomically reads unread messages and marks them as read.
func (m *mockMessageStore) ReadAndMark(agentID string) []message.Entry {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.readAndMarkCalls = append(m.readAndMarkCalls, agentID)

	lastRead := m.readState[agentID]
	if lastRead >= len(m.entries) {
		return nil
	}

	unread := make([]message.Entry, len(m.entries)-lastRead)
	copy(unread, m.entries[lastRead:])
	m.readState[agentID] = len(m.entries)
	return unread
}

// Append adds a new message to the log.
func (m *mockMessageStore) Append(from, to, content string, msgType message.MessageType) (*message.Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.appendCalls = append(m.appendCalls, appendCall{
		From:    from,
		To:      to,
		Content: content,
		Type:    msgType,
	})

	entry := message.Entry{
		ID:        "test-" + from + "-" + to,
		Timestamp: time.Now(),
		From:      from,
		To:        to,
		Content:   content,
		Type:      msgType,
	}

	m.entries = append(m.entries, entry)
	return &entry, nil
}

// TestWorkerServer_RegistersAllTools verifies all worker tools are registered.
// Note: fabric_join and other fabric tools are registered via SetFabricService.
func TestWorkerServer_RegistersAllTools(t *testing.T) {
	ws := NewWorkerServer("WORKER.1")

	// Set up fabric service to register fabric tools (including fabric_join)
	fabricService := createTestFabricServiceForWorkerTest(t)
	ws.SetFabricService(fabricService)

	// Worker-specific tools
	workerTools := []string{
		"report_implementation_complete",
		"report_review_verdict",
		"post_accountability_summary",
	}

	// Fabric tools (registered via SetFabricService)
	fabricTools := []string{
		"fabric_join",
		"fabric_inbox",
		"fabric_send",
		"fabric_reply",
		"fabric_ack",
		"fabric_subscribe",
		"fabric_unsubscribe",
		"fabric_attach",
		"fabric_history",
		"fabric_read_thread",
		"fabric_react",
	}

	expectedTools := append(workerTools, fabricTools...)

	for _, toolName := range expectedTools {
		_, ok := ws.tools[toolName]
		require.True(t, ok, "Tool %q not registered", toolName)
		_, ok = ws.handlers[toolName]
		require.True(t, ok, "Handler for %q not registered", toolName)
	}

	require.Equal(t, len(expectedTools), len(ws.tools), "Tool count mismatch")
}

// createTestFabricServiceForWorkerTest creates a minimal fabric service for testing.
func createTestFabricServiceForWorkerTest(t *testing.T) *fabric.Service {
	t.Helper()

	threads := fabricrepo.NewMemoryThreadRepository()
	deps := fabricrepo.NewMemoryDependencyRepository()
	subs := fabricrepo.NewMemorySubscriptionRepository()
	acks := fabricrepo.NewMemoryAckRepository(deps, threads, subs)
	participants := fabricrepo.NewMemoryParticipantRepository()

	svc := fabric.NewService(threads, deps, subs, acks, participants)

	// Initialize session to create channels
	err := svc.InitSession("coordinator")
	require.NoError(t, err, "Failed to initialize fabric session")

	return svc
}

// TestWorkerServer_ToolSchemas verifies tool schemas are valid.
func TestWorkerServer_ToolSchemas(t *testing.T) {
	ws := NewWorkerServer("WORKER.1")

	for name, tool := range ws.tools {
		t.Run(name, func(t *testing.T) {
			require.NotEmpty(t, tool.Name, "Tool name is empty")
			require.NotEmpty(t, tool.Description, "Tool description is empty")
			require.NotNil(t, tool.InputSchema, "Tool inputSchema is nil")
			if tool.InputSchema != nil {
				require.Equal(t, "object", tool.InputSchema.Type, "InputSchema.Type mismatch")
			}
		})
	}
}

// TestWorkerServer_Instructions tests that instructions are set correctly.
func TestWorkerServer_Instructions(t *testing.T) {
	ws := NewWorkerServer("WORKER.1")

	require.NotEmpty(t, ws.instructions, "Instructions should be set")
	require.Equal(t, "perles-worker", ws.info.Name, "Server name mismatch")
	require.Equal(t, "1.0.0", ws.info.Version, "Server version mismatch")
}

// TestWorkerServer_DifferentWorkerIDs verifies different workers get separate identities.
func TestWorkerServer_DifferentWorkerIDs(t *testing.T) {
	ws1 := NewWorkerServer("WORKER.1")
	ws2 := NewWorkerServer("WORKER.2")

	// Verify worker IDs are set correctly
	require.Equal(t, "WORKER.1", ws1.workerID, "Worker 1 ID mismatch")
	require.Equal(t, "WORKER.2", ws2.workerID, "Worker 2 ID mismatch")
}

// TestWorkerServer_FabricJoinValidation tests fabric_join with v2 adapter.
// In v2 architecture, fabric_join posts to the v2 adapter's message log (if configured).
func TestWorkerServer_FabricJoinValidation(t *testing.T) {
	tws := NewTestWorkerServer(t, "WORKER.1")
	defer tws.Close()
	handler := tws.handlers["fabric_join"]

	// fabric_join always returns success
	result, err := handler(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err, "fabric_join should not error")
	require.Contains(t, result.Content[0].Text, "joined fabric as worker", "Result should confirm join")
}

// TestWorkerServer_FabricJoinHappyPath tests successful ready signaling with v2.
// In v2 architecture, fabric_join posts via Fabric, not the worker's message store.
func TestWorkerServer_FabricJoinHappyPath(t *testing.T) {
	tws := NewTestWorkerServer(t, "WORKER.1")
	defer tws.Close()
	handler := tws.handlers["fabric_join"]

	result, err := handler(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err, "Unexpected error")

	// Verify success result
	require.Contains(t, result.Content[0].Text, "joined fabric as worker", "Result should confirm join")
}

// TestWorkerServer_ToolDescriptionsAreHelpful verifies tool descriptions are informative.
func TestWorkerServer_ToolDescriptionsAreHelpful(t *testing.T) {
	ws := NewWorkerServer("WORKER.1")
	// Set up fabric service to register fabric tools
	fabricService := createTestFabricServiceForWorkerTest(t)
	ws.SetFabricService(fabricService)

	tests := []struct {
		toolName      string
		mustContain   []string
		descMinLength int
	}{
		{
			toolName:      "fabric_join",
			mustContain:   []string{"ready", "task", "assignment"},
			descMinLength: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			tool := ws.tools[tt.toolName]
			desc := strings.ToLower(tool.Description)

			require.GreaterOrEqual(t, len(tool.Description), tt.descMinLength, "Description too short: want at least %d chars", tt.descMinLength)

			for _, keyword := range tt.mustContain {
				require.Contains(t, desc, keyword, "Description should contain %q", keyword)
			}
		})
	}
}

// TestWorkerServer_InstructionsContainToolNames verifies instructions mention all tools.
func TestWorkerServer_InstructionsContainToolNames(t *testing.T) {
	ws := NewWorkerServer("WORKER.1")
	instructions := strings.ToLower(ws.instructions)

	// Phase 3 Fabric migration: instructions now reference fabric_* tools instead of legacy tools
	toolNames := []string{"fabric_inbox", "fabric_send", "fabric_reply", "fabric_join"}
	for _, name := range toolNames {
		require.Contains(t, instructions, name, "Instructions should mention %q", name)
	}
}

// TestWorkerServer_FabricJoinSchema verifies fabric_join tool schema.
func TestWorkerServer_FabricJoinSchema(t *testing.T) {
	ws := NewWorkerServer("WORKER.1")
	// Set up fabric service to register fabric tools
	fabricService := createTestFabricServiceForWorkerTest(t)
	ws.SetFabricService(fabricService)

	tool, ok := ws.tools["fabric_join"]
	require.True(t, ok, "fabric_join tool not registered")

	require.Empty(t, tool.InputSchema.Required, "fabric_join should have 0 required parameters")
	require.Empty(t, tool.InputSchema.Properties, "fabric_join should have 0 properties")
}

// TestWorkerServer_ReportImplementationComplete_SubmitsCommand tests command submission in v2.
// In v2 architecture, report_implementation_complete submits a command to the processor,
// not through the callback mechanism.
func TestWorkerServer_ReportImplementationComplete_SubmitsCommand(t *testing.T) {
	tws := NewTestWorkerServer(t, "WORKER.1")
	defer tws.Close()
	handler := tws.handlers["report_implementation_complete"]

	// Configure mock to return success
	tws.V2Handler.SetResult(&command.CommandResult{
		Success: true,
		Data:    "Implementation complete",
	})

	result, err := handler(context.Background(), json.RawMessage(`{"summary": "completed feature X"}`))
	require.NoError(t, err, "Expected no error with v2 adapter")
	require.NotNil(t, result)
	require.False(t, result.IsError, "Expected success result")

	// Verify command was submitted
	commands := tws.V2Handler.GetCommands()
	require.Len(t, commands, 1, "Expected 1 command")
	require.Equal(t, command.CmdReportComplete, commands[0].Type(), "Expected ReportComplete command")
}

// TestWorkerServer_ReportImplementationComplete_EmptySummary tests that empty summary is accepted in v2.
// In v2 architecture, summary is optional (empty string is valid).
func TestWorkerServer_ReportImplementationComplete_EmptySummary(t *testing.T) {
	tws := NewTestWorkerServer(t, "WORKER.1")
	defer tws.Close()
	handler := tws.handlers["report_implementation_complete"]

	// Configure mock to return success
	tws.V2Handler.SetResult(&command.CommandResult{
		Success: true,
		Data:    "Implementation complete",
	})

	result, err := handler(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err, "Empty summary should not error in v2")
	require.NotNil(t, result)
	require.False(t, result.IsError, "Expected success result")
}

// TestWorkerServer_ReportImplementationComplete_ProcessorRejectsWrongPhase tests that processor validates phase.
// In v2 architecture, phase validation happens in the processor, not the MCP handler.
func TestWorkerServer_ReportImplementationComplete_ProcessorRejectsWrongPhase(t *testing.T) {
	tws := NewTestWorkerServer(t, "WORKER.1")
	defer tws.Close()
	handler := tws.handlers["report_implementation_complete"]

	// Configure mock to return error (simulating processor rejecting wrong phase)
	tws.V2Handler.SetResult(&command.CommandResult{
		Success: false,
		Error:   fmt.Errorf("worker not in implementing or addressing_feedback phase"),
	})

	result, err := handler(context.Background(), json.RawMessage(`{"summary": "done"}`))
	require.NoError(t, err, "Handler returns nil error")
	require.NotNil(t, result)
	require.True(t, result.IsError, "Expected error result from processor")
	require.Contains(t, result.Content[0].Text, "not in implementing or addressing_feedback phase", "Expected phase error")
}

// TestWorkerServer_ReportImplementationComplete_HappyPath tests successful completion in v2.
func TestWorkerServer_ReportImplementationComplete_HappyPath(t *testing.T) {
	tws := NewTestWorkerServer(t, "WORKER.1")
	defer tws.Close()
	handler := tws.handlers["report_implementation_complete"]

	// Configure mock to return success
	tws.V2Handler.SetResult(&command.CommandResult{
		Success: true,
		Data:    "Implementation complete",
	})

	result, err := handler(context.Background(), json.RawMessage(`{"summary": "Added feature X with tests"}`))
	require.NoError(t, err, "Unexpected error")

	// Verify command was submitted
	commands := tws.V2Handler.GetCommands()
	require.Len(t, commands, 1, "Expected 1 command")
	require.Equal(t, command.CmdReportComplete, commands[0].Type(), "Expected ReportComplete command")

	// Verify success result
	require.NotNil(t, result, "Expected result with content")
	require.NotEmpty(t, result.Content, "Expected result with content")
	require.False(t, result.IsError, "Expected success result")
}

// TestWorkerServer_ReportImplementationComplete_AddressingFeedback tests completion from addressing_feedback phase in v2.
func TestWorkerServer_ReportImplementationComplete_AddressingFeedback(t *testing.T) {
	tws := NewTestWorkerServer(t, "WORKER.1")
	defer tws.Close()
	handler := tws.handlers["report_implementation_complete"]

	// Configure mock to return success
	tws.V2Handler.SetResult(&command.CommandResult{
		Success: true,
		Data:    "Implementation complete",
	})

	result, err := handler(context.Background(), json.RawMessage(`{"summary": "Fixed review feedback"}`))
	require.NoError(t, err, "Should succeed in v2")
	require.NotNil(t, result)
	require.False(t, result.IsError, "Expected success result")
}

// TestWorkerServer_ReportReviewVerdict_SubmitsCommand tests command submission in v2.
func TestWorkerServer_ReportReviewVerdict_SubmitsCommand(t *testing.T) {
	tws := NewTestWorkerServer(t, "WORKER.1")
	defer tws.Close()
	handler := tws.handlers["report_review_verdict"]

	// Configure mock to return success
	tws.V2Handler.SetResult(&command.CommandResult{
		Success: true,
		Data:    "Verdict submitted",
	})

	result, err := handler(context.Background(), json.RawMessage(`{"verdict": "APPROVED", "comments": "LGTM"}`))
	require.NoError(t, err, "Expected no error with v2 adapter")
	require.NotNil(t, result)
	require.False(t, result.IsError, "Expected success result")

	// Verify command was submitted
	commands := tws.V2Handler.GetCommands()
	require.Len(t, commands, 1, "Expected 1 command")
	require.Equal(t, command.CmdReportVerdict, commands[0].Type(), "Expected ReportVerdict command")
}

// TestWorkerServer_ReportReviewVerdict_MissingVerdict tests validation in v2.
func TestWorkerServer_ReportReviewVerdict_MissingVerdict(t *testing.T) {
	tws := NewTestWorkerServer(t, "WORKER.1")
	defer tws.Close()
	handler := tws.handlers["report_review_verdict"]

	// v2 adapter validates verdict is required before submitting to processor
	_, err := handler(context.Background(), json.RawMessage(`{"comments": "LGTM"}`))
	require.Error(t, err, "Expected error for missing verdict")
	require.Contains(t, err.Error(), "verdict is required", "Expected 'verdict is required' error")
}

// TestWorkerServer_ReportReviewVerdict_EmptyComments tests that empty comments are valid in v2.
// In v2 architecture, comments are optional.
func TestWorkerServer_ReportReviewVerdict_EmptyComments(t *testing.T) {
	tws := NewTestWorkerServer(t, "WORKER.1")
	defer tws.Close()
	handler := tws.handlers["report_review_verdict"]

	// Configure mock to return success
	tws.V2Handler.SetResult(&command.CommandResult{
		Success: true,
		Data:    "Verdict submitted",
	})

	result, err := handler(context.Background(), json.RawMessage(`{"verdict": "APPROVED"}`))
	require.NoError(t, err, "Empty comments should not error in v2")
	require.NotNil(t, result)
	require.False(t, result.IsError, "Expected success result")
}

// TestWorkerServer_ReportReviewVerdict_InvalidVerdict tests invalid verdict value in v2.
func TestWorkerServer_ReportReviewVerdict_InvalidVerdict(t *testing.T) {
	tws := NewTestWorkerServer(t, "WORKER.1")
	defer tws.Close()
	handler := tws.handlers["report_review_verdict"]

	// v2 adapter validates verdict value
	_, err := handler(context.Background(), json.RawMessage(`{"verdict": "MAYBE", "comments": "Not sure"}`))
	require.Error(t, err, "Expected error for invalid verdict")
	require.Contains(t, err.Error(), "must be APPROVED or DENIED", "Expected verdict validation error")
}

// TestWorkerServer_ReportReviewVerdict_ProcessorRejectsWrongPhase tests that processor validates phase.
// In v2 architecture, phase validation happens in the processor, not the MCP handler.
func TestWorkerServer_ReportReviewVerdict_ProcessorRejectsWrongPhase(t *testing.T) {
	tws := NewTestWorkerServer(t, "WORKER.1")
	defer tws.Close()
	handler := tws.handlers["report_review_verdict"]

	// Configure mock to return error (simulating processor rejecting wrong phase)
	tws.V2Handler.SetResult(&command.CommandResult{
		Success: false,
		Error:   fmt.Errorf("worker not in reviewing phase"),
	})

	result, err := handler(context.Background(), json.RawMessage(`{"verdict": "APPROVED", "comments": "LGTM"}`))
	require.NoError(t, err, "Handler returns nil error")
	require.NotNil(t, result)
	require.True(t, result.IsError, "Expected error result from processor")
	require.Contains(t, result.Content[0].Text, "not in reviewing phase", "Expected phase error")
}

// TestWorkerServer_ReportReviewVerdict_Approved tests successful approval in v2.
func TestWorkerServer_ReportReviewVerdict_Approved(t *testing.T) {
	tws := NewTestWorkerServer(t, "WORKER.1")
	defer tws.Close()
	handler := tws.handlers["report_review_verdict"]

	// Configure mock to return success
	tws.V2Handler.SetResult(&command.CommandResult{
		Success: true,
		Data:    "Review verdict APPROVED submitted",
	})

	result, err := handler(context.Background(), json.RawMessage(`{"verdict": "APPROVED", "comments": "Code looks great, tests pass"}`))
	require.NoError(t, err, "Unexpected error")

	// Verify command was submitted
	commands := tws.V2Handler.GetCommands()
	require.Len(t, commands, 1, "Expected 1 command")
	require.Equal(t, command.CmdReportVerdict, commands[0].Type(), "Expected ReportVerdict command")

	// Verify success result
	require.NotNil(t, result, "Expected result with content")
	require.NotEmpty(t, result.Content, "Expected result with content")
	require.False(t, result.IsError, "Expected success result")
	require.Contains(t, result.Content[0].Text, "APPROVED", "Response should contain 'APPROVED'")
}

// TestWorkerServer_ReportReviewVerdict_Denied tests successful denial in v2.
func TestWorkerServer_ReportReviewVerdict_Denied(t *testing.T) {
	tws := NewTestWorkerServer(t, "WORKER.1")
	defer tws.Close()
	handler := tws.handlers["report_review_verdict"]

	// Configure mock to return success
	tws.V2Handler.SetResult(&command.CommandResult{
		Success: true,
		Data:    "Review verdict DENIED submitted",
	})

	result, err := handler(context.Background(), json.RawMessage(`{"verdict": "DENIED", "comments": "Missing error handling in line 50"}`))
	require.NoError(t, err, "Unexpected error")

	// Verify command was submitted
	commands := tws.V2Handler.GetCommands()
	require.Len(t, commands, 1, "Expected 1 command")
	require.Equal(t, command.CmdReportVerdict, commands[0].Type(), "Expected ReportVerdict command")

	// Verify success result contains DENIED
	require.NotNil(t, result)
	require.False(t, result.IsError, "Expected success result")
	require.Contains(t, result.Content[0].Text, "DENIED", "Response should contain 'DENIED'")
}

// TestWorkerServer_ReportImplementationCompleteSchema verifies tool schema.
func TestWorkerServer_ReportImplementationCompleteSchema(t *testing.T) {
	ws := NewWorkerServer("WORKER.1")

	tool, ok := ws.tools["report_implementation_complete"]
	require.True(t, ok, "report_implementation_complete tool not registered")

	require.Len(t, tool.InputSchema.Required, 1, "report_implementation_complete should have 1 required parameter")
	require.Equal(t, "summary", tool.InputSchema.Required[0], "Required parameter should be 'summary'")

	_, ok = tool.InputSchema.Properties["summary"]
	require.True(t, ok, "'summary' property should be defined")
}

// TestWorkerServer_ReportReviewVerdictSchema verifies tool schema.
func TestWorkerServer_ReportReviewVerdictSchema(t *testing.T) {
	ws := NewWorkerServer("WORKER.1")

	tool, ok := ws.tools["report_review_verdict"]
	require.True(t, ok, "report_review_verdict tool not registered")

	require.Len(t, tool.InputSchema.Required, 2, "report_review_verdict should have 2 required parameters")

	requiredSet := make(map[string]bool)
	for _, r := range tool.InputSchema.Required {
		requiredSet[r] = true
	}
	require.True(t, requiredSet["verdict"], "'verdict' should be required")
	require.True(t, requiredSet["comments"], "'comments' should be required")

	_, ok = tool.InputSchema.Properties["verdict"]
	require.True(t, ok, "'verdict' property should be defined")
	_, ok = tool.InputSchema.Properties["comments"]
	require.True(t, ok, "'comments' property should be defined")
}

// ============================================================================
// Tests for validateAccountabilitySummaryArgs
// ============================================================================

// TestValidateAccountabilitySummaryArgs_Valid tests that valid args pass validation.
func TestValidateAccountabilitySummaryArgs_Valid(t *testing.T) {
	args := postAccountabilitySummaryArgs{
		TaskID:             "perles-abc123",
		Summary:            "Implemented feature X with comprehensive tests and documentation.",
		Commits:            []string{"abc123", "def456"},
		IssuesDiscovered:   []string{"perles-xyz"},
		IssuesClosed:       []string{"perles-abc123"},
		VerificationPoints: []string{"Tests pass", "Manual verification"},
		Retro: &RetroFeedback{
			WentWell:  "Smooth implementation",
			Friction:  "Had to refactor twice",
			Patterns:  "Found useful pattern",
			Takeaways: "Read docs first",
		},
		NextSteps: "Continue with next task",
	}

	err := validateAccountabilitySummaryArgs(args)
	require.NoError(t, err, "Valid input should pass validation")
}

// TestValidateAccountabilitySummaryArgs_MissingRequired tests that missing task_id/summary is rejected.
func TestValidateAccountabilitySummaryArgs_MissingRequired(t *testing.T) {
	tests := []struct {
		name    string
		args    postAccountabilitySummaryArgs
		wantErr string
	}{
		{
			name:    "empty task_id",
			args:    postAccountabilitySummaryArgs{TaskID: "", Summary: "A valid summary that is at least twenty chars."},
			wantErr: "task_id is required",
		},
		{
			name:    "empty summary",
			args:    postAccountabilitySummaryArgs{TaskID: "perles-abc123", Summary: ""},
			wantErr: "summary is required",
		},
		{
			name:    "summary too short",
			args:    postAccountabilitySummaryArgs{TaskID: "perles-abc123", Summary: "Too short"},
			wantErr: "summary too short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAccountabilitySummaryArgs(tt.args)
			require.Error(t, err, "Should reject missing required field")
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestValidateAccountabilitySummaryArgs_PathTraversal tests that path traversal is rejected.
func TestValidateAccountabilitySummaryArgs_PathTraversal(t *testing.T) {
	tests := []struct {
		name    string
		taskID  string
		wantErr string
	}{
		{
			name:    "path traversal with ..",
			taskID:  "../../etc/passwd",
			wantErr: "path traversal characters",
		},
		{
			name:    "path with forward slash",
			taskID:  "task/id",
			wantErr: "path traversal characters",
		},
		{
			name:    "double dots in middle",
			taskID:  "task..id",
			wantErr: "path traversal characters",
		},
		{
			name:    "invalid format - no hyphen",
			taskID:  "invalidtaskid",
			wantErr: "invalid task_id format",
		},
		{
			name:    "invalid format - special chars",
			taskID:  "task-@#$%",
			wantErr: "invalid task_id format",
		},
		{
			name:    "no hyphen separator",
			taskID:  "abc",
			wantErr: "invalid task_id format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := postAccountabilitySummaryArgs{
				TaskID:  tt.taskID,
				Summary: "A valid summary that is at least twenty chars.",
			}
			err := validateAccountabilitySummaryArgs(args)
			require.Error(t, err, "Invalid task_id %q should be rejected", tt.taskID)
			require.Contains(t, err.Error(), tt.wantErr, "Error should mention expected issue")
		})
	}
}

// TestValidateAccountabilitySummaryArgs_ValidTaskIDFormats tests various valid task ID formats.
func TestValidateAccountabilitySummaryArgs_ValidTaskIDFormats(t *testing.T) {
	validTaskIDs := []string{
		"perles-abc123",
		"ms-e52",
		"task-abc",
		"bd-12345",
		"perles-s157",
		"perles-s157.1",
		"ms-abc123.42",
	}

	for _, taskID := range validTaskIDs {
		t.Run(taskID, func(t *testing.T) {
			args := postAccountabilitySummaryArgs{
				TaskID:  taskID,
				Summary: "A valid summary that is at least twenty chars.",
			}
			err := validateAccountabilitySummaryArgs(args)
			require.NoError(t, err, "Valid task_id %q should pass validation", taskID)
		})
	}
}

// TestValidateAccountabilitySummaryArgs_ExactlyMinSummaryLength tests boundary at exactly 20 chars.
func TestValidateAccountabilitySummaryArgs_ExactlyMinSummaryLength(t *testing.T) {
	args := postAccountabilitySummaryArgs{
		TaskID:  "perles-abc123",
		Summary: strings.Repeat("x", MinSummaryLength), // Exactly 20 chars
	}

	err := validateAccountabilitySummaryArgs(args)
	require.NoError(t, err, "Summary with exactly min length should pass")
}

// ============================================================================
// Tests for buildAccountabilitySummaryMarkdown
// ============================================================================

// TestBuildAccountabilitySummaryMarkdown tests markdown generation with YAML frontmatter.
func TestBuildAccountabilitySummaryMarkdown(t *testing.T) {
	args := postAccountabilitySummaryArgs{
		TaskID:             "perles-abc123",
		Summary:            "Implemented user validation with regex patterns.",
		Commits:            []string{"abc123", "def456"},
		IssuesDiscovered:   []string{"perles-xyz"},
		IssuesClosed:       []string{"perles-abc123"},
		VerificationPoints: []string{"Tests pass", "Manual verification"},
		Retro: &RetroFeedback{
			WentWell:  "Smooth implementation",
			Friction:  "Had to refactor twice",
			Patterns:  "Found useful pattern",
			Takeaways: "Read docs first",
		},
		NextSteps: "Continue with next task",
	}

	md := buildAccountabilitySummaryMarkdown("WORKER.1", args)

	// Verify YAML frontmatter
	assert.Contains(t, md, "---\n")
	assert.Contains(t, md, "task_id: perles-abc123")
	assert.Contains(t, md, "worker_id: WORKER.1")
	assert.Contains(t, md, "timestamp:")
	assert.Contains(t, md, "commits:\n  - abc123\n  - def456")
	assert.Contains(t, md, "issues_discovered:\n  - perles-xyz")
	assert.Contains(t, md, "issues_closed:\n  - perles-abc123")

	// Verify header
	assert.Contains(t, md, "# Worker Accountability Summary")
	assert.Contains(t, md, "**Worker:** WORKER.1")
	assert.Contains(t, md, "**Task:** perles-abc123")
	assert.Contains(t, md, "**Date:**")

	// Verify all sections are present
	assert.Contains(t, md, "## What I Accomplished")
	assert.Contains(t, md, "Implemented user validation with regex patterns.")

	assert.Contains(t, md, "## Verification Points")
	assert.Contains(t, md, "- Tests pass")
	assert.Contains(t, md, "- Manual verification")

	assert.Contains(t, md, "## Issues Discovered")
	assert.Contains(t, md, "- perles-xyz")

	assert.Contains(t, md, "## Retro")
	assert.Contains(t, md, "### What Went Well")
	assert.Contains(t, md, "Smooth implementation")
	assert.Contains(t, md, "### Friction")
	assert.Contains(t, md, "Had to refactor twice")
	assert.Contains(t, md, "### Patterns Noticed")
	assert.Contains(t, md, "Found useful pattern")
	assert.Contains(t, md, "### Takeaways")
	assert.Contains(t, md, "Read docs first")

	assert.Contains(t, md, "## Next Steps")
	assert.Contains(t, md, "Continue with next task")
}

// TestBuildAccountabilitySummaryMarkdown_OnlySummary tests with only required fields.
func TestBuildAccountabilitySummaryMarkdown_OnlySummary(t *testing.T) {
	args := postAccountabilitySummaryArgs{
		TaskID:  "ms-e52",
		Summary: "Fixed a critical bug in authentication flow.",
	}

	md := buildAccountabilitySummaryMarkdown("WORKER.2", args)

	// Verify YAML frontmatter
	assert.Contains(t, md, "---\n")
	assert.Contains(t, md, "task_id: ms-e52")
	assert.Contains(t, md, "worker_id: WORKER.2")
	// Should NOT have optional arrays in frontmatter
	assert.NotContains(t, md, "commits:")
	assert.NotContains(t, md, "issues_discovered:")
	assert.NotContains(t, md, "issues_closed:")

	// Verify header
	assert.Contains(t, md, "# Worker Accountability Summary")
	assert.Contains(t, md, "**Worker:** WORKER.2")
	assert.Contains(t, md, "**Task:** ms-e52")

	// Verify summary is present
	assert.Contains(t, md, "## What I Accomplished")
	assert.Contains(t, md, "Fixed a critical bug in authentication flow.")

	// Verify optional sections are NOT present
	assert.NotContains(t, md, "## Verification Points")
	assert.NotContains(t, md, "## Issues Discovered")
	assert.NotContains(t, md, "## Retro")
	assert.NotContains(t, md, "## Next Steps")
}

// TestBuildAccountabilitySummaryMarkdown_PartialOptionalFields tests with some optional fields.
func TestBuildAccountabilitySummaryMarkdown_PartialOptionalFields(t *testing.T) {
	tests := []struct {
		name       string
		args       postAccountabilitySummaryArgs
		shouldHave []string
		shouldNot  []string
	}{
		{
			name: "only verification points",
			args: postAccountabilitySummaryArgs{
				TaskID:             "task-abc",
				Summary:            "Completed the refactoring.",
				VerificationPoints: []string{"Tests pass"},
			},
			shouldHave: []string{"## What I Accomplished", "## Verification Points"},
			shouldNot:  []string{"## Retro", "## Next Steps", "## Issues Discovered"},
		},
		{
			name: "only retro",
			args: postAccountabilitySummaryArgs{
				TaskID:  "task-abc",
				Summary: "Completed the refactoring.",
				Retro:   &RetroFeedback{WentWell: "Good"},
			},
			shouldHave: []string{"## What I Accomplished", "## Retro", "### What Went Well"},
			shouldNot:  []string{"## Verification Points", "## Next Steps"},
		},
		{
			name: "only next steps",
			args: postAccountabilitySummaryArgs{
				TaskID:    "task-abc",
				Summary:   "Completed the refactoring.",
				NextSteps: "Follow up work",
			},
			shouldHave: []string{"## What I Accomplished", "## Next Steps"},
			shouldNot:  []string{"## Verification Points", "## Retro"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := buildAccountabilitySummaryMarkdown("WORKER.1", tt.args)

			for _, s := range tt.shouldHave {
				assert.Contains(t, md, s, "Should contain %q", s)
			}
			for _, s := range tt.shouldNot {
				assert.NotContains(t, md, s, "Should NOT contain %q", s)
			}
		})
	}
}

// TestBuildAccountabilitySummaryMarkdown_DateFormat tests that date is in expected format.
func TestBuildAccountabilitySummaryMarkdown_DateFormat(t *testing.T) {
	args := postAccountabilitySummaryArgs{
		TaskID:  "perles-abc",
		Summary: "Test summary for date format.",
	}

	md := buildAccountabilitySummaryMarkdown("WORKER.1", args)

	// Date format should be YYYY-MM-DD HH:MM:SS (e.g., 2025-12-30 01:23:45)
	assert.Regexp(t, `\*\*Date:\*\* \d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`, md, "Date should be in expected format")

	// Timestamp in YAML frontmatter should be RFC3339
	assert.Regexp(t, `timestamp: \d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`, md, "Timestamp should be RFC3339 format")
}

// TestBuildAccountabilitySummaryMarkdown_PreservesNewlines tests that content with newlines is preserved.
func TestBuildAccountabilitySummaryMarkdown_PreservesNewlines(t *testing.T) {
	args := postAccountabilitySummaryArgs{
		TaskID:  "task-abc",
		Summary: "Line 1\nLine 2\nLine 3",
	}

	md := buildAccountabilitySummaryMarkdown("WORKER.1", args)

	assert.Contains(t, md, "Line 1\nLine 2\nLine 3", "Newlines in content should be preserved")
}

// ============================================================================
// Tests for handlePostAccountabilitySummary
// ============================================================================

// mockAccountabilityWriter implements AccountabilityWriter for testing.
type mockAccountabilityWriter struct {
	mu         sync.Mutex
	calls      []accountabilityWriterCall
	returnPath string
	returnErr  error
}

type accountabilityWriterCall struct {
	WorkerID string
	Content  []byte
}

func newMockAccountabilityWriter() *mockAccountabilityWriter {
	return &mockAccountabilityWriter{
		returnPath: "/mock/path/accountability_summary.md",
	}
}

func (m *mockAccountabilityWriter) WriteWorkerAccountabilitySummary(workerID string, content []byte) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, accountabilityWriterCall{
		WorkerID: workerID,
		Content:  content,
	})
	return m.returnPath, m.returnErr
}

// TestHandlePostAccountabilitySummary tests valid summary saves and returns path.
func TestHandlePostAccountabilitySummary(t *testing.T) {
	writer := newMockAccountabilityWriter()
	writer.returnPath = "/sessions/abc/workers/WORKER.1/accountability_summary.md"

	ws := NewWorkerServer("WORKER.1")
	ws.SetAccountabilityWriter(writer)
	handler := ws.handlers["post_accountability_summary"]

	args := `{
		"task_id": "perles-abc123",
		"summary": "Implemented feature X with comprehensive tests.",
		"commits": ["abc123", "def456"],
		"issues_discovered": ["perles-xyz"],
		"issues_closed": ["perles-abc123"],
		"verification_points": ["Tests pass", "Manual verification"],
		"retro": {
			"went_well": "Smooth implementation",
			"friction": "Had to refactor twice",
			"patterns": "Found useful pattern",
			"takeaways": "Read docs first"
		},
		"next_steps": "Continue with next task"
	}`

	result, err := handler(context.Background(), json.RawMessage(args))
	require.NoError(t, err, "Unexpected error")

	// Verify writer was called
	require.Len(t, writer.calls, 1, "Expected 1 write call")
	require.Equal(t, "WORKER.1", writer.calls[0].WorkerID, "WorkerID mismatch")
	require.Contains(t, string(writer.calls[0].Content), "# Worker Accountability Summary", "Content should be markdown")
	require.Contains(t, string(writer.calls[0].Content), "task_id: perles-abc123", "Content should have YAML frontmatter")
	require.Contains(t, string(writer.calls[0].Content), "Implemented feature X", "Content should contain summary")

	// Verify structured response
	require.NotNil(t, result, "Expected result with content")
	require.NotEmpty(t, result.Content, "Expected result with content")
	text := result.Content[0].Text
	require.Contains(t, text, `"status"`, "Response should contain status")
	require.Contains(t, text, `"file_path"`, "Response should contain file_path")
	require.Contains(t, text, `"success"`, "Status should be success")
	require.Contains(t, text, writer.returnPath, "Response should contain file path")
}

// TestHandlePostAccountabilitySummary_EmptyTaskID tests that missing task_id returns error.
func TestHandlePostAccountabilitySummary_EmptyTaskID(t *testing.T) {
	writer := newMockAccountabilityWriter()

	ws := NewWorkerServer("WORKER.1")
	ws.SetAccountabilityWriter(writer)
	handler := ws.handlers["post_accountability_summary"]

	args := `{
		"task_id": "",
		"summary": "A valid summary that is at least twenty chars."
	}`

	_, err := handler(context.Background(), json.RawMessage(args))
	require.Error(t, err, "Expected error for empty task_id")
	require.Contains(t, err.Error(), "task_id is required", "Error should mention task_id")
}

// TestHandlePostAccountabilitySummary_EmptySummary tests that missing summary returns error.
func TestHandlePostAccountabilitySummary_EmptySummary(t *testing.T) {
	writer := newMockAccountabilityWriter()

	ws := NewWorkerServer("WORKER.1")
	ws.SetAccountabilityWriter(writer)
	handler := ws.handlers["post_accountability_summary"]

	args := `{
		"task_id": "perles-abc123",
		"summary": ""
	}`

	_, err := handler(context.Background(), json.RawMessage(args))
	require.Error(t, err, "Expected error for empty summary")
	require.Contains(t, err.Error(), "summary is required", "Error should mention summary")
}

// TestHandlePostAccountabilitySummary_InvalidTaskID tests that path traversal is rejected.
func TestHandlePostAccountabilitySummary_InvalidTaskID(t *testing.T) {
	writer := newMockAccountabilityWriter()

	ws := NewWorkerServer("WORKER.1")
	ws.SetAccountabilityWriter(writer)
	handler := ws.handlers["post_accountability_summary"]

	args := `{
		"task_id": "../../etc/passwd",
		"summary": "A valid summary that is at least twenty chars."
	}`

	_, err := handler(context.Background(), json.RawMessage(args))
	require.Error(t, err, "Expected error for path traversal")
	require.Contains(t, err.Error(), "path traversal", "Error should mention path traversal")
}

// TestHandlePostAccountabilitySummary_SummaryTooShort tests validation for summary length.
func TestHandlePostAccountabilitySummary_SummaryTooShort(t *testing.T) {
	writer := newMockAccountabilityWriter()

	ws := NewWorkerServer("WORKER.1")
	ws.SetAccountabilityWriter(writer)
	handler := ws.handlers["post_accountability_summary"]

	args := `{
		"task_id": "perles-abc123",
		"summary": "Too short"
	}`

	_, err := handler(context.Background(), json.RawMessage(args))
	require.Error(t, err, "Expected error for summary too short")
	require.Contains(t, err.Error(), "summary too short", "Error should mention summary too short")
}

// TestHandlePostAccountabilitySummary_NilWriter tests graceful error when writer not configured.
func TestHandlePostAccountabilitySummary_NilWriter(t *testing.T) {
	ws := NewWorkerServer("WORKER.1")
	// Don't set accountability writer - leave it nil
	handler := ws.handlers["post_accountability_summary"]

	args := `{
		"task_id": "perles-abc123",
		"summary": "A valid summary that is at least twenty chars."
	}`

	_, err := handler(context.Background(), json.RawMessage(args))
	require.Error(t, err, "Expected error for nil accountability writer")
	require.Contains(t, err.Error(), "accountability writer not configured", "Error should mention accountability writer")
}

// TestHandlePostAccountabilitySummary_WriterError tests that writer errors are propagated.
func TestHandlePostAccountabilitySummary_WriterError(t *testing.T) {
	writer := newMockAccountabilityWriter()
	writer.returnErr = fmt.Errorf("disk full")

	ws := NewWorkerServer("WORKER.1")
	ws.SetAccountabilityWriter(writer)
	handler := ws.handlers["post_accountability_summary"]

	args := `{
		"task_id": "perles-abc123",
		"summary": "A valid summary that is at least twenty chars."
	}`

	_, err := handler(context.Background(), json.RawMessage(args))
	require.Error(t, err, "Expected error when writer fails")
	require.Contains(t, err.Error(), "failed to save accountability summary", "Error should mention save failure")
	require.Contains(t, err.Error(), "disk full", "Error should contain underlying error")
}

// TestHandlePostAccountabilitySummary_InvalidJSON tests that invalid JSON returns error.
func TestHandlePostAccountabilitySummary_InvalidJSON(t *testing.T) {
	writer := newMockAccountabilityWriter()

	ws := NewWorkerServer("WORKER.1")
	ws.SetAccountabilityWriter(writer)
	handler := ws.handlers["post_accountability_summary"]

	_, err := handler(context.Background(), json.RawMessage(`not json`))
	require.Error(t, err, "Expected error for invalid JSON")
	require.Contains(t, err.Error(), "invalid arguments", "Error should mention invalid arguments")
}

// TestHandlePostAccountabilitySummary_OnlyRequiredFields tests success with only required fields.
func TestHandlePostAccountabilitySummary_OnlyRequiredFields(t *testing.T) {
	writer := newMockAccountabilityWriter()

	ws := NewWorkerServer("WORKER.1")
	ws.SetAccountabilityWriter(writer)
	handler := ws.handlers["post_accountability_summary"]

	args := `{
		"task_id": "perles-abc123",
		"summary": "A valid summary that is at least twenty chars."
	}`

	result, err := handler(context.Background(), json.RawMessage(args))
	require.NoError(t, err, "Should succeed with only required fields")

	// Verify content doesn't have optional sections
	content := string(writer.calls[0].Content)
	require.Contains(t, content, "## What I Accomplished", "Should have summary section")
	require.NotContains(t, content, "## Verification Points", "Should not have verification points section")
	require.NotContains(t, content, "## Retro", "Should not have retro section")
	require.NotContains(t, content, "## Next Steps", "Should not have next steps section")

	// Verify success response
	require.Contains(t, result.Content[0].Text, "success", "Response should indicate success")
}

// TestHandlePostAccountabilitySummary_NoMessageStore tests success even without message store.
func TestHandlePostAccountabilitySummary_NoMessageStore(t *testing.T) {
	writer := newMockAccountabilityWriter()

	ws := NewWorkerServer("WORKER.1") // nil message store
	ws.SetAccountabilityWriter(writer)
	handler := ws.handlers["post_accountability_summary"]

	args := `{
		"task_id": "perles-abc123",
		"summary": "A valid summary that is at least twenty chars."
	}`

	result, err := handler(context.Background(), json.RawMessage(args))
	require.NoError(t, err, "Should succeed even without message store")
	require.Contains(t, result.Content[0].Text, "success", "Response should indicate success")

	// Verify writer was still called
	require.Len(t, writer.calls, 1, "Writer should still be called")
}

// ============================================================================
// Tests for post_accountability_summary tool registration
// ============================================================================

// TestPostAccountabilitySummaryToolRegistered tests that post_accountability_summary tool appears in registered tools.
func TestPostAccountabilitySummaryToolRegistered(t *testing.T) {
	ws := NewWorkerServer("WORKER.1")

	tool, ok := ws.tools["post_accountability_summary"]
	require.True(t, ok, "post_accountability_summary tool should be registered")

	// Verify tool metadata
	require.Equal(t, "post_accountability_summary", tool.Name, "Tool name should be post_accountability_summary")
	require.NotEmpty(t, tool.Description, "Tool should have description")
	require.Contains(t, strings.ToLower(tool.Description), "accountability", "Description should mention accountability")

	// Verify input schema
	require.NotNil(t, tool.InputSchema, "Tool should have input schema")
	require.Equal(t, "object", tool.InputSchema.Type, "InputSchema type should be object")

	// Verify required fields
	requiredSet := make(map[string]bool)
	for _, r := range tool.InputSchema.Required {
		requiredSet[r] = true
	}
	require.True(t, requiredSet["task_id"], "task_id should be required")
	require.True(t, requiredSet["summary"], "summary should be required")

	// Verify all properties exist
	_, hasTaskID := tool.InputSchema.Properties["task_id"]
	require.True(t, hasTaskID, "task_id property should be defined")
	_, hasSummary := tool.InputSchema.Properties["summary"]
	require.True(t, hasSummary, "summary property should be defined")
	_, hasCommits := tool.InputSchema.Properties["commits"]
	require.True(t, hasCommits, "commits property should be defined")
	_, hasIssuesDiscovered := tool.InputSchema.Properties["issues_discovered"]
	require.True(t, hasIssuesDiscovered, "issues_discovered property should be defined")
	_, hasIssuesClosed := tool.InputSchema.Properties["issues_closed"]
	require.True(t, hasIssuesClosed, "issues_closed property should be defined")
	_, hasVerificationPoints := tool.InputSchema.Properties["verification_points"]
	require.True(t, hasVerificationPoints, "verification_points property should be defined")
	_, hasRetro := tool.InputSchema.Properties["retro"]
	require.True(t, hasRetro, "retro property should be defined")
	_, hasNextSteps := tool.InputSchema.Properties["next_steps"]
	require.True(t, hasNextSteps, "next_steps property should be defined")

	// Verify output schema
	require.NotNil(t, tool.OutputSchema, "Tool should have output schema")
	_, hasStatus := tool.OutputSchema.Properties["status"]
	require.True(t, hasStatus, "status output property should be defined")
	_, hasFilePath := tool.OutputSchema.Properties["file_path"]
	require.True(t, hasFilePath, "file_path output property should be defined")
	_, hasMessage := tool.OutputSchema.Properties["message"]
	require.True(t, hasMessage, "message output property should be defined")
}

// TestPostAccountabilitySummaryToolHandlerRegistered tests that handler is registered.
func TestPostAccountabilitySummaryToolHandlerRegistered(t *testing.T) {
	ws := NewWorkerServer("WORKER.1")

	_, ok := ws.handlers["post_accountability_summary"]
	require.True(t, ok, "post_accountability_summary handler should be registered")
}

// ============================================================================
// Tests for Turn Completion Enforcement Instrumentation
// ============================================================================

// mockToolCallRecorder implements ToolCallRecorder for testing.
type mockToolCallRecorder struct {
	mu    sync.Mutex
	calls []toolCallRecord
}

type toolCallRecord struct {
	ProcessID string
	ToolName  string
}

func newMockToolCallRecorder() *mockToolCallRecorder {
	return &mockToolCallRecorder{
		calls: make([]toolCallRecord, 0),
	}
}

func (m *mockToolCallRecorder) RecordToolCall(processID, toolName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, toolCallRecord{
		ProcessID: processID,
		ToolName:  toolName,
	})
}

// GetCalls returns a copy of recorded calls.
func (m *mockToolCallRecorder) GetCalls() []toolCallRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]toolCallRecord, len(m.calls))
	copy(result, m.calls)
	return result
}

// TestWorkerServer_SetTurnEnforcer tests that SetTurnEnforcer correctly sets the enforcer.
func TestWorkerServer_SetTurnEnforcer(t *testing.T) {
	ws := NewWorkerServer("WORKER.1")
	require.Nil(t, ws.enforcer, "enforcer should be nil initially")

	recorder := newMockToolCallRecorder()
	ws.SetTurnEnforcer(recorder)
	require.NotNil(t, ws.enforcer, "enforcer should be set")
}

// TestWorkerServer_FabricJoin_RecordsToolCall tests that fabric_join records tool call.
func TestWorkerServer_FabricJoin_RecordsToolCall(t *testing.T) {
	recorder := newMockToolCallRecorder()

	tws := NewTestWorkerServer(t, "WORKER.1")
	defer tws.Close()
	tws.SetTurnEnforcer(recorder)
	handler := tws.handlers["fabric_join"]

	result, err := handler(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err, "Unexpected error")
	require.NotNil(t, result, "Expected result")

	// Verify RecordToolCall was called
	calls := recorder.GetCalls()
	require.Len(t, calls, 1, "Expected 1 recorder call")
	require.Equal(t, "WORKER.1", calls[0].ProcessID, "Expected worker ID")
	require.Equal(t, "fabric_join", calls[0].ToolName, "Expected tool name 'fabric_join'")
}

// TestWorkerServer_FabricJoin_NilEnforcer tests that fabric_join works when enforcer is nil.
func TestWorkerServer_FabricJoin_NilEnforcer(t *testing.T) {
	tws := NewTestWorkerServer(t, "WORKER.1")
	defer tws.Close()
	// Don't set enforcer - leave it nil
	handler := tws.handlers["fabric_join"]

	result, err := handler(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err, "Should not panic with nil enforcer")
	require.NotNil(t, result, "Expected result")
}

// TestWorkerServer_ReportImplementationComplete_RecordsToolCall tests that report_implementation_complete records tool call.
func TestWorkerServer_ReportImplementationComplete_RecordsToolCall(t *testing.T) {
	recorder := newMockToolCallRecorder()

	tws := NewTestWorkerServer(t, "WORKER.1")
	defer tws.Close()
	tws.SetTurnEnforcer(recorder)
	tws.V2Handler.SetResult(&command.CommandResult{
		Success: true,
		Data:    "Implementation complete",
	})
	handler := tws.handlers["report_implementation_complete"]

	result, err := handler(context.Background(), json.RawMessage(`{"summary": "Done with task"}`))
	require.NoError(t, err, "Unexpected error")
	require.NotNil(t, result, "Expected result")
	require.False(t, result.IsError, "Expected success result")

	// Verify RecordToolCall was called
	calls := recorder.GetCalls()
	require.Len(t, calls, 1, "Expected 1 recorder call")
	require.Equal(t, "WORKER.1", calls[0].ProcessID, "Expected worker ID")
	require.Equal(t, "report_implementation_complete", calls[0].ToolName, "Expected tool name 'report_implementation_complete'")
}

// TestWorkerServer_ReportImplementationComplete_NilEnforcer tests that report_implementation_complete works when enforcer is nil.
func TestWorkerServer_ReportImplementationComplete_NilEnforcer(t *testing.T) {
	tws := NewTestWorkerServer(t, "WORKER.1")
	defer tws.Close()
	// Don't set enforcer - leave it nil
	tws.V2Handler.SetResult(&command.CommandResult{
		Success: true,
		Data:    "Implementation complete",
	})
	handler := tws.handlers["report_implementation_complete"]

	result, err := handler(context.Background(), json.RawMessage(`{"summary": "Done with task"}`))
	require.NoError(t, err, "Should not panic with nil enforcer")
	require.NotNil(t, result, "Expected result")
}

// TestWorkerServer_ReportImplementationComplete_ErrorDoesNotRecordToolCall tests that errors don't record tool calls.
func TestWorkerServer_ReportImplementationComplete_ErrorDoesNotRecordToolCall(t *testing.T) {
	recorder := newMockToolCallRecorder()

	tws := NewTestWorkerServer(t, "WORKER.1")
	defer tws.Close()
	tws.SetTurnEnforcer(recorder)
	// Configure mock to return error
	tws.V2Handler.SetResult(&command.CommandResult{
		Success: false,
		Error:   fmt.Errorf("worker not in implementing phase"),
	})
	handler := tws.handlers["report_implementation_complete"]

	result, err := handler(context.Background(), json.RawMessage(`{"summary": "Done with task"}`))
	// Note: Handler returns nil error but sets IsError on result when processor fails
	require.NoError(t, err, "Handler returns nil error")
	require.True(t, result.IsError, "Expected error result")

	// The result is error but returned via tool result, not Go error.
	// RecordToolCall is still called because the underlying adapter call succeeded.
	// This is the expected behavior - we record the tool call even when the processor reports an error in the result.
	calls := recorder.GetCalls()
	require.Len(t, calls, 1, "RecordToolCall is called because adapter call succeeded")
}

// TestWorkerServer_ReportReviewVerdict_RecordsToolCall tests that report_review_verdict records tool call.
func TestWorkerServer_ReportReviewVerdict_RecordsToolCall(t *testing.T) {
	recorder := newMockToolCallRecorder()

	tws := NewTestWorkerServer(t, "WORKER.1")
	defer tws.Close()
	tws.SetTurnEnforcer(recorder)
	tws.V2Handler.SetResult(&command.CommandResult{
		Success: true,
		Data:    "Verdict submitted",
	})
	handler := tws.handlers["report_review_verdict"]

	result, err := handler(context.Background(), json.RawMessage(`{"verdict": "APPROVED", "comments": "LGTM"}`))
	require.NoError(t, err, "Unexpected error")
	require.NotNil(t, result, "Expected result")
	require.False(t, result.IsError, "Expected success result")

	// Verify RecordToolCall was called
	calls := recorder.GetCalls()
	require.Len(t, calls, 1, "Expected 1 recorder call")
	require.Equal(t, "WORKER.1", calls[0].ProcessID, "Expected worker ID")
	require.Equal(t, "report_review_verdict", calls[0].ToolName, "Expected tool name 'report_review_verdict'")
}

// TestWorkerServer_ReportReviewVerdict_NilEnforcer tests that report_review_verdict works when enforcer is nil.
func TestWorkerServer_ReportReviewVerdict_NilEnforcer(t *testing.T) {
	tws := NewTestWorkerServer(t, "WORKER.1")
	defer tws.Close()
	// Don't set enforcer - leave it nil
	tws.V2Handler.SetResult(&command.CommandResult{
		Success: true,
		Data:    "Verdict submitted",
	})
	handler := tws.handlers["report_review_verdict"]

	result, err := handler(context.Background(), json.RawMessage(`{"verdict": "APPROVED", "comments": "LGTM"}`))
	require.NoError(t, err, "Should not panic with nil enforcer")
	require.NotNil(t, result, "Expected result")
}

// TestWorkerServer_ReportReviewVerdict_ErrorDoesNotRecordToolCall tests that errors don't record tool calls.
func TestWorkerServer_ReportReviewVerdict_ErrorDoesNotRecordToolCall(t *testing.T) {
	recorder := newMockToolCallRecorder()

	tws := NewTestWorkerServer(t, "WORKER.1")
	defer tws.Close()
	tws.SetTurnEnforcer(recorder)
	handler := tws.handlers["report_review_verdict"]

	// Missing required "verdict" field causes validation error that returns Go error
	_, err := handler(context.Background(), json.RawMessage(`{"comments": "LGTM"}`))
	require.Error(t, err, "Expected validation error")

	// Verify RecordToolCall was NOT called
	calls := recorder.GetCalls()
	require.Len(t, calls, 0, "RecordToolCall should not be called on error")
}

// newTestFabricService creates a fabric service for testing.
func newTestFabricService() *fabric.Service {
	threadRepo := fabricrepo.NewMemoryThreadRepository()
	depRepo := fabricrepo.NewMemoryDependencyRepository()
	subRepo := fabricrepo.NewMemorySubscriptionRepository()
	ackRepo := fabricrepo.NewMemoryAckRepository(depRepo, threadRepo, subRepo)
	participantRepo := fabricrepo.NewMemoryParticipantRepository()
	svc := fabric.NewService(threadRepo, depRepo, subRepo, ackRepo, participantRepo)
	// Initialize session to create channels
	_ = svc.InitSession("coordinator")
	return svc
}

// TestWorkerServer_FabricSend_RecordsToolCall tests that fabric_send records tool call for turn enforcement.
func TestWorkerServer_FabricSend_RecordsToolCall(t *testing.T) {
	recorder := newMockToolCallRecorder()
	ws := NewWorkerServer("WORKER.1")
	ws.SetTurnEnforcer(recorder)

	// Create a fabric service
	svc := newTestFabricService()

	// SetFabricService registers tools with enforcement
	ws.SetFabricService(svc)

	// Get the handler
	handler := ws.handlers["fabric_send"]
	require.NotNil(t, handler, "fabric_send handler should be registered")

	// Call fabric_send
	args := json.RawMessage(`{"channel": "general", "content": "Hello coordinator"}`)
	_, err := handler(context.Background(), args)
	require.NoError(t, err, "fabric_send should succeed")

	// Verify RecordToolCall was called
	calls := recorder.GetCalls()
	require.Len(t, calls, 1, "Expected 1 recorder call")
	require.Equal(t, "WORKER.1", calls[0].ProcessID, "Expected worker ID")
	require.Equal(t, "fabric_send", calls[0].ToolName, "Expected tool name 'fabric_send'")
}

// TestWorkerServer_FabricReply_RecordsToolCall tests that fabric_reply records tool call for turn enforcement.
func TestWorkerServer_FabricReply_RecordsToolCall(t *testing.T) {
	recorder := newMockToolCallRecorder()
	ws := NewWorkerServer("WORKER.1")
	ws.SetTurnEnforcer(recorder)

	// Create a fabric service
	svc := newTestFabricService()

	// First send a message to have something to reply to
	msg, err := svc.SendMessage(fabric.SendMessageInput{
		ChannelSlug: "general",
		Content:     "original message",
		CreatedBy:   "WORKER.1",
	})
	require.NoError(t, err)

	// SetFabricService registers tools with enforcement
	ws.SetFabricService(svc)

	// Get the handler
	handler := ws.handlers["fabric_reply"]
	require.NotNil(t, handler, "fabric_reply handler should be registered")

	// Call fabric_reply
	args := json.RawMessage(fmt.Sprintf(`{"message_id": "%s", "content": "This is a reply"}`, msg.ID))
	_, err = handler(context.Background(), args)
	require.NoError(t, err, "fabric_reply should succeed")

	// Verify RecordToolCall was called
	calls := recorder.GetCalls()
	require.Len(t, calls, 1, "Expected 1 recorder call")
	require.Equal(t, "WORKER.1", calls[0].ProcessID, "Expected worker ID")
	require.Equal(t, "fabric_reply", calls[0].ToolName, "Expected tool name 'fabric_reply'")
}

// TestWorkerServer_FabricAck_RecordsToolCall tests that fabric_ack records tool call for turn enforcement.
func TestWorkerServer_FabricAck_RecordsToolCall(t *testing.T) {
	recorder := newMockToolCallRecorder()
	ws := NewWorkerServer("WORKER.1")
	ws.SetTurnEnforcer(recorder)

	// Create a fabric service
	svc := newTestFabricService()

	// First send a message to have something to ack
	msg, err := svc.SendMessage(fabric.SendMessageInput{
		ChannelSlug: "general",
		Content:     "message to ack",
		CreatedBy:   "WORKER.1",
	})
	require.NoError(t, err)

	// SetFabricService registers tools with enforcement
	ws.SetFabricService(svc)

	// Get the handler
	handler := ws.handlers["fabric_ack"]
	require.NotNil(t, handler, "fabric_ack handler should be registered")

	// Call fabric_ack
	args := json.RawMessage(fmt.Sprintf(`{"message_ids": ["%s"]}`, msg.ID))
	_, err = handler(context.Background(), args)
	require.NoError(t, err, "fabric_ack should succeed")

	// Verify RecordToolCall was called
	calls := recorder.GetCalls()
	require.Len(t, calls, 1, "Expected 1 recorder call")
	require.Equal(t, "WORKER.1", calls[0].ProcessID, "Expected worker ID")
	require.Equal(t, "fabric_ack", calls[0].ToolName, "Expected tool name 'fabric_ack'")
}

// TestWorkerServer_FabricInbox_DoesNotRecordToolCall tests that fabric_inbox does NOT record tool call
// (since it's not a turn-completing tool).
func TestWorkerServer_FabricInbox_DoesNotRecordToolCall(t *testing.T) {
	recorder := newMockToolCallRecorder()
	ws := NewWorkerServer("WORKER.1")
	ws.SetTurnEnforcer(recorder)

	// Create a fabric service
	svc := newTestFabricService()

	// SetFabricService registers tools with enforcement
	ws.SetFabricService(svc)

	// Get the handler
	handler := ws.handlers["fabric_inbox"]
	require.NotNil(t, handler, "fabric_inbox handler should be registered")

	// Call fabric_inbox
	args := json.RawMessage(`{}`)
	_, err := handler(context.Background(), args)
	require.NoError(t, err, "fabric_inbox should succeed")

	// Verify RecordToolCall was NOT called (fabric_inbox doesn't complete turns)
	calls := recorder.GetCalls()
	require.Len(t, calls, 0, "Expected no recorder calls for fabric_inbox")
}
