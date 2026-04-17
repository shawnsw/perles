package board

import (
	"fmt"
	"strconv"
	"strings"
)

// Zone ID format: col:{colIdx}:issue:{issueID}
// This format enables:
// - Unique identification of issues even if they appear in multiple columns
// - Easy parsing to extract column index and issue ID for click handling
// - Extensibility for future zone types (e.g., col:2:header for column headers)

// makeZoneID creates a zone ID for an issue in a specific column.
func makeZoneID(colIdx int, issueID string) string {
	return fmt.Sprintf("col:%d:issue:%s", colIdx, issueID)
}

// makeHeaderActionZoneID creates a zone ID for a column header action button.
func makeHeaderActionZoneID(colIdx int) string {
	return fmt.Sprintf("col:%d:header-action", colIdx)
}

// MakeZoneID is an exported version of makeZoneID for use in tests.
// It creates a zone ID for an issue in a specific column.
func MakeZoneID(colIdx int, issueID string) string {
	return makeZoneID(colIdx, issueID)
}

// parseZoneID extracts the column index and issue ID from a zone ID.
// Returns (colIdx, issueID, true) on success, or (0, "", false) on failure.
// Note: This function is used by tests to verify zone ID format round-trips.
//
//nolint:unused // Used in zone_test.go for round-trip verification
func parseZoneID(zoneID string) (colIdx int, issueID string, ok bool) {
	parts := strings.Split(zoneID, ":")
	if len(parts) != 4 || parts[0] != "col" || parts[2] != "issue" {
		return 0, "", false
	}
	colIdx, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, "", false
	}
	return colIdx, parts[3], true
}
