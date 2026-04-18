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
	var req plugin.PluginRequest
	var parsed bool

	// 1. Tentar ler da variável de ambiente YBY_PLUGIN_REQUEST (modo interativo/RunInteractive)
	if envReq := os.Getenv("YBY_PLUGIN_REQUEST"); envReq != "" {
		if err := json.Unmarshal([]byte(envReq), &req); err != nil {
			return fmt.Errorf("failed to decode plugin request from env: %w", err)
		}
		parsed = true
	}

	// 2. Fallback: ler do stdin (modo não-interativo, quando stdin não é TTY)
	if !parsed {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			decoder := json.NewDecoder(os.Stdin)
			if err := decoder.Decode(&req); err != nil {
				return fmt.Errorf("failed to decode plugin request from stdin: %w", err)
			}
			parsed = true
		}
	}

	if parsed {
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
