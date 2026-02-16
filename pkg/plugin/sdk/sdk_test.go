package sdk

import (
	"os"
	"testing"
)

func TestExtractContextFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "Short flag with space",
			args:     []string{"plugin", "-c", "prod"},
			expected: "prod",
		},
		{
			name:     "Long flag with space",
			args:     []string{"plugin", "--context", "staging"},
			expected: "staging",
		},
		{
			name:     "Short flag with equals",
			args:     []string{"plugin", "-c=dev"},
			expected: "dev",
		},
		{
			name:     "Long flag with equals",
			args:     []string{"plugin", "--context=local"},
			expected: "local",
		},
		{
			name:     "No context flag",
			args:     []string{"plugin", "arg1", "arg2"},
			expected: "",
		},
		{
			name:     "Flag at end without value",
			args:     []string{"plugin", "-c"},
			expected: "",
		},
		{
			name:     "Mixed flags",
			args:     []string{"plugin", "--verbose", "-c", "prod", "--debug"},
			expected: "prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractContextFlag(tt.args)
			if result != tt.expected {
				t.Errorf("extractContextFlag() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestInit_NoStdin(t *testing.T) {
	// Save original os.Args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Test with no stdin (normal terminal mode)
	os.Args = []string{"plugin"}

	err := Init()
	if err != nil {
		t.Errorf("Init() should not error when no stdin: %v", err)
	}

	// Context should be nil when no stdin
	if GetFullContext() != nil {
		t.Error("Expected nil context when no stdin provided")
	}
}

func TestGetters(t *testing.T) {
	// Reset global state
	currentContext = nil
	currentHook = ""
	currentArgs = nil

	// Test nil context
	if GetFullContext() != nil {
		t.Error("Expected nil context initially")
	}
	if GetValues() != nil {
		t.Error("Expected nil values initially")
	}
	if GetHook() != "" {
		t.Error("Expected empty hook initially")
	}
	if GetArgs() != nil {
		t.Error("Expected nil args initially")
	}
}
