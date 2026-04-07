package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/pkg/plugin"
)

// helperBuildAtlas compila o binário do atlas em um diretório temporário.
// Retorna o caminho do binário compilado.
func helperBuildAtlas(t *testing.T) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "atlas")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = filepath.Join(projectRoot(t), "plugins", "atlas")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("falha ao compilar atlas: %v\n%s", err, out)
	}
	return binPath
}

// projectRoot retorna o diretório raiz do projeto.
func projectRoot(t *testing.T) string {
	t.Helper()
	// Subir dois níveis a partir de plugins/atlas/
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("falha ao obter diretório de trabalho: %v", err)
	}
	// wd deve ser plugins/atlas já que o teste roda nesse pacote
	return filepath.Join(wd, "..", "..")
}

// TestHookManifest_RetornaJSONValido verifica que o hook "manifest" retorna
// um JSON válido com os campos esperados.
func TestHookManifest_RetornaJSONValido(t *testing.T) {
	binPath := helperBuildAtlas(t)

	req := plugin.PluginRequest{Hook: "manifest"}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("falha ao serializar requisição: %v", err)
	}

	cmd := exec.Command(binPath)
	cmd.Stdin = bytes.NewReader(reqJSON)

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("falha ao executar atlas: %v", err)
	}

	var resp plugin.PluginResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		t.Fatalf("resposta não é JSON válido: %v\nSaída: %s", err, output)
	}

	if resp.Error != "" {
		t.Fatalf("resposta contém erro: %s", resp.Error)
	}

	if resp.Data == nil {
		t.Fatal("resposta não contém dados")
	}

	// Converter Data para JSON e depois para PluginManifest
	dataJSON, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("falha ao re-serializar Data: %v", err)
	}

	var manifest plugin.PluginManifest
	if err := json.Unmarshal(dataJSON, &manifest); err != nil {
		t.Fatalf("Data não é PluginManifest válido: %v", err)
	}

	if manifest.Name != "atlas" {
		t.Errorf("nome esperado 'atlas', obtido %q", manifest.Name)
	}
	if manifest.Version == "" {
		t.Error("versão não deve estar vazia")
	}
	if len(manifest.Hooks) == 0 {
		t.Error("hooks não deve estar vazio")
	}

	// Verificar que 'manifest', 'context' e 'command' estão nos hooks
	hookSet := make(map[string]bool)
	for _, h := range manifest.Hooks {
		hookSet[h] = true
	}
	if !hookSet["manifest"] {
		t.Error("hooks deve conter 'manifest'")
	}
	if !hookSet["context"] {
		t.Error("hooks deve conter 'context'")
	}
	if !hookSet["command"] {
		t.Error("hooks deve conter 'command'")
	}
}

// TestHookManifest_ViaEnvVar verifica que o hook "manifest" funciona
// quando a requisição é passada via variável de ambiente.
func TestHookManifest_ViaEnvVar(t *testing.T) {
	binPath := helperBuildAtlas(t)

	req := plugin.PluginRequest{Hook: "manifest"}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("falha ao serializar requisição: %v", err)
	}

	cmd := exec.Command(binPath)
	cmd.Env = append(os.Environ(), "YBY_PLUGIN_REQUEST="+string(reqJSON))
	// Stdin vazio para não bloquear
	cmd.Stdin = bytes.NewReader(nil)

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("falha ao executar atlas via env var: %v", err)
	}

	var resp plugin.PluginResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		t.Fatalf("resposta não é JSON válido: %v\nSaída: %s", err, output)
	}

	if resp.Error != "" {
		t.Fatalf("resposta contém erro: %s", resp.Error)
	}

	dataJSON, _ := json.Marshal(resp.Data)
	var manifest plugin.PluginManifest
	if err := json.Unmarshal(dataJSON, &manifest); err != nil {
		t.Fatalf("Data não é PluginManifest válido: %v", err)
	}

	if manifest.Name != "atlas" {
		t.Errorf("nome esperado 'atlas', obtido %q", manifest.Name)
	}
}

