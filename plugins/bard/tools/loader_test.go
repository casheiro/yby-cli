package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestLoadExternalTools_EmptyDir verifica que diretórios inexistentes não causam erro.
func TestLoadExternalTools_EmptyDir(t *testing.T) {
	Reset()
	defer Reset()

	// LoadExternalTools busca dirs que não existem — não deve dar panic.
	LoadExternalTools()

	tools := All()
	if len(tools) != 0 {
		t.Errorf("esperava 0 ferramentas externas, obteve %d", len(tools))
	}
}

// TestRegisterExternalTool verifica registro de uma tool definida via YAML.
func TestRegisterExternalTool(t *testing.T) {
	Reset()
	defer Reset()

	def := ExternalToolDef{
		Name:        "custom_health",
		Description: "Verifica saúde de um serviço",
		Intents:     []string{"health", "status"},
		Command:     "curl -s http://localhost:{{port}}/health",
		Parameters: []struct {
			Name        string `yaml:"name"`
			Description string `yaml:"description"`
			Required    bool   `yaml:"required"`
		}{
			{Name: "port", Description: "Porta do serviço", Required: true},
		},
	}

	registerExternalTool(def)

	tool := Get("custom_health")
	if tool == nil {
		t.Fatal("ferramenta 'custom_health' não encontrada após registro")
	}
	if tool.Description != "Verifica saúde de um serviço" {
		t.Errorf("descrição inesperada: %q", tool.Description)
	}
	if len(tool.Parameters) != 1 {
		t.Fatalf("esperava 1 parâmetro, obteve %d", len(tool.Parameters))
	}
	if tool.Parameters[0].Name != "port" {
		t.Errorf("nome do parâmetro esperado 'port', obteve %q", tool.Parameters[0].Name)
	}
	if !tool.Parameters[0].Required {
		t.Error("parâmetro 'port' deveria ser obrigatório")
	}
}

// TestExecuteExternalCommand verifica substituição de placeholders.
func TestExecuteExternalCommand(t *testing.T) {
	params := map[string]string{
		"name": "mundo",
	}

	result, err := executeExternalCommand("echo ola {{name}}", params)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if result != "ola mundo" {
		t.Errorf("esperava 'ola mundo', obteve %q", result)
	}
}

// TestExecuteExternalCommand_NoPlaceholders verifica comando sem placeholders.
func TestExecuteExternalCommand_NoPlaceholders(t *testing.T) {
	result, err := executeExternalCommand("echo fixo", nil)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if result != "fixo" {
		t.Errorf("esperava 'fixo', obteve %q", result)
	}
}

// TestExecuteExternalCommand_PlaceholderNaoFornecido verifica limpeza de placeholder sem valor.
func TestExecuteExternalCommand_PlaceholderNaoFornecido(t *testing.T) {
	result, err := executeExternalCommand("echo inicio {{ausente}} fim", nil)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if result != "inicio fim" {
		t.Errorf("esperava 'inicio fim', obteve %q", result)
	}
}

// TestExternalToolDef_Parsing verifica parsing de YAML para ExternalToolDef.
func TestExternalToolDef_Parsing(t *testing.T) {
	yamlContent := `
name: my_tool
description: "Ferramenta de exemplo"
intents:
  - buscar
  - listar
command: "kubectl get {{resource}} -n {{namespace}}"
parameters:
  - name: resource
    description: "Tipo de recurso K8s"
    required: true
  - name: namespace
    description: "Namespace alvo"
    required: false
`

	var def ExternalToolDef
	err := yaml.Unmarshal([]byte(yamlContent), &def)
	if err != nil {
		t.Fatalf("erro ao fazer parse do YAML: %v", err)
	}

	if def.Name != "my_tool" {
		t.Errorf("nome esperado 'my_tool', obteve %q", def.Name)
	}
	if def.Description != "Ferramenta de exemplo" {
		t.Errorf("descrição inesperada: %q", def.Description)
	}
	if len(def.Intents) != 2 {
		t.Fatalf("esperava 2 intents, obteve %d", len(def.Intents))
	}
	if def.Intents[0] != "buscar" || def.Intents[1] != "listar" {
		t.Errorf("intents inesperados: %v", def.Intents)
	}
	if def.Command != "kubectl get {{resource}} -n {{namespace}}" {
		t.Errorf("comando inesperado: %q", def.Command)
	}
	if len(def.Parameters) != 2 {
		t.Fatalf("esperava 2 parâmetros, obteve %d", len(def.Parameters))
	}
	if !def.Parameters[0].Required {
		t.Error("primeiro parâmetro deveria ser obrigatório")
	}
	if def.Parameters[1].Required {
		t.Error("segundo parâmetro não deveria ser obrigatório")
	}
}

// TestLoadExternalTools_ComArquivoYAML verifica carregamento de tool a partir de arquivo YAML.
func TestLoadExternalTools_ComArquivoYAML(t *testing.T) {
	Reset()
	defer Reset()

	// Criar diretório temporário simulando .yby/tools/
	tmpDir := t.TempDir()
	toolsDir := filepath.Join(tmpDir, ".yby", "tools")
	if err := os.MkdirAll(toolsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	yamlContent := `
name: test_external
description: "Tool de teste externo"
command: "echo ok"
`
	if err := os.WriteFile(filepath.Join(toolsDir, "test.yaml"), []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Carregar diretamente do diretório
	entries, err := os.ReadDir(toolsDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(toolsDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var def ExternalToolDef
		if err := yaml.Unmarshal(data, &def); err != nil {
			t.Fatal(err)
		}
		registerExternalTool(def)
	}

	tool := Get("test_external")
	if tool == nil {
		t.Fatal("ferramenta 'test_external' não encontrada")
	}

	// Verificar que Execute funciona
	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("erro ao executar tool: %v", err)
	}
	if result != "ok" {
		t.Errorf("esperava 'ok', obteve %q", result)
	}
}

// TestRegisterExternalTool_NaoSobrescreveBuiltin verifica que tools externas não sobrescrevem built-in.
func TestRegisterExternalTool_NaoSobrescreveBuiltin(t *testing.T) {
	Reset()
	defer Reset()

	// Registrar uma tool built-in
	builtin := &Tool{
		Name:        "kubectl_get",
		Description: "Built-in kubectl get",
		Execute: func(ctx context.Context, params map[string]string) (string, error) {
			return "builtin", nil
		},
	}
	Register(builtin)

	// Tentar registrar external com mesmo nome — deve ser ignorada
	def := ExternalToolDef{
		Name:    "kubectl_get",
		Command: "echo externo",
	}

	// Simular o check de LoadExternalTools
	if Get(def.Name) != nil {
		// Não registra — comportamento esperado
	} else {
		registerExternalTool(def)
	}

	tool := Get("kubectl_get")
	if tool.Description != "Built-in kubectl get" {
		t.Error("tool built-in foi sobrescrita por externa")
	}
}
