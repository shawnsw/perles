package command

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zjrosen/perles/internal/orchestration/events"
	"github.com/zjrosen/perles/internal/orchestration/validation"
)

// ===========================================================================
// Verdict Tests
// ===========================================================================

func TestVerdict_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		verdict Verdict
		want    bool
	}{
		{"APPROVED is valid", VerdictApproved, true},
		{"DENIED is valid", VerdictDenied, true},
		{"empty is invalid", "", false},
		{"lowercase approved is invalid", "approved", false},
		{"random string is invalid", "MAYBE", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.verdict.IsValid())
		})
	}
}

func TestVerdict_String(t *testing.T) {
	tests := []struct {
		verdict Verdict
		want    string
	}{
		{VerdictApproved, "APPROVED"},
		{VerdictDenied, "DENIED"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			require.Equal(t, tt.want, tt.verdict.String())
		})
	}
}

// ===========================================================================
// ReviewType Tests
// ===========================================================================

func TestReviewType_IsValid(t *testing.T) {
	tests := []struct {
		name       string
		reviewType ReviewType
		want       bool
	}{
		{"simple is valid", ReviewTypeSimple, true},
		{"complex is valid", ReviewTypeComplex, true},
		{"empty is invalid", "", false},
		{"uppercase Simple is invalid", "Simple", false},
		{"uppercase SIMPLE is invalid", "SIMPLE", false},
		{"uppercase Complex is invalid", "Complex", false},
		{"uppercase COMPLEX is invalid", "COMPLEX", false},
		{"random string is invalid", "medium", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.reviewType.IsValid())
		})
	}
}

// ===========================================================================
// AssignTaskCommand Tests
// ===========================================================================