// TestHookContext_DescobertaDeComponentes verifica que o hook "context"
// descobre componentes em um diretório preparado.
func TestHookContext_DescobertaDeComponentes(t *testing.T) {
	binPath := helperBuildAtlas(t)

	// Preparar diretório com estrutura conhecida
	tmpDir := t.TempDir()
	serviceDir := filepath.Join(tmpDir, "meu-servico")
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		t.Fatalf("falha ao criar diretório: %v", err)
	}
	if err := os.WriteFile(filepath.Join(serviceDir, "go.mod"), []byte("module meu-servico\n\ngo 1.26\n"), 0644); err != nil {
		t.Fatalf("falha ao criar go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(serviceDir, "Dockerfile"), []byte("FROM alpine\n"), 0644); err != nil {
		t.Fatalf("falha ao criar Dockerfile: %v", err)
	}

	req := plugin.PluginRequest{Hook: "context"}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("falha ao serializar requisição: %v", err)
	}

	cmd := exec.Command(binPath)
	cmd.Stdin = bytes.NewReader(reqJSON)
	cmd.Dir = tmpDir // O atlas usa os.Getwd() para a raiz

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("falha ao executar atlas context: %v", err)
	}

	var resp plugin.PluginResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		t.Fatalf("resposta não é JSON válido: %v\nSaída: %s", err, output)
	}

	if resp.Error != "" {
		t.Fatalf("resposta contém erro: %s", resp.Error)
	}

	// Verificar que blueprint está presente
	dataMap, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Data não é map[string]interface{}, tipo: %T", resp.Data)
	}

	blueprint, ok := dataMap["blueprint"]
	if !ok {
		t.Fatal("resposta não contém campo 'blueprint'")
	}

	bpMap, ok := blueprint.(map[string]interface{})
	if !ok {
		t.Fatalf("blueprint não é map[string]interface{}, tipo: %T", blueprint)
	}

	components, ok := bpMap["components"].([]interface{})
	if !ok {
		t.Fatalf("components não é slice, tipo: %T", bpMap["components"])
	}

	if len(components) == 0 {
		t.Error("esperado pelo menos 1 componente descoberto")
	}

	// Verificar que temos pelo menos um componente do tipo "app" (go.mod)
	tiposEncontrados := make(map[string]bool)
	for _, c := range components {
		comp, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if t, ok := comp["type"].(string); ok {
			tiposEncontrados[t] = true
		}
	}

	if !tiposEncontrados["app"] {
		t.Error("esperado componente do tipo 'app' (go.mod detectado)")
	}
	if !tiposEncontrados["infra"] {
		t.Error("esperado componente do tipo 'infra' (Dockerfile detectado)")
	}
}

