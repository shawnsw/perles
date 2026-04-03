package task

import "fmt"

// ServerDownError indicates the backend's server is not reachable.
// This is used by backends that require an external server process
// (e.g., Dolt in server mode).
type ServerDownError struct {
	Host string
	Port int
}

func (e *ServerDownError) Error() string {
	return fmt.Sprintf("backend server unreachable at %s:%d", e.Host, e.Port)
}

// EmbeddedModeError indicates the backend is running in embedded mode,
// which uses an exclusive file lock and cannot be accessed by perles concurrently.
type EmbeddedModeError struct {
	BeadsDir string
}

func (e *EmbeddedModeError) Error() string {
	return fmt.Sprintf("beads is using embedded dolt mode in %s (requires server mode for concurrent access)", e.BeadsDir)
}

// VersionIncompatibleError indicates the backend data store version is too old
// for this version of perles.
type VersionIncompatibleError struct {
	Current  string // version found in the data store (or "unknown")
	Required string // minimum version required by this backend
}

func (e *VersionIncompatibleError) Error() string {
	return fmt.Sprintf("backend version %s is too old (requires %s)", e.Current, e.Required)
}