func TestAssignTaskCommand_Validate(t *testing.T) {
	tests := []struct {
		name      string
		workerID  string
		taskID    string
		summary   string
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "valid with summary",
			workerID: "worker-1",
			taskID:   "perles-abc1",
			summary:  "implement feature X",
			wantErr:  false,
		},
		{
			name:     "valid without summary",
			workerID: "worker-1",
			taskID:   "perles-abc1.2",
			summary:  "",
			wantErr:  false,
		},
		{
			name:      "empty worker_id",
			workerID:  "",
			taskID:    "perles-abc1",
			summary:   "",
			wantErr:   true,
			errSubstr: "worker_id is required",
		},
		{
			name:      "empty task_id",
			workerID:  "worker-1",
			taskID:    "",
			summary:   "",
			wantErr:   true,
			errSubstr: "task_id is required",
		},
		{
			name:      "invalid task_id format - no hyphen",
			workerID:  "worker-1",
			taskID:    "invalid",
			summary:   "",
			wantErr:   true,
			errSubstr: "invalid task_id format",
		},
		{
			name:      "invalid task_id format - starts with hyphen",
			workerID:  "worker-1",
			taskID:    "-abc",
			summary:   "",
			wantErr:   true,
			errSubstr: "invalid task_id format",
		},
		{
			name:      "invalid task_id format - ends with hyphen",
			workerID:  "worker-1",
			taskID:    "project-",
			summary:   "",
			wantErr:   true,
			errSubstr: "invalid task_id format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewAssignTaskCommand(SourceMCPTool, tt.workerID, tt.taskID, tt.summary, "")
			err := cmd.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					require.Contains(t, err.Error(), tt.errSubstr)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAssignTaskCommand_Type(t *testing.T) {
	cmd := NewAssignTaskCommand(SourceMCPTool, "worker-1", "perles-abc1", "", "")
	require.Equal(t, CmdAssignTask, cmd.Type())
}

func TestAssignTaskCommand_ImplementsCommand(t *testing.T) {
	var _ Command = &AssignTaskCommand{}
}

// ===========================================================================
// AssignReviewCommand Tests
// ===========================================================================

func TestAssignReviewCommand_Validate(t *testing.T) {
	tests := []struct {
		name          string
		reviewerID    string
		taskID        string
		implementerID string
		wantErr       bool
		errSubstr     string
	}{
		{
			name:          "valid",
			reviewerID:    "worker-2",
			taskID:        "perles-abc1",
			implementerID: "worker-1",
			wantErr:       false,
		},
		{
			name:          "empty reviewer_id",
			reviewerID:    "",
			taskID:        "perles-abc1",
			implementerID: "worker-1",
			wantErr:       true,
			errSubstr:     "reviewer_id is required",
		},
		{
			name:          "empty task_id",
			reviewerID:    "worker-2",
			taskID:        "",
			implementerID: "worker-1",
			wantErr:       true,
			errSubstr:     "task_id is required",
		},
		{
			name:          "empty implementer_id",
			reviewerID:    "worker-2",
			taskID:        "perles-abc1",
			implementerID: "",
			wantErr:       true,
			errSubstr:     "implementer_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewAssignReviewCommand(SourceMCPTool, tt.reviewerID, tt.taskID, tt.implementerID, ReviewTypeComplex)
			err := cmd.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					require.Contains(t, err.Error(), tt.errSubstr)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAssignReviewCommand_Type(t *testing.T) {
	cmd := NewAssignReviewCommand(SourceMCPTool, "worker-2", "perles-abc1", "worker-1", ReviewTypeComplex)
	require.Equal(t, CmdAssignReview, cmd.Type())
}

func TestAssignReviewCommand_ReviewType(t *testing.T) {
	tests := []struct {
		name       string
		reviewType ReviewType
	}{
		{"simple review type", ReviewTypeSimple},
		{"complex review type", ReviewTypeComplex},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewAssignReviewCommand(SourceMCPTool, "worker-2", "perles-abc1", "worker-1", tt.reviewType)
			require.Equal(t, tt.reviewType, cmd.ReviewType)
		})
	}
}

func TestAssignReviewCommand_ImplementsCommand(t *testing.T) {
	var _ Command = &AssignReviewCommand{}
}

// ===========================================================================
// ApproveCommitCommand Tests
// ===========================================================================

func TestApproveCommitCommand_Validate(t *testing.T) {
	tests := []struct {
		name          string
		implementerID string
		taskID        string
		wantErr       bool
		errSubstr     string
	}{
		{
			name:          "valid",
			implementerID: "worker-1",
			taskID:        "perles-abc1",
			wantErr:       false,
		},
		{
			name:          "empty implementer_id",
			implementerID: "",
			taskID:        "perles-abc1",
			wantErr:       true,
			errSubstr:     "implementer_id is required",
		},
		{
			name:          "empty task_id",
			implementerID: "worker-1",
			taskID:        "",
			wantErr:       true,
			errSubstr:     "task_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewApproveCommitCommand(SourceMCPTool, tt.implementerID, tt.taskID)
			err := cmd.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					require.Contains(t, err.Error(), tt.errSubstr)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestApproveCommitCommand_Type(t *testing.T) {
	cmd := NewApproveCommitCommand(SourceMCPTool, "worker-1", "perles-abc1")
	require.Equal(t, CmdApproveCommit, cmd.Type())
}

func TestApproveCommitCommand_ImplementsCommand(t *testing.T) {
	var _ Command = &ApproveCommitCommand{}
}

// ===========================================================================
// AssignReviewFeedbackCommand Tests
// ===========================================================================

func TestAssignReviewFeedbackCommand_Validate(t *testing.T) {
	tests := []struct {
		name          string
		implementerID string
		taskID        string
		feedback      string
		wantErr       bool
	}{
		{
			name:          "valid command",
			implementerID: "worker-1",
			taskID:        "perles-abc1",
			feedback:      "Please fix error handling",
			wantErr:       false,
		},
		{
			name:          "missing implementerID",
			implementerID: "",
			taskID:        "perles-abc1",
			feedback:      "Feedback",
			wantErr:       true,
		},
		{
			name:          "missing taskID",
			implementerID: "worker-1",
			taskID:        "",
			feedback:      "Feedback",
			wantErr:       true,
		},
		{
			name:          "missing feedback",
			implementerID: "worker-1",
			taskID:        "perles-abc1",
			feedback:      "",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewAssignReviewFeedbackCommand(SourceMCPTool, tt.implementerID, tt.taskID, tt.feedback)
			err := cmd.Validate()

			if tt.wantErr {
				require.Error(t, err, "expected validation error")
			} else {
				require.NoError(t, err, "expected no validation error")
			}
		})
	}
}

func TestAssignReviewFeedbackCommand_Type(t *testing.T) {
	cmd := NewAssignReviewFeedbackCommand(SourceMCPTool, "worker-1", "perles-abc1", "Feedback")
	require.Equal(t, CmdAssignReviewFeedback, cmd.Type())
}

func TestAssignReviewFeedbackCommand_ImplementsCommand(t *testing.T) {
	var _ Command = &AssignReviewFeedbackCommand{}
}

// ===========================================================================
// BroadcastCommand Tests
// ===========================================================================

func TestBroadcastCommand_Validate(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		excludeWorkers []string
		wantErr        bool
		errSubstr      string
	}{
		{
			name:           "valid without exclusions",
			content:        "Attention all workers!",
			excludeWorkers: nil,
			wantErr:        false,
		},
		{
			name:           "valid with exclusions",
			content:        "Attention all workers!",
			excludeWorkers: []string{"worker-1", "worker-2"},
			wantErr:        false,
		},
		{
			name:           "empty content",
			content:        "",
			excludeWorkers: nil,
			wantErr:        true,
			errSubstr:      "content is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewBroadcastCommand(SourceMCPTool, tt.content, tt.excludeWorkers)
			err := cmd.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					require.Contains(t, err.Error(), tt.errSubstr)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestBroadcastCommand_Type(t *testing.T) {
	cmd := NewBroadcastCommand(SourceMCPTool, "content", nil)
	require.Equal(t, CmdBroadcast, cmd.Type())
}

func TestBroadcastCommand_ImplementsCommand(t *testing.T) {
	var _ Command = &BroadcastCommand{}
}

// ===========================================================================
// ReportCompleteCommand Tests
// ===========================================================================

func TestReportCompleteCommand_Validate(t *testing.T) {
	tests := []struct {
		name      string
		workerID  string
		summary   string
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "valid with summary",
			workerID: "worker-1",
			summary:  "Implemented feature X",
			wantErr:  false,
		},
		{
			name:     "valid without summary",
			workerID: "worker-1",
			summary:  "",
			wantErr:  false,
		},
		{
			name:      "empty worker_id",
			workerID:  "",
			summary:   "Implemented feature X",
			wantErr:   true,
			errSubstr: "worker_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewReportCompleteCommand(SourceCallback, tt.workerID, tt.summary)
			err := cmd.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					require.Contains(t, err.Error(), tt.errSubstr)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestReportCompleteCommand_Type(t *testing.T) {
	cmd := NewReportCompleteCommand(SourceCallback, "worker-1", "")
	require.Equal(t, CmdReportComplete, cmd.Type())
}

func TestReportCompleteCommand_ImplementsCommand(t *testing.T) {
	var _ Command = &ReportCompleteCommand{}
}

// ===========================================================================
// ReportVerdictCommand Tests
// ===========================================================================

func TestReportVerdictCommand_Validate(t *testing.T) {
	tests := []struct {
		name      string
		workerID  string
		verdict   Verdict
		comments  string
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "valid APPROVED with comments",
			workerID: "worker-2",
			verdict:  VerdictApproved,
			comments: "LGTM!",
			wantErr:  false,
		},
		{
			name:     "valid DENIED with comments",
			workerID: "worker-2",
			verdict:  VerdictDenied,
			comments: "Needs more tests",
			wantErr:  false,
		},
		{
			name:     "valid without comments",
			workerID: "worker-2",
			verdict:  VerdictApproved,
			comments: "",
			wantErr:  false,
		},
		{
			name:      "empty worker_id",
			workerID:  "",
			verdict:   VerdictApproved,
			comments:  "",
			wantErr:   true,
			errSubstr: "worker_id is required",
		},
		{
			name:      "empty verdict",
			workerID:  "worker-2",
			verdict:   "",
			comments:  "",
			wantErr:   true,
			errSubstr: "verdict must be APPROVED or DENIED",
		},
		{
			name:      "invalid verdict",
			workerID:  "worker-2",
			verdict:   "MAYBE",
			comments:  "",
			wantErr:   true,
			errSubstr: "verdict must be APPROVED or DENIED",
		},
		{
			name:      "lowercase approved is invalid",
			workerID:  "worker-2",
			verdict:   "approved",
			comments:  "",
			wantErr:   true,
			errSubstr: "verdict must be APPROVED or DENIED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewReportVerdictCommand(SourceCallback, tt.workerID, tt.verdict, tt.comments)
			err := cmd.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					require.Contains(t, err.Error(), tt.errSubstr)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestReportVerdictCommand_Type(t *testing.T) {
	cmd := NewReportVerdictCommand(SourceCallback, "worker-2", VerdictApproved, "")
	require.Equal(t, CmdReportVerdict, cmd.Type())
}

func TestReportVerdictCommand_ImplementsCommand(t *testing.T) {
	var _ Command = &ReportVerdictCommand{}
}

// ===========================================================================
// TransitionPhaseCommand Tests
// ===========================================================================

func TestTransitionPhaseCommand_Validate(t *testing.T) {
	tests := []struct {
		name      string
		workerID  string
		newPhase  events.ProcessPhase
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "valid PhaseIdle",
			workerID: "worker-1",
			newPhase: events.ProcessPhaseIdle,
			wantErr:  false,
		},
		{
			name:     "valid PhaseImplementing",
			workerID: "worker-1",
			newPhase: events.ProcessPhaseImplementing,
			wantErr:  false,
		},
		{
			name:     "valid PhaseAwaitingReview",
			workerID: "worker-1",
			newPhase: events.ProcessPhaseAwaitingReview,
			wantErr:  false,
		},
		{
			name:     "valid PhaseReviewing",
			workerID: "worker-1",
			newPhase: events.ProcessPhaseReviewing,
			wantErr:  false,
		},
		{
			name:     "valid PhaseAddressingFeedback",
			workerID: "worker-1",
			newPhase: events.ProcessPhaseAddressingFeedback,
			wantErr:  false,
		},
		{
			name:     "valid PhaseCommitting",
			workerID: "worker-1",
			newPhase: events.ProcessPhaseCommitting,
			wantErr:  false,
		},
		{
			name:      "empty worker_id",
			workerID:  "",
			newPhase:  events.ProcessPhaseIdle,
			wantErr:   true,
			errSubstr: "worker_id is required",
		},
		{
			name:      "empty phase",
			workerID:  "worker-1",
			newPhase:  "",
			wantErr:   true,
			errSubstr: "invalid worker phase",
		},
		{
			name:      "invalid phase",
			workerID:  "worker-1",
			newPhase:  "invalid_phase",
			wantErr:   true,
			errSubstr: "invalid worker phase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewTransitionPhaseCommand(SourceInternal, tt.workerID, tt.newPhase)
			err := cmd.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					require.Contains(t, err.Error(), tt.errSubstr)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTransitionPhaseCommand_Type(t *testing.T) {
	cmd := NewTransitionPhaseCommand(SourceInternal, "worker-1", events.ProcessPhaseIdle)
	require.Equal(t, CmdTransitionPhase, cmd.Type())
}

func TestTransitionPhaseCommand_ImplementsCommand(t *testing.T) {
	var _ Command = &TransitionPhaseCommand{}
}

// ===========================================================================
// MarkTaskCompleteCommand Tests
// ===========================================================================

func TestMarkTaskCompleteCommand_Validate(t *testing.T) {
	tests := []struct {
		name      string
		taskID    string
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "valid task ID",
			taskID:  "perles-abc1",
			wantErr: false,
		},
		{
			name:    "valid task ID with subtask",
			taskID:  "perles-abc1.2",
			wantErr: false,
		},
		{
			name:      "empty task_id",
			taskID:    "",
			wantErr:   true,
			errSubstr: "task_id is required",
		},
		{
			name:      "invalid task_id format - no hyphen",
			taskID:    "invalid",
			wantErr:   true,
			errSubstr: "invalid task_id format",
		},
		{
			name:      "invalid task_id format - starts with hyphen",
			taskID:    "-abc",
			wantErr:   true,
			errSubstr: "invalid task_id format",
		},
		{
			name:      "invalid task_id format - ends with hyphen",
			taskID:    "project-",
			wantErr:   true,
			errSubstr: "invalid task_id format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewMarkTaskCompleteCommand(SourceMCPTool, tt.taskID)
			err := cmd.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					require.Contains(t, err.Error(), tt.errSubstr)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMarkTaskCompleteCommand_Type(t *testing.T) {
	cmd := NewMarkTaskCompleteCommand(SourceMCPTool, "perles-abc1")
	require.Equal(t, CmdMarkTaskComplete, cmd.Type())
}

func TestMarkTaskCompleteCommand_ImplementsCommand(t *testing.T) {
	var _ Command = &MarkTaskCompleteCommand{}
}

// ===========================================================================
// MarkTaskFailedCommand Tests
// ===========================================================================

func TestMarkTaskFailedCommand_Validate(t *testing.T) {
	tests := []struct {
		name      string
		taskID    string
		reason    string
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "valid with reason",
			taskID:  "perles-abc1",
			reason:  "Test failure",
			wantErr: false,
		},
		{
			name:    "valid task ID with subtask",
			taskID:  "perles-abc1.2",
			reason:  "Build failed",
			wantErr: false,
		},
		{
			name:      "empty task_id",
			taskID:    "",
			reason:    "Some reason",
			wantErr:   true,
			errSubstr: "task_id is required",
		},
		{
			name:      "empty reason",
			taskID:    "perles-abc1",
			reason:    "",
			wantErr:   true,
			errSubstr: "reason is required",
		},
		{
			name:      "invalid task_id format - no hyphen",
			taskID:    "invalid",
			reason:    "Some reason",
			wantErr:   true,
			errSubstr: "invalid task_id format",
		},
		{
			name:      "invalid task_id format - starts with hyphen",
			taskID:    "-abc",
			reason:    "Some reason",
			wantErr:   true,
			errSubstr: "invalid task_id format",
		},
		{
			name:      "invalid task_id format - ends with hyphen",
			taskID:    "project-",
			reason:    "Some reason",
			wantErr:   true,
			errSubstr: "invalid task_id format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewMarkTaskFailedCommand(SourceMCPTool, tt.taskID, tt.reason)
			err := cmd.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					require.Contains(t, err.Error(), tt.errSubstr)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMarkTaskFailedCommand_Type(t *testing.T) {
	cmd := NewMarkTaskFailedCommand(SourceMCPTool, "perles-abc1", "test failure")
	require.Equal(t, CmdMarkTaskFailed, cmd.Type())
}

func TestMarkTaskFailedCommand_ImplementsCommand(t *testing.T) {
	var _ Command = &MarkTaskFailedCommand{}
}

// ===========================================================================
// isValidTaskID Tests
// ===========================================================================

func TestIsValidTaskID(t *testing.T) {
	tests := []struct {
		taskID string
		want   bool
	}{
		{"pe-perles-abc1", true},
		{"perles-abc1", true},
		{"perles-abc1.2", true},
		{"project-x123", true},
		{"ms-e52", true},
		{"a-bc", true},          // single-char prefix
		{"a-b", true},           // single-char segments
		{"x-abcdefghijk", true}, // single-char prefix, long suffix
		{"p-dolt-m78.4", true},  // single-char prefix with subtask

		// Invalid formats
		{"invalid", false},  // no hyphen
		{"-abc", false},     // starts with hyphen
		{"project-", false}, // ends with hyphen
		{"", false},         // empty
	}

	for _, tt := range tests {
		t.Run(tt.taskID, func(t *testing.T) {
			require.Equal(t, tt.want, validation.IsValidTaskID(tt.taskID))
		})
	}
}

// ===========================================================================
// isValidProcessPhase Tests
// ===========================================================================

func TestIsValidProcessPhase(t *testing.T) {
	tests := []struct {
		phase events.ProcessPhase
		want  bool
	}{
		{events.ProcessPhaseIdle, true},
		{events.ProcessPhaseImplementing, true},
		{events.ProcessPhaseAwaitingReview, true},
		{events.ProcessPhaseReviewing, true},
		{events.ProcessPhaseAddressingFeedback, true},
		{events.ProcessPhaseCommitting, true},
		{"", false},
		{"invalid", false},
		{"working", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			require.Equal(t, tt.want, isValidProcessPhase(tt.phase))
		})
	}
}

// ===========================================================================
// Empty String Validation Tests (comprehensive)
// ===========================================================================

func TestEmptyStringValidation(t *testing.T) {
	// This test verifies that empty string is always caught
	// for required fields across all command types

	testCases := []struct {
		name      string
		cmd       Command
		expectErr bool
	}{
		{"RetireProcess empty ProcessID", NewRetireProcessCommand(SourceMCPTool, "", "reason"), true},
		{"ReplaceProcess empty ProcessID", NewReplaceProcessCommand(SourceMCPTool, "", "reason"), true},
		{"AssignTask empty WorkerID", NewAssignTaskCommand(SourceMCPTool, "", "task-1", "", ""), true},
		{"AssignTask empty TaskID", NewAssignTaskCommand(SourceMCPTool, "worker-1", "", "", ""), true},
		{"AssignReview empty ReviewerID", NewAssignReviewCommand(SourceMCPTool, "", "task-1", "worker-1", ReviewTypeComplex), true},
		{"AssignReview empty TaskID", NewAssignReviewCommand(SourceMCPTool, "worker-2", "", "worker-1", ReviewTypeComplex), true},
		{"AssignReview empty ImplementerID", NewAssignReviewCommand(SourceMCPTool, "worker-2", "task-1", "", ReviewTypeComplex), true},
		{"ApproveCommit empty ImplementerID", NewApproveCommitCommand(SourceMCPTool, "", "task-1"), true},
		{"ApproveCommit empty TaskID", NewApproveCommitCommand(SourceMCPTool, "worker-1", ""), true},
		{"SendToProcess empty ProcessID", NewSendToProcessCommand(SourceMCPTool, "", "content"), true},
		{"SendToProcess empty Content", NewSendToProcessCommand(SourceMCPTool, "worker-1", ""), true},
		{"Broadcast empty Content", NewBroadcastCommand(SourceMCPTool, "", nil), true},
		{"DeliverProcessQueued empty ProcessID", NewDeliverProcessQueuedCommand(SourceInternal, ""), true},
		{"ReportComplete empty WorkerID", NewReportCompleteCommand(SourceCallback, "", "summary"), true},
		{"ReportVerdict empty WorkerID", NewReportVerdictCommand(SourceCallback, "", VerdictApproved, ""), true},
		{"ReportVerdict empty Verdict", NewReportVerdictCommand(SourceCallback, "worker-1", "", ""), true},
		{"TransitionPhase empty WorkerID", NewTransitionPhaseCommand(SourceInternal, "", events.ProcessPhaseIdle), true},
		{"TransitionPhase empty NewPhase", NewTransitionPhaseCommand(SourceInternal, "worker-1", ""), true},
		{"MarkTaskComplete empty TaskID", NewMarkTaskCompleteCommand(SourceMCPTool, ""), true},
		{"MarkTaskFailed empty TaskID", NewMarkTaskFailedCommand(SourceMCPTool, "", "reason"), true},
		{"MarkTaskFailed empty Reason", NewMarkTaskFailedCommand(SourceMCPTool, "perles-abc1", ""), true},
		{"StopProcess empty ProcessID", NewStopProcessCommand(SourceUser, "", false, "reason"), true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cmd.Validate()
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ===========================================================================
// All Commands Implement Command Interface
// ===========================================================================

func TestAllCommandsImplementCommandInterface(t *testing.T) {
	// Verify all command types implement Command interface
	// (unified process commands + workflow commands)
	commands := []Command{
		&SpawnProcessCommand{},
		&RetireProcessCommand{},
		&ReplaceProcessCommand{},
		&SendToProcessCommand{},
		&DeliverProcessQueuedCommand{},
		&ProcessTurnCompleteCommand{},
		&PauseProcessCommand{},
		&ResumeProcessCommand{},
		&AssignTaskCommand{},
		&AssignReviewCommand{},
		&ApproveCommitCommand{},
		&BroadcastCommand{},
		&ReportCompleteCommand{},
		&ReportVerdictCommand{},
		&TransitionPhaseCommand{},
		&MarkTaskCompleteCommand{},
		&MarkTaskFailedCommand{},
		&StopProcessCommand{},
	}

	require.Len(t, commands, 18)
}

// ===========================================================================
// StopProcessCommand Tests
// ===========================================================================

func TestStopProcessCommand_Validate(t *testing.T) {
	tests := []struct {
		name      string
		processID string
		force     bool
		reason    string
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "valid graceful stop with reason",
			processID: "worker-1",
			force:     false,
			reason:    "user_requested",
			wantErr:   false,
		},
		{
			name:      "valid graceful stop without reason",
			processID: "worker-1",
			force:     false,
			reason:    "",
			wantErr:   false,
		},
		{
			name:      "valid force stop with reason",
			processID: "worker-2",
			force:     true,
			reason:    "emergency",
			wantErr:   false,
		},
		{
			name:      "valid force stop without reason",
			processID: "worker-3",
			force:     true,
			reason:    "",
			wantErr:   false,
		},
		{
			name:      "valid coordinator stop",
			processID: "coordinator",
			force:     false,
			reason:    "user_requested",
			wantErr:   false,
		},
		{
			name:      "empty process_id",
			processID: "",
			force:     false,
			reason:    "user_requested",
			wantErr:   true,
			errSubstr: "process_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewStopProcessCommand(SourceUser, tt.processID, tt.force, tt.reason)
			err := cmd.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					require.Contains(t, err.Error(), tt.errSubstr)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNewStopProcessCommand(t *testing.T) {
	tests := []struct {
		name           string
		source         CommandSource
		processID      string
		force          bool
		reason         string
		expectedType   CommandType
		expectedSource CommandSource
	}{
		{
			name:           "graceful stop from user",
			source:         SourceUser,
			processID:      "worker-1",
			force:          false,
			reason:         "user_requested",
			expectedType:   CmdStopProcess,
			expectedSource: SourceUser,
		},
		{
			name:           "force stop from MCP tool",
			source:         SourceMCPTool,
			processID:      "worker-2",
			force:          true,
			reason:         "coordinator_initiated",
			expectedType:   CmdStopProcess,
			expectedSource: SourceMCPTool,
		},
		{
			name:           "internal stop",
			source:         SourceInternal,
			processID:      "worker-3",
			force:          false,
			reason:         "",
			expectedType:   CmdStopProcess,
			expectedSource: SourceInternal,
		},
		{
			name:           "coordinator stop",
			source:         SourceUser,
			processID:      "coordinator",
			force:          false,
			reason:         "context_limit",
			expectedType:   CmdStopProcess,
			expectedSource: SourceUser,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewStopProcessCommand(tt.source, tt.processID, tt.force, tt.reason)

			// Verify fields are set correctly
			require.Equal(t, tt.processID, cmd.ProcessID)
			require.Equal(t, tt.force, cmd.Force)
			require.Equal(t, tt.reason, cmd.Reason)

			// Verify BaseCommand fields
			require.Equal(t, tt.expectedType, cmd.Type())
			require.Equal(t, tt.expectedSource, cmd.Source())
			require.NotEmpty(t, cmd.ID())
			require.False(t, cmd.CreatedAt().IsZero())
		})
	}
}

func TestStopProcessCommand_Type(t *testing.T) {
	cmd := NewStopProcessCommand(SourceUser, "worker-1", false, "")
	require.Equal(t, CmdStopProcess, cmd.Type())
}

func TestStopProcessCommand_ImplementsCommand(t *testing.T) {
	var _ Command = &StopProcessCommand{}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
