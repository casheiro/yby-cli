//go:build e2e

package scenarios

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// compileBardBinary compila o binário do plugin bard e retorna o caminho.
func compileBardBinary(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	binaryName := "bard"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(tmpDir, binaryName)

	cmd := exec.Command("go", "build", "-o", binaryPath, "../../plugins/bard")
	cmd.Dir = filepath.Join("..", "..", "test", "e2e", "scenarios")
	// Usar diretório relativo ao módulo raiz
	cmd.Dir = ""
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Compilar a partir da raiz do projeto
	projectRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("Falha ao resolver raiz do projeto: %v", err)
	}
	cmd = exec.Command("go", "build", "-o", binaryPath, "./plugins/bard")
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("Falha ao compilar binário do bard: %v", err)
	}

	return binaryPath
}

// startMockOllamaForBard cria um mock HTTP que simula o Ollama para o Bard.
// Responde a /api/tags (ping) e /api/generate (streaming ou não).
func startMockOllamaForBard(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"models": []map[string]string{{"name": "llama3"}},
			})

		case "/api/generate":
			var req struct {
				Stream bool `json:"stream"`
			}
			json.NewDecoder(r.Body).Decode(&req)

			if req.Stream {
				// Streaming: enviar múltiplos objetos JSON (protocolo Ollama)
				w.Header().Set("Content-Type", "application/x-ndjson")
				flusher, _ := w.(http.Flusher)
				chunks := []string{"Resposta ", "do ", "mock."}
				for _, chunk := range chunks {
					json.NewEncoder(w).Encode(map[string]interface{}{
						"response": chunk,
						"done":     false,
					})
					if flusher != nil {
						flusher.Flush()
					}
				}
				// Mensagem final
				json.NewEncoder(w).Encode(map[string]interface{}{
					"response": "",
					"done":     true,
				})
			} else {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"response": "Resposta do mock.",
				})
			}

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestBardBatch_MultipleQuestions verifica que o Bard em modo batch (non-TTY)
// processa múltiplas perguntas separadas por linha, exibe separador "---",
// e NÃO exibe o prompt "You >".
func TestBardBatch_MultipleQuestions(t *testing.T) {
	bardBinary := compileBardBinary(t)
	mockServer := startMockOllamaForBard(t)
	defer mockServer.Close()

	workDir := t.TempDir()

	// Preparar input com 3 perguntas
	input := "Pergunta um\nPergunta dois\nPergunta três\n"

	pluginRequest, _ := json.Marshal(map[string]interface{}{
		"hook":    "command",
		"context": map[string]interface{}{},
	})

	cmd := exec.Command(bardBinary)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("OLLAMA_HOST=%s", mockServer.URL),
		fmt.Sprintf("YBY_PLUGIN_REQUEST=%s", string(pluginRequest)),
		"YBY_AI_PROVIDER=ollama",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Bard batch falhou (exit code != 0): %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)

	// Não deve conter o prompt interativo
	if strings.Contains(outputStr, "You >") {
		t.Error("Output do modo batch NÃO deve conter o prompt 'You >'")
	}

	// Deve conter separador entre respostas
	if !strings.Contains(outputStr, "---") {
		t.Error("Output do modo batch deve conter separador '---' entre respostas")
	}

	t.Logf("Output do batch:\n%s", outputStr)
}

// TestBardBatch_EmptyLines verifica que linhas vazias são ignoradas no modo batch.
func TestBardBatch_EmptyLines(t *testing.T) {
	bardBinary := compileBardBinary(t)
	mockServer := startMockOllamaForBard(t)
	defer mockServer.Close()

	workDir := t.TempDir()

	// Input com linhas vazias entre perguntas
	input := "\n\nPergunta válida\n\n\nOutra pergunta\n\n"

	pluginRequest, _ := json.Marshal(map[string]interface{}{
		"hook":    "command",
		"context": map[string]interface{}{},
	})

	cmd := exec.Command(bardBinary)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("OLLAMA_HOST=%s", mockServer.URL),
		fmt.Sprintf("YBY_PLUGIN_REQUEST=%s", string(pluginRequest)),
		"YBY_AI_PROVIDER=ollama",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Bard batch com linhas vazias falhou: %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)

	// Deve ter exatamente 1 separador (entre 2 perguntas válidas)
	separatorCount := strings.Count(outputStr, "---")
	if separatorCount != 1 {
		t.Errorf("Esperava 1 separador '---' (2 perguntas), encontrou %d. Output:\n%s", separatorCount, outputStr)
	}
}

// TestBardBatch_ErrorExitCode verifica que o Bard retorna exit code 1
// quando nenhum provider de IA está disponível.
func TestBardBatch_ErrorExitCode(t *testing.T) {
	bardBinary := compileBardBinary(t)

	workDir := t.TempDir()

	pluginRequest, _ := json.Marshal(map[string]interface{}{
		"hook":    "command",
		"context": map[string]interface{}{},
	})

	cmd := exec.Command(bardBinary)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader("pergunta teste\n")
	// Forçar provider explícito (openai) sem API key para garantir falha.
	// Quando YBY_AI_PROVIDER é definido explicitamente, GetProvider opera em modo
	// estrito e retorna nil se o provider não está disponível, sem tentar outros.
	cleanEnv := []string{
		fmt.Sprintf("YBY_PLUGIN_REQUEST=%s", string(pluginRequest)),
		fmt.Sprintf("HOME=%s", workDir),
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
		"TERM=dumb",
		"YBY_AI_PROVIDER=openai",
		"OPENAI_API_KEY=",
	}
	cmd.Env = cleanEnv

	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatal("Esperava exit code != 0 quando nenhum provider está disponível")
	}

	// Verificar que a mensagem de erro menciona provider
	outputStr := string(output)
	if !strings.Contains(outputStr, "provedor") && !strings.Contains(outputStr, "provider") && !strings.Contains(outputStr, "IA") {
		t.Errorf("Esperava mensagem de erro sobre provider de IA, obteve: %s", outputStr)
	}

	t.Logf("Erro esperado obtido: %s", outputStr)
}