// TestHookContext_ComConfigExterna verifica que o hook "context" respeita
// a configuração externa .yby/atlas.yaml quando presente.
func TestHookContext_ComConfigExterna(t *testing.T) {
	binPath := helperBuildAtlas(t)

	// Preparar diretório com estrutura conhecida
	tmpDir := t.TempDir()

	// Criar .yby/atlas.yaml com ignores e regras customizadas
	ybyDir := filepath.Join(tmpDir, ".yby")
	if err := os.MkdirAll(ybyDir, 0755); err != nil {
		t.Fatalf("falha ao criar diretório .yby: %v", err)
	}
	configContent := `ignores:
  - ignorar-este
rules:
  - match_file: "custom.marker"
    type: "lib"
`
	if err := os.WriteFile(filepath.Join(ybyDir, "atlas.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("falha ao criar atlas.yaml: %v", err)
	}

	// Criar componente customizado
	customDir := filepath.Join(tmpDir, "minha-lib")
	if err := os.MkdirAll(customDir, 0755); err != nil {
		t.Fatalf("falha ao criar diretório: %v", err)
	}
	if err := os.WriteFile(filepath.Join(customDir, "custom.marker"), []byte(""), 0644); err != nil {
		t.Fatalf("falha ao criar custom.marker: %v", err)
	}

	// Criar componente que deve ser ignorado
	ignoredDir := filepath.Join(tmpDir, "ignorar-este", "service")
	if err := os.MkdirAll(ignoredDir, 0755); err != nil {
		t.Fatalf("falha ao criar diretório ignorado: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ignoredDir, "go.mod"), []byte("module ignorado\n"), 0644); err != nil {
		t.Fatalf("falha ao criar go.mod ignorado: %v", err)
	}

	req := plugin.PluginRequest{Hook: "context"}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("falha ao serializar requisição: %v", err)
	}

	cmd := exec.Command(binPath)
	cmd.Stdin = bytes.NewReader(reqJSON)
	cmd.Dir = tmpDir

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("falha ao executar atlas context: %v", err)
	}

	var resp plugin.PluginResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		t.Fatalf("resposta não é JSON válido: %v\nSaída: %s", err, output)
	}

	if resp.Error != "" {
		t.Fatalf("resposta contém erro: %s", resp.Error)
	}

	// Verificar que blueprint está presente
	dataMap, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Data não é map[string]interface{}, tipo: %T", resp.Data)
	}

	blueprint, ok := dataMap["blueprint"]
	if !ok {
		t.Fatal("resposta não contém campo 'blueprint'")
	}

	bpMap, ok := blueprint.(map[string]interface{})
	if !ok {
		t.Fatalf("blueprint não é map[string]interface{}, tipo: %T", blueprint)
	}

	components, ok := bpMap["components"].([]interface{})
	if !ok {
		t.Fatalf("components não é slice, tipo: %T", bpMap["components"])
	}

	// Verificar que o componente customizado foi detectado
	encontrouCustom := false
	encontrouIgnorado := false
	for _, c := range components {
		comp, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		nome, _ := comp["name"].(string)
		tipo, _ := comp["type"].(string)

		if tipo == "lib" && nome == "minha-lib" {
			encontrouCustom = true
		}
		if nome == "service" {
			encontrouIgnorado = true
		}
	}

	if !encontrouCustom {
		t.Error("esperado componente do tipo 'lib' com nome 'minha-lib' (regra customizada)")
	}
	if encontrouIgnorado {
		t.Error("componente no diretório 'ignorar-este' deveria ter sido ignorado")
	}
}

// TestHookCommand_SubcomandoDiagram verifica que o subcomando "diagram" retorna blueprint.
func TestHookCommand_SubcomandoDiagram(t *testing.T) {
	binPath := helperBuildAtlas(t)

	tmpDir := t.TempDir()

	// Criar chart com dependência pra gerar edges (nós isolados são removidos no overview)
	chartDir := filepath.Join(tmpDir, "charts", "api")
	templatesDir := filepath.Join(chartDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("falha ao criar diretórios: %v", err)
	}
	chartYaml := "apiVersion: v2\nname: api\nversion: 1.0.0\ndependencies:\n  - name: redis\n    version: 17.0.0\n    repository: https://charts.bitnami.com/bitnami\n"
	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(chartYaml), 0644); err != nil {
		t.Fatalf("falha ao criar Chart.yaml: %v", err)
	}
	deployYaml := "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api-server\n"
	if err := os.WriteFile(filepath.Join(templatesDir, "deployment.yaml"), []byte(deployYaml), 0644); err != nil {
		t.Fatalf("falha ao criar deployment.yaml: %v", err)
	}

	// Usar --detail full e --no-ai para teste determinístico
	req := plugin.PluginRequest{Hook: "command", Args: []string{"diagram", "--detail", "full", "--no-ai"}}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("falha ao serializar requisição: %v", err)
	}

	cmd := exec.Command(binPath)
	cmd.Stdin = bytes.NewReader(reqJSON)
	cmd.Dir = tmpDir

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("falha ao executar atlas command diagram: %v", err)
	}

	outStr := string(output)

	if !strings.Contains(outStr, "Diagrama gerado com sucesso") {
		t.Errorf("output deve conter mensagem de sucesso, obtido: %s", outStr)
	}

	if !strings.Contains(outStr, ".yby/atlas-diagram.mmd") {
		t.Error("output deve conter path do arquivo gerado")
	}

	diagramPath := filepath.Join(tmpDir, ".yby", "atlas-diagram.mmd")
	data, err := os.ReadFile(diagramPath)
	if err != nil {
		t.Fatalf("arquivo de diagrama não foi criado: %v", err)
	}

	if !strings.Contains(string(data), "flowchart TD") {
		t.Error("arquivo de diagrama deve conter 'flowchart TD'")
	}

	if !strings.Contains(string(data), "api") {
		t.Error("diagrama deve conter o chart 'api'")
	}
}

