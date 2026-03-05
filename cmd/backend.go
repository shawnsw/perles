package cmd

import (
	"fmt"

	beadsadapter "github.com/zjrosen/perles/internal/beads/adapter"
	"github.com/zjrosen/perles/internal/config"
	"github.com/zjrosen/perles/internal/task"
)

// newBackend creates the appropriate backend based on configuration.
// The config.Backend field selects the backend type; it defaults to "beads"
// when empty. Future backends (linear, github, etc.) will be added here.
func newBackend(cfg *config.Config, workDir string) (task.Backend, error) {
	backendType := cfg.Backend
	if backendType == "" {
		backendType = "beads"
	}

	switch backendType {
	case "beads":
		return beadsadapter.NewBeadsBackend(cfg.ResolvedBeadsDir, workDir)
	default:
		return nil, fmt.Errorf("unknown backend: %q (available: beads)", backendType)
	}
}
