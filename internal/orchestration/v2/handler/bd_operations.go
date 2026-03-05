// Package handler provides command handlers for the v2 orchestration architecture.
// This file contains handlers for BD task status commands: MarkTaskComplete and MarkTaskFailed.
// These handlers interact with the BD executor to update task status in the beads database.
package handler

import (
	"context"
	"fmt"

	"github.com/zjrosen/perles/internal/orchestration/events"
	"github.com/zjrosen/perles/internal/orchestration/v2/command"
	"github.com/zjrosen/perles/internal/orchestration/v2/repository"
	taskpkg "github.com/zjrosen/perles/internal/task"
)

// ===========================================================================
// MarkTaskCompleteHandler
// ===========================================================================

// MarkTaskCompleteHandler handles CmdMarkTaskComplete commands.
// It marks a BD task as completed by updating its status to "closed" and adding a completion comment.
// It also deletes the in-memory task assignment and resets associated worker processes
// (implementer and reviewer) back to idle phase so they can accept new tasks.
type MarkTaskCompleteHandler struct {
	bdExecutor  taskpkg.TaskExecutor
	taskRepo    repository.TaskRepository
	processRepo repository.ProcessRepository
}

// MarkTaskCompleteHandlerOption configures MarkTaskCompleteHandler.
type MarkTaskCompleteHandlerOption func(*MarkTaskCompleteHandler)

// WithMarkTaskCompleteProcessRepo sets the process repository for worker state reset.
// When provided, the handler resets implementer/reviewer processes to idle after task completion.
func WithMarkTaskCompleteProcessRepo(processRepo repository.ProcessRepository) MarkTaskCompleteHandlerOption {
	return func(h *MarkTaskCompleteHandler) {
		h.processRepo = processRepo
	}
}