// TestHookCommand_SubcomandoDiagramC4 verifica que o formato c4 é passado corretamente.
// TestHookCommand_SubcomandoDiagramSemInfra verifica que diagram sem infra mostra mensagem adequada.
func TestHookCommand_SubcomandoDiagramSemInfra(t *testing.T) {
	binPath := helperBuildAtlas(t)

	tmpDir := t.TempDir()
	// Projeto sem nenhum arquivo de infra
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("falha ao criar README: %v", err)
	}

	req := plugin.PluginRequest{Hook: "command", Args: []string{"diagram"}}
	reqJSON, _ := json.Marshal(req)

	cmd := exec.Command(binPath)
	cmd.Stdin = bytes.NewReader(reqJSON)
	cmd.Dir = tmpDir

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("falha ao executar: %v", err)
	}

	outStr := string(output)
	if !strings.Contains(outStr, "Nenhuma topologia") {
		t.Errorf("deveria informar que nenhuma topologia foi encontrada, obtido: %s", outStr)
	}
}

// TestHookCommand_SemSubcomando verifica que hook command sem args retorna erro.
func TestHookCommand_SemSubcomando(t *testing.T) {
	binPath := helperBuildAtlas(t)

	req := plugin.PluginRequest{Hook: "command", Args: []string{}}
	reqJSON, _ := json.Marshal(req)

	cmd := exec.Command(binPath)
	cmd.Stdin = bytes.NewReader(reqJSON)

	output, _ := cmd.Output()
	outStr := string(output)

	// Agora sem subcomando exibe help em vez de erro
	if !strings.Contains(outStr, "Atlas") || !strings.Contains(outStr, "Subcomandos") {
		t.Errorf("esperado help quando sem subcomando, obtido: %s", outStr)
	}
}

// TestHookCommand_SubcomandoInvalido verifica que subcomando inválido retorna erro.
func TestHookCommand_SubcomandoInvalido(t *testing.T) {
	binPath := helperBuildAtlas(t)

	req := plugin.PluginRequest{Hook: "command", Args: []string{"invalido"}}
	reqJSON, _ := json.Marshal(req)

	cmd := exec.Command(binPath)
	cmd.Stdin = bytes.NewReader(reqJSON)

	output, _ := cmd.Output()

	var resp plugin.PluginResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		t.Fatalf("resposta não é JSON válido: %v", err)
	}

	if resp.Error == "" {
		t.Error("esperado erro para subcomando inválido")
	}
}

// TestHookDesconhecido_RetornaErro verifica que um hook inválido retorna erro.
func TestHookDesconhecido_RetornaErro(t *testing.T) {
	binPath := helperBuildAtlas(t)

	req := plugin.PluginRequest{Hook: "invalido"}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("falha ao serializar requisição: %v", err)
	}

	cmd := exec.Command(binPath)
	cmd.Stdin = bytes.NewReader(reqJSON)

	output, _ := cmd.Output()

	var resp plugin.PluginResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		t.Fatalf("resposta não é JSON válido: %v\nSaída: %s", err, output)
	}

	if resp.Error == "" {
		t.Error("esperado erro para hook desconhecido, mas erro está vazio")
	}
}
