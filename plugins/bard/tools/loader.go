package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ExternalToolDef define uma tool customizada carregada de YAML.
type ExternalToolDef struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Intents     []string `yaml:"intents"`
	Command     string   `yaml:"command"`
	Parameters  []struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Required    bool   `yaml:"required"`
	} `yaml:"parameters"`
}

// LoadExternalTools carrega tools de ~/.yby/tools/ e .yby/tools/.
// Tools do projeto (.yby/tools/) têm precedência sobre globais.
func LoadExternalTools() {
	home, _ := os.UserHomeDir()
	dirs := []string{
		filepath.Join(home, ".yby", "tools"),
		filepath.Join(".yby", "tools"),
	}

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml")) {
				continue
			}

			filePath := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			var def ExternalToolDef
			if err := yaml.Unmarshal(data, &def); err != nil {
				fmt.Fprintf(os.Stderr, "aviso: tool %s invalida: %v\n", entry.Name(), err)
				continue
			}

			if def.Name == "" || def.Command == "" {
				continue
			}

			// Não sobrescrever tools built-in
			if Get(def.Name) != nil {
				continue
			}

			registerExternalTool(def)
		}
	}
}

// registerExternalTool converte uma definição YAML em Tool registrada.
func registerExternalTool(def ExternalToolDef) {
	params := make([]ToolParam, len(def.Parameters))
	for i, p := range def.Parameters {
		params[i] = ToolParam{
			Name:        p.Name,
			Description: p.Description,
			Required:    p.Required,
		}
	}

	tool := &Tool{
		Name:        def.Name,
		Description: def.Description,
		Parameters:  params,
		Execute: func(ctx context.Context, toolParams map[string]string) (string, error) {
			return executeExternalCommand(def.Command, toolParams)
		},
	}

	Register(tool)
}

// executeExternalCommand executa um comando shell com substituição de parâmetros.
// Placeholders {{param}} são substituídos pelos valores dos parâmetros.
func executeExternalCommand(cmdTemplate string, params map[string]string) (string, error) {
	cmd := cmdTemplate
	for key, value := range params {
		cmd = strings.ReplaceAll(cmd, "{{"+key+"}}", value)
	}

	// Limpar placeholders não substituídos
	// (ex: {{namespace}} quando namespace não foi fornecido)
	for {
		start := strings.Index(cmd, "{{")
		if start == -1 {
			break
		}
		end := strings.Index(cmd[start:], "}}")
		if end == -1 {
			break
		}
		cmd = cmd[:start] + cmd[start+end+2:]
	}

	cmd = strings.TrimSpace(cmd)

	// Executar via shell
	shellCmd := exec.Command("sh", "-c", cmd)
	output, err := shellCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("comando falhou: %w\n%s", err, string(output))
	}

	return strings.TrimSpace(string(output)), nil
}
