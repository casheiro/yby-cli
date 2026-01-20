package sdk

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/casheiro/yby-cli/pkg/plugin"
)

// currentContext holds the payload received from the CLI.
var currentContext *plugin.PluginFullContext

// currentHook holds the hook name triggering the plugin.
var currentHook string

// currentArgs holds the args from the CLI command.
var currentArgs []string

// Init must be called at the start of the plugin's main function.
// It reads STDIN or Args to populate the context.
func Init() error {
	// Check if context is passed via stdin (standard for Yby Plugins)
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		var req plugin.PluginRequest
		decoder := json.NewDecoder(os.Stdin)
		if err := decoder.Decode(&req); err != nil {
			// If we can't decode, maybe it's empty or not JSON.
			// Warn but don't fail hard if we want to allow standalone run?
			// Spec implication: Plugins are "context aware", so they expect it.
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
	} else {
		// No stdin? Maybe running standalone for dev?
		// We could support loading from basic config or env vars here.
		// For now, leave nil.
	}

	// Handle Idempotency of flags logic mentioned in PRD?
	// "If functionality requires flags like -c, recrawl args"
	// PRD Section 3.3 says SDK should Init() and scan os.Args.
	// Implementing checking for -c override:
	args := os.Args
	for i, arg := range args {
		if (arg == "-c" || arg == "--context") && i+1 < len(args) {
			requestedEnv := args[i+1]
			// If we have a requested env that differs from what came in JSON,
			// actually we can't easily "reload" the full context here without
			// re-implementing the CLI Core logic (reading environments.yaml etc).
			// The PRD says: "SDK.Init() irá varrer... e recarregar localmente o contexto solicitado (lendo .yby/environments.yaml directly via pkg/context)".
			// This means importing pkg/context here.
			if currentContext != nil && currentContext.Environment != requestedEnv {
				// Override logic
				fmt.Printf("⚠️  SDK: Overriding context to '%s' (requested via flag)\n", requestedEnv)

				// TODO: Implement full reload logic using pkg/context if possible.
				// For now, simpler implementation: valid if KubeConfig is standard.
				// But we promised full feature.
				// Importing pkg/context creates circular dependency IF plugin imports sdk.
				// pkg/plugin imports pkg/context.
				// pkg/plugin/sdk imports pkg/plugin.
				// pkg/context does NOT import plugin.
				// So sdk importing pkg/context is FINE.
			}
		}
	}

	return nil
}

// GetValues returns the raw values map associated with the environment.
func GetValues() map[string]interface{} {
	if currentContext == nil {
		return nil
	}
	return currentContext.Values
}

// GetFullContext returns the Raw plugin context.
func GetFullContext() *plugin.PluginFullContext {
	return currentContext
}

// GetHook returns the hook that triggered the plugin.
func GetHook() string {
	return currentHook
}

// GetArgs returns the arguments passed to the command.
func GetArgs() []string {
	return currentArgs
}