// NewMarkTaskCompleteHandler creates a new MarkTaskCompleteHandler.
// Panics if bdExecutor is nil.
// taskRepo can be nil for backward compatibility (graceful degradation).
// Use WithMarkTaskCompleteProcessRepo to enable worker state reset on task completion.
func NewMarkTaskCompleteHandler(bdExecutor taskpkg.TaskExecutor, taskRepo repository.TaskRepository, opts ...MarkTaskCompleteHandlerOption) *MarkTaskCompleteHandler {
	if bdExecutor == nil {
		panic("bdExecutor is required for MarkTaskCompleteHandler")
	}
	h := &MarkTaskCompleteHandler{
		bdExecutor: bdExecutor,
		taskRepo:   taskRepo,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Handle processes a MarkTaskCompleteCommand.
// It updates the BD task status to "closed", adds a completion comment,
// resets associated worker processes (implementer/reviewer) to idle, and
// deletes the in-memory task assignment.
func (h *MarkTaskCompleteHandler) Handle(ctx context.Context, cmd command.Command) (*command.CommandResult, error) {
	markCmd := cmd.(*command.MarkTaskCompleteCommand)

	// 1. Update task status to closed
	if err := h.bdExecutor.UpdateStatus(markCmd.TaskID, taskpkg.StatusClosed); err != nil {
		return nil, fmt.Errorf("failed to update BD task status: %w", err)
	}

	// 2. Add completion comment
	comment := "Task completed"
	if err := h.bdExecutor.AddComment(markCmd.TaskID, "coordinator", comment); err != nil {
		return nil, fmt.Errorf("failed to add BD comment: %w", err)
	}

	// 3. Reset associated worker processes to idle before deleting the task.
	// This prevents workers from getting stuck in stale phases (e.g., awaiting_review)
	// with a TaskID pointing to a deleted task, which would block future assign_task calls.
	// Queue draining is handled by ProcessTurnCompleteHandler when the worker's next turn completes.
	var resultEvents []any
	if h.taskRepo != nil && h.processRepo != nil {
		task, err := h.taskRepo.Get(markCmd.TaskID)
		if err == nil {
			// Reset implementer to idle
			if task.Implementer != "" {
				resultEvents = append(resultEvents, h.resetProcessToIdle(task.Implementer)...)
			}
			// Reset reviewer to idle (if one was assigned)
			if task.Reviewer != "" {
				resultEvents = append(resultEvents, h.resetProcessToIdle(task.Reviewer)...)
			}
		}
		// If task not found, nothing to reset - proceed gracefully
	}

	// 4. Remove task from in-memory tracking
	// This is best-effort - task may not exist in memory if workflow was restarted
	if h.taskRepo != nil {
		_ = h.taskRepo.Delete(markCmd.TaskID)
	}

	// 5. Return success result with any events from process resets
	result := &MarkTaskCompleteResult{
		TaskID: markCmd.TaskID,
	}

	if len(resultEvents) > 0 {
		return SuccessWithEvents(result, resultEvents...), nil
	}
	return SuccessResult(result), nil
}

// resetProcessToIdle resets a worker process to idle phase, ready status, and clears its TaskID.
// Returns a ProcessStatusChange event so TUI/observers see the transition, or nil if the
// process was not reset (not found, terminal state, or save failure).
func (h *MarkTaskCompleteHandler) resetProcessToIdle(processID string) []any {
	proc, err := h.processRepo.Get(processID)
	if err != nil {
		// Process not found (may have been retired/stopped) - gracefully skip
		return nil
	}

	// Skip if process is in a terminal state
	if proc.Status == repository.StatusRetired || proc.Status == repository.StatusStopped || proc.Status == repository.StatusFailed {
		return nil
	}

	// Skip if process is already idle and ready - no state change needed
	if proc.Status == repository.StatusReady && proc.Phase != nil && *proc.Phase == events.ProcessPhaseIdle && proc.TaskID == "" {
		return nil
	}

	// Reset to idle
	idle := events.ProcessPhaseIdle
	proc.Phase = &idle
	proc.Status = repository.StatusReady
	proc.TaskID = ""

	if err := h.processRepo.Save(proc); err != nil {
		// Save failed - log but don't fail the task completion
		return nil
	}

	// Emit status change event so TUI/observers see the transition
	event := events.NewProcessEvent(events.ProcessStatusChange, proc.ID, proc.Role).
		WithStatus(events.ProcessStatusReady).
		WithPhase(events.ProcessPhaseIdle)
	return []any{event}
}

// MarkTaskCompleteResult contains the result of marking a task as complete.
type MarkTaskCompleteResult struct {
	TaskID string
}

// ===========================================================================
// MarkTaskFailedHandler
// ===========================================================================

// MarkTaskFailedHandler handles CmdMarkTaskFailed commands.
// It adds a failure comment to the BD task with the provided reason.
type MarkTaskFailedHandler struct {
	bdExecutor taskpkg.TaskExecutor
}

// NewMarkTaskFailedHandler creates a new MarkTaskFailedHandler.
// Panics if bdExecutor is nil.
func NewMarkTaskFailedHandler(bdExecutor taskpkg.TaskExecutor) *MarkTaskFailedHandler {
	if bdExecutor == nil {
		panic("bdExecutor is required for MarkTaskFailedHandler")
	}
	return &MarkTaskFailedHandler{
		bdExecutor: bdExecutor,
	}
}

// Handle processes a MarkTaskFailedCommand.
// It adds a failure comment to the BD task with the provided reason.
func (h *MarkTaskFailedHandler) Handle(ctx context.Context, cmd command.Command) (*command.CommandResult, error) {
	markCmd := cmd.(*command.MarkTaskFailedCommand)

	// 1. Add failure comment with reason
	comment := fmt.Sprintf("Task failed: %s", markCmd.Reason)
	if err := h.bdExecutor.AddComment(markCmd.TaskID, "coordinator", comment); err != nil {
		return nil, fmt.Errorf("failed to add BD comment: %w", err)
	}

	// 2. Return success result
	result := &MarkTaskFailedResult{
		TaskID: markCmd.TaskID,
		Reason: markCmd.Reason,
	}

	return SuccessResult(result), nil
}

// MarkTaskFailedResult contains the result of marking a task as failed.
type MarkTaskFailedResult struct {
	TaskID string
	Reason string
}
