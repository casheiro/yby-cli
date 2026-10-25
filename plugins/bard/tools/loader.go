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
	// rotulo é como o diretório aparece no /ferramentas do Bard.
	dirs := []struct{ dir, rotulo string }{
		{filepath.Join(home, ".yby", "tools"), "~/.yby/tools/"},
		{filepath.Join(".yby", "tools"), ".yby/tools/"},
	}

	mu.Lock()
	invalidas = nil
	mu.Unlock()

	for _, d := range dirs {
		entries, err := os.ReadDir(d.dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml")) {
				continue
			}

			filePath := filepath.Join(d.dir, entry.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			arquivo := d.rotulo + entry.Name()

			var def ExternalToolDef
			if err := yaml.Unmarshal(data, &def); err != nil {
				fmt.Fprintf(os.Stderr, "aviso: tool %s invalida: %v\n", entry.Name(), err)
				registrarInvalida(arquivo, "o YAML não é válido")
				continue
			}

			if motivo := camposFaltando(def); motivo != "" {
				registrarInvalida(arquivo, motivo)
				continue
			}

			// Não sobrescrever tools built-in
			if t := Get(def.Name); t != nil {
				if !t.Externa {
					registrarInvalida(arquivo, "o nome "+def.Name+" já é de uma ferramenta nativa")
				}
				continue
			}

			registerExternalTool(def)
		}
	}
}

// camposFaltando diz quais campos obrigatórios faltam na definição ("falta o campo
// command"); vazio se nenhum.
func camposFaltando(def ExternalToolDef) string {
	switch {
	case def.Name == "" && def.Command == "":
		return "faltam os campos name e command"
	case def.Name == "":
		return "falta o campo name"
	case def.Command == "":
		return "falta o campo command"
	}
	return ""
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
		Externa: true,
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
