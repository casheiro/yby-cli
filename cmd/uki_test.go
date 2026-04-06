package cmd

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAIProvider implementa ai.Provider para testes.
// Completion retorna o blueprint serializado como JSON para compatibilidade
// com generateGovernanceViaCompletion.
type mockAIProvider struct {
	name      string
	blueprint *ai.GovernanceBlueprint
	err       error
}

func (m *mockAIProvider) Name() string                       { return m.name }
func (m *mockAIProvider) IsAvailable(_ context.Context) bool { return true }
func (m *mockAIProvider) Completion(_ context.Context, _, _ string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if m.blueprint != nil {
		data, _ := json.Marshal(m.blueprint)
		return string(data), nil
	}
	return "", nil
}
func (m *mockAIProvider) StreamCompletion(_ context.Context, _, _ string, _ io.Writer) error {
	return nil
}
func (m *mockAIProvider) EmbedDocuments(_ context.Context, _ []string) ([][]float32, error) {
	return nil, nil
}
func (m *mockAIProvider) GenerateGovernance(_ context.Context, _ string) (*ai.GovernanceBlueprint, error) {
	return m.blueprint, m.err
}

func TestCaptureCmd_NoDescription(t *testing.T) {
	err := captureCmd.RunE(captureCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "informe uma descrição ou use --file")
}

func TestCaptureCmd_FileFlag(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	// Cria arquivo de entrada
	inputFile := filepath.Join(dir, "input.txt")
	os.WriteFile(inputFile, []byte("conteúdo do arquivo para captura"), 0644)

	original := getAIProvider
	defer func() { getAIProvider = original }()

	getAIProvider = func(_ context.Context, _ string) ai.Provider {
		return &mockAIProvider{
			name: "Mock",
			blueprint: &ai.GovernanceBlueprint{
				Files: []ai.GeneratedFile{
					{Path: ".synapstor/.uki/from-file.md", Content: "# Do Arquivo"},
				},
			},
		}
	}

	// Seta a flag --file
	captureCmd.Flags().Set("file", inputFile)
	defer captureCmd.Flags().Set("file", "")

	err := captureCmd.RunE(captureCmd, []string{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, ".synapstor", ".uki", "from-file.md"))
	require.NoError(t, err)
	assert.Equal(t, "# Do Arquivo", string(data))
}

func TestCaptureCmd_FileFlag_ArquivoInexistente(t *testing.T) {
	captureCmd.Flags().Set("file", "/caminho/inexistente/arquivo.txt")
	defer captureCmd.Flags().Set("file", "")

	err := captureCmd.RunE(captureCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "falha ao ler arquivo")
}

func TestCaptureCmd_NoProvider(t *testing.T) {
	original := getAIProvider
	defer func() { getAIProvider = original }()

	getAIProvider = func(_ context.Context, _ string) ai.Provider {
		return nil
	}

	err := captureCmd.RunE(captureCmd, []string{"teste descrição"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Nenhum provedor de IA")
}

func TestCaptureCmd_Success(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	original := getAIProvider
	defer func() { getAIProvider = original }()

	getAIProvider = func(_ context.Context, _ string) ai.Provider {
		return &mockAIProvider{
			name: "Mock",
			blueprint: &ai.GovernanceBlueprint{
				Files: []ai.GeneratedFile{
					{Path: ".synapstor/.uki/test.md", Content: "# Teste"},
				},
			},
		}
	}

	err := captureCmd.RunE(captureCmd, []string{"teste de governança"})
	require.NoError(t, err)

	// Verifica que o arquivo foi criado
	data, err := os.ReadFile(filepath.Join(dir, ".synapstor", ".uki", "test.md"))
	require.NoError(t, err)
	assert.Equal(t, "# Teste", string(data))
}

func TestCaptureCmd_AIError(t *testing.T) {
	original := getAIProvider
	defer func() { getAIProvider = original }()

	getAIProvider = func(_ context.Context, _ string) ai.Provider {
		return &mockAIProvider{
			name: "Mock",
			err:  assert.AnError,
		}
	}

	err := captureCmd.RunE(captureCmd, []string{"teste"})
	assert.Error(t, err)
}

func TestCaptureCmd_UnsafePath(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	original := getAIProvider
	defer func() { getAIProvider = original }()

	getAIProvider = func(_ context.Context, _ string) ai.Provider {
		return &mockAIProvider{
			name: "Mock",
			blueprint: &ai.GovernanceBlueprint{
				Files: []ai.GeneratedFile{
					{Path: "../../../etc/passwd", Content: "hacked"},
					{Path: "/absolute/path", Content: "hacked"},
					{Path: ".synapstor/.uki/safe.md", Content: "ok"},
				},
			},
		}
	}

	err := captureCmd.RunE(captureCmd, []string{"teste segurança"})
	require.NoError(t, err)

	// O arquivo seguro deve existir
	_, err = os.Stat(filepath.Join(dir, ".synapstor", ".uki", "safe.md"))
	assert.NoError(t, err)

	// Os arquivos inseguros NÃO devem existir dentro do diretório de trabalho
	entries, _ := filepath.Glob(filepath.Join(dir, "etc", "*"))
	assert.Empty(t, entries, "nenhum arquivo deve ser criado no caminho 'etc/'")

	entries2, _ := filepath.Glob(filepath.Join(dir, "absolute", "*"))
	assert.Empty(t, entries2, "nenhum arquivo deve ser criado no caminho 'absolute/'")
}
