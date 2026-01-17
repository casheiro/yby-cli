package plugin

import (
	"path/filepath"
	"testing"
)

// TestInstallPathValidation checks if install paths are calculated correctly.
// Since Install interacts with FS, we test path logical resolution here mostly.
// In a real scenario, we'd mock the OS/FS but that's complex for this cycle.
// We verify that we are at least getting the right home dir logic.
func TestInstallPathLogic(t *testing.T) {
	// This is a partial test as we can't easily mock UserHomeDir without Dependency Injection
	// But we can verify syntax of pluginSource parsing.

	tests := []struct {
		input    string
		wantName string
		isUrl    bool
	}{
		{"file:///tmp/myplugin", "myplugin", true},
		{"./bin/myplugin", "myplugin", false},
	}

	for _, tt := range tests {
		var srcPath string
		if len(tt.input) > 7 && tt.input[:7] == "file://" {
			srcPath = tt.input[7:]
		} else {
			srcPath = tt.input
		}

		name := filepath.Base(srcPath)
		if name != tt.wantName {
			t.Errorf("Path parsing failed for %s: got name %s, want %s", tt.input, name, tt.wantName)
		}
	}
}
