// Package validation provides shared validation functions for the orchestration layer.
package validation

import "regexp"

// taskIDPattern validates bd task IDs to prevent command injection.
// Valid formats: "prefix-xxxx" or "prefix-xxxx.N" (for subtasks)
// Examples: "perles-abc1", "perles-abc1.2", "ms-e52", "p-dolt-m78.4"
var taskIDPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)+(\.[0-9]+)*$`)

// IsValidTaskID validates that a task ID matches the expected format.
// Valid formats: "prefix-xxxx" or "prefix-xxxx.N" (for subtasks)
func IsValidTaskID(taskID string) bool {
	return taskIDPattern.MatchString(taskID)
}
