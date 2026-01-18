package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/casheiro/yby-cli/plugins/atlas/discovery"
)

func main() {
	var req plugin.PluginRequest

	// 1. Check for Environment Variable Protocol
	if envReq := os.Getenv("YBY_PLUGIN_REQUEST"); envReq != "" {
		if err := json.Unmarshal([]byte(envReq), &req); err != nil {
			fail(fmt.Errorf("falha ao decodificar requisição do env: %w", err))
		}
	} else {
		// 2. Fallback to Stdin
		if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
			// If running without input (manual run?), just print generic error or help
			// But strictly speaking, it expects JSON.
			fail(fmt.Errorf("falha ao decodificar requisição do stdin: %w", err))
		}
	}

	switch req.Hook {
	case "manifest":
		respond(plugin.PluginManifest{
			Name:    "atlas",
			Version: "0.1.0",
			Hooks:   []string{"context", "manifest"},
		})
	case "context":
		// Run discovery
		cwd, err := os.Getwd()
		if err != nil {
			fail(err)
		}

		// TODO: Load config from .yby/atlas.yaml if exists
		ignores := []string{"node_modules", "vendor", ".git", ".idea", ".vscode"}

		blueprint, err := discovery.Scan(cwd, ignores)
		if err != nil {
			fail(err)
		}

		// Return as ContextPatch
		respond(map[string]interface{}{
			"blueprint": blueprint,
		})
	default:
		fail(fmt.Errorf("hook desconhecido: %s", req.Hook))
	}
}

func respond(data interface{}) {
	resp := plugin.PluginResponse{Data: data}
	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		fmt.Fprintf(os.Stderr, "falha ao codificar resposta: %v\n", err)
		os.Exit(1)
	}
}

func fail(err error) {
	resp := plugin.PluginResponse{Error: err.Error()}
	_ = json.NewEncoder(os.Stdout).Encode(resp)
	os.Exit(1)
}
