package sdk

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/casheiro/yby-cli/pkg/plugin"
)

var (
	currentContext *plugin.PluginFullContext
	currentHook    string
	currentArgs    []string
)

// Init initializes the plugin SDK by reading the PluginRequest from stdin or environment.
// It also handles context overrides via command-line flags (-c/--context).
func Init() error {
	// Check if context is passed via stdin (standard for Yby Plugins)
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		var req plugin.PluginRequest
		decoder := json.NewDecoder(os.Stdin)
		if err := decoder.Decode(&req); err != nil {
			return fmt.Errorf("failed to decode plugin request from stdin: %w", err)
		}

		currentHook = req.Hook
		currentArgs = req.Args

		// Parse Context map back into PluginFullContext struct
		if req.Context != nil {
			ctxBytes, err := json.Marshal(req.Context)
			if err != nil {
				return fmt.Errorf("failed to marshal context map: %w", err)
			}
			var fullCtx plugin.PluginFullContext
			if err := json.Unmarshal(ctxBytes, &fullCtx); err != nil {
				return fmt.Errorf("failed to unmarshal into PluginFullContext: %w", err)
			}
			currentContext = &fullCtx
		}
	}

	// Handle context override via flags
	// Supports: -c prod, --context prod, -c=prod, --context=prod
	requestedEnv := extractContextFlag(os.Args)
	if requestedEnv != "" && currentContext != nil && currentContext.Environment != requestedEnv {
		// Context override detected
		// Note: Full reload would require importing pkg/context which could create
		// circular dependencies. For now, we log a warning.
		// A full implementation would reload the context from .yby/environments.yaml
		fmt.Fprintf(os.Stderr, "⚠️  SDK: Context override detected (-c %s) but full reload not implemented. Using context from CLI.\n", requestedEnv)
	}

	return nil
}

// extractContextFlag extracts the context value from command-line arguments.
// Supports: -c value, --context value, -c=value, --context=value
func extractContextFlag(args []string) string {
	for i, arg := range args {
		// Handle -c=value or --context=value
		if strings.HasPrefix(arg, "-c=") {
			return strings.TrimPrefix(arg, "-c=")
		}
		if strings.HasPrefix(arg, "--context=") {
			return strings.TrimPrefix(arg, "--context=")
		}

		// Handle -c value or --context value
		if (arg == "-c" || arg == "--context") && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// GetFullContext returns the full plugin context
func GetFullContext() *plugin.PluginFullContext {
	return currentContext
}

// GetValues returns the parsed values from the context
func GetValues() map[string]interface{} {
	if currentContext == nil {
		return nil
	}
	return currentContext.Values
}

// GetHook returns the current hook being executed
func GetHook() string {
	return currentHook
}

// GetArgs returns the arguments passed to the plugin
func GetArgs() []string {
	return currentArgs
}
