package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/casheiro/yby-cli/plugins/atlas/discovery"
)

func main() {
	var req plugin.PluginRequest

	// 1. Verificar protocolo via variável de ambiente
	if envReq := os.Getenv("YBY_PLUGIN_REQUEST"); envReq != "" {
		if err := json.Unmarshal([]byte(envReq), &req); err != nil {
			fail(fmt.Errorf("falha ao decodificar requisição do env: %w", err))
		}
	} else {
		// 2. Fallback para stdin
		if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
			fail(fmt.Errorf("falha ao decodificar requisição do stdin: %w", err))
		}
	}

	switch req.Hook {
	case "manifest":
		respond(plugin.PluginManifest{
			Name:        "atlas",
			Version:     "0.1.0",
			Description: "Mapeamento contínuo de recursos e blueprint do cluster",
			Hooks:       []string{"context", "manifest"},
		})
	case "context":
		// Executar descoberta
		cwd, err := os.Getwd()
		if err != nil {
			fail(err)
		}

		cfg := loadConfig()
		ignores := []string{"node_modules", "vendor", ".git", ".idea", ".vscode"}
		var rules []discovery.Rule
		if cfg != nil {
			if len(cfg.Ignores) > 0 {
				ignores = append(ignores, cfg.Ignores...)
			}
			rules = discovery.MergeRules(cfg.Rules)
		} else {
			rules = discovery.DefaultRules
		}

		blueprint, err := discovery.ScanWithRules(cwd, ignores, rules)
		if err != nil {
			fail(err)
		}

		// Retornar como ContextPatch
		respond(map[string]interface{}{
			"blueprint": blueprint,
		})
	default:
		fail(fmt.Errorf("hook desconhecido: %s", req.Hook))
	}
}

// loadConfig carrega a configuração externa do Atlas a partir de .yby/atlas.yaml.
// Retorna nil se o arquivo não existir ou não puder ser lido.
func loadConfig() *discovery.AtlasConfig {
	configPath := filepath.Join(".yby", "atlas.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil // Arquivo não existe, usar defaults
	}
	var cfg discovery.AtlasConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "aviso: erro ao ler .yby/atlas.yaml: %v\n", err)
		return nil
	}
	return &cfg
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
