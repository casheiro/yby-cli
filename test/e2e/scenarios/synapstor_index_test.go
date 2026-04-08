//go:build e2e

package scenarios

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// compileSynapstor compila o binário do plugin Synapstor em um diretório temporário.
// O Synapstor tem seu próprio go.mod, então compilamos a partir do diretório do plugin.
func compileSynapstor(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	binary := filepath.Join(tmpDir, "synapstor")
	projectRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err, "falha ao resolver raiz do projeto")

	synapstorDir := filepath.Join(projectRoot, "plugins", "synapstor")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = synapstorDir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "falha ao compilar synapstor: %s", string(out))
	return binary
}

// runSynapstorHook executa o synapstor com um hook específico via stdin (protocolo SDK) e retorna a resposta.
func runSynapstorHook(t *testing.T, binary, workDir, hook string, args []string) plugin.PluginResponse {
	t.Helper()
	req := plugin.PluginRequest{Hook: hook, Args: args}
	reqJSON, err := json.Marshal(req)
	require.NoError(t, err)

	cmd := exec.Command(binary)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(string(reqJSON))
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "falha ao executar synapstor: %s", string(output))

	// Synapstor pode emitir múltiplas linhas (stderr + stdout); pegar a primeira linha JSON válida
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var resp plugin.PluginResponse
	for _, line := range lines {
		if err := json.Unmarshal([]byte(line), &resp); err == nil {
			return resp
		}
	}
	t.Fatalf("nenhuma linha JSON válida no output do synapstor: %s", string(output))
	return resp
}

func TestSynapstor_HookContext(t *testing.T) {
	binary := compileSynapstor(t)
	workDir := t.TempDir()

	// Criar manifest pré-populado
	manifestDir := filepath.Join(workDir, ".synapstor")
	require.NoError(t, os.MkdirAll(manifestDir, 0755))

	now := time.Now().UTC().Truncate(time.Second)
	manifest := map[string]interface{}{
		"files": map[string]interface{}{
			"doc1.md": map[string]interface{}{
				"sha256":     "abc123",
				"indexed_at": now.Format(time.RFC3339),
			},
			"doc2.md": map[string]interface{}{
				"sha256":     "def456",
				"indexed_at": now.Add(-time.Hour).Format(time.RFC3339),
			},
		},
	}
	manifestJSON, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(manifestDir, ".index_manifest.json"),
		manifestJSON, 0644,
	))

	resp := runSynapstorHook(t, binary, workDir, "context", nil)
	require.Empty(t, resp.Error, "synapstor retornou erro: %s", resp.Error)

	// A resposta do synapstor tem um duplo wrapping: respond() wrapa em PluginResponse{Data: ...}
	// e handleContextHook passa um PluginResponse como Data. O resultado real pode ser:
	// {"data": {"data": {...}}} ou {"data": {...}} dependendo da implementação.
	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "Data deveria ser mapa, obteve: %T", resp.Data)

	// Verificar se o data contém os campos diretamente ou dentro de outro "data"
	contextData := data
	if inner, ok := data["data"].(map[string]interface{}); ok {
		contextData = inner
	}

	assert.Contains(t, contextData, "synapstor_indexed_files",
		"resposta deveria conter synapstor_indexed_files")
	assert.Contains(t, contextData, "synapstor_last_indexed",
		"resposta deveria conter synapstor_last_indexed")
	assert.Contains(t, contextData, "synapstor_status",
		"resposta deveria conter synapstor_status")

	// Verificar valores
	status, _ := contextData["synapstor_status"].(string)
	assert.Equal(t, "active", status, "status deveria ser 'active' com manifest presente")

	// synapstor_indexed_files pode ser float64 (JSON number) ou int
	indexedFiles := contextData["synapstor_indexed_files"]
	switch v := indexedFiles.(type) {
	case float64:
		assert.Equal(t, float64(2), v, "deveria ter 2 arquivos indexados")
	case int:
		assert.Equal(t, 2, v, "deveria ter 2 arquivos indexados")
	default:
		t.Errorf("synapstor_indexed_files tem tipo inesperado: %T", indexedFiles)
	}
}

func TestSynapstor_HookContext_NotIndexed(t *testing.T) {
	binary := compileSynapstor(t)
	workDir := t.TempDir()

	// Sem manifest — deveria retornar status "not_indexed"
	resp := runSynapstorHook(t, binary, workDir, "context", nil)
	require.Empty(t, resp.Error, "synapstor retornou erro: %s", resp.Error)

	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "Data deveria ser mapa")

	contextData := data
	if inner, ok := data["data"].(map[string]interface{}); ok {
		contextData = inner
	}

	status, _ := contextData["synapstor_status"].(string)
	assert.Equal(t, "not_indexed", status,
		"status deveria ser 'not_indexed' sem manifest")

	indexedFiles := contextData["synapstor_indexed_files"]
	switch v := indexedFiles.(type) {
	case float64:
		assert.Equal(t, float64(0), v, "deveria ter 0 arquivos indexados")
	case int:
		assert.Equal(t, 0, v, "deveria ter 0 arquivos indexados")
	}
}

func TestSynapstor_IndexReport(t *testing.T) {
	binary := compileSynapstor(t)
	workDir := t.TempDir()

	// Criar arquivos UKI para indexação
	ukiDir := filepath.Join(workDir, ".synapstor", ".uki")
	require.NoError(t, os.MkdirAll(ukiDir, 0755))

	require.NoError(t, os.WriteFile(
		filepath.Join(ukiDir, "doc1.md"),
		[]byte("# Documento 1\n\nConteúdo do primeiro documento sobre deploy.\n"),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(ukiDir, "doc2.md"),
		[]byte("# Documento 2\n\nConteúdo do segundo documento sobre kubernetes.\n"),
		0644,
	))

	// Executar index via protocolo SDK (stdin com hook "command")
	req := plugin.PluginRequest{Hook: "command", Args: []string{"index"}}
	reqJSON, err := json.Marshal(req)
	require.NoError(t, err)

	cmd := exec.Command(binary)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(string(reqJSON))
	// Sem provider de IA configurado, esperamos erro
	cmd.Env = append(os.Environ(),
		"GEMINI_API_KEY=",
		"OPENAI_API_KEY=",
		"OLLAMA_HOST=",
	)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// O index precisa de AI provider para embeddings.
	// Sem provider, deve indicar o erro claramente.
	if err != nil {
		assert.True(t,
			strings.Contains(outputStr, "provedor") ||
				strings.Contains(outputStr, "provider") ||
				strings.Contains(outputStr, "configurado") ||
				strings.Contains(outputStr, "IA"),
			"erro deveria indicar falta de provider de IA, obteve: %s", outputStr)
		return
	}

	// Se por algum motivo tiver provider (ex: Ollama local), verificar métricas
	assert.True(t,
		strings.Contains(outputStr, "Arquivos escaneados") ||
			strings.Contains(outputStr, "escaneados"),
		"output deveria conter métricas de arquivos escaneados: %s", outputStr)
	assert.True(t,
		strings.Contains(outputStr, "Chunks gerados") ||
			strings.Contains(outputStr, "chunks"),
		"output deveria conter métricas de chunks: %s", outputStr)
}

