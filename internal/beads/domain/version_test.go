package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{"equal", "0.30.6", "0.30.6", 0},
		{"a less than b patch", "0.30.5", "0.30.6", -1},
		{"a greater than b patch", "0.30.7", "0.30.6", 1},
		{"a less than b minor not lexicographic", "0.9.0", "0.30.0", -1},
		{"a greater than b major", "1.0.0", "0.99.99", 1},
		{"v prefix on a", "v0.30.6", "0.30.6", 0},
		{"v prefix on b", "0.30.6", "v0.30.6", 0},
		{"v prefix on both", "v0.30.6", "v0.30.6", 0},
		{"a less than b major", "0.30.6", "1.0.0", -1},
		{"missing patch treated as 0", "0.30", "0.30.0", 0},
		{"missing minor and patch treated as 0", "1", "1.0.0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareVersions(tt.a, tt.b)
			require.Equal(t, tt.want, got, "CompareVersions(%q, %q)", tt.a, tt.b)
		})
	}
}

func TestCheckVersion(t *testing.T) {
	tests := []struct {
		name    string
		current string
		wantErr bool
	}{
		{"exact match", MinBeadsVersion, false},
		{"newer patch", "0.62.1", false},
		{"newer minor", "0.63.0", false},
		{"newer major", "1.0.0", false},
		{"older patch", "0.61.9", true},
		{"older minor", "0.61.0", true},
		{"much older", "0.9.0", true},
		{"with v prefix newer", "v1.0.0", false},
		{"with v prefix older", "v0.29.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckVersion(tt.current)
			if tt.wantErr {
				require.Error(t, err, "CheckVersion(%q) should return error", tt.current)
				require.Contains(t, err.Error(), MinBeadsVersion)
				require.Contains(t, err.Error(), tt.current)
			} else {
				require.NoError(t, err, "CheckVersion(%q) should not return error", tt.current)
			}
		})
	}
}
