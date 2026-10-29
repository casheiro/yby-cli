package plugin

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	ybyerrors "github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// execucoes registra os binários que o executor tentou rodar, com o processo auxiliar no
// lugar deles (o yby-plugin-manifesto devolve um manifesto).
type execucoes struct {
	mu       sync.Mutex
	binarios []string
}

func (e *execucoes) lista() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.binarios...)
}

// registrarExecucoes troca o execCommandContext por um que anota cada binário antes de
// rodar o processo auxiliar.
func registrarExecucoes(t *testing.T) *execucoes {
	t.Helper()
	reg := &execucoes{}
	original := execCommandContext
	execCommandContext = func(ctx context.Context, nome string, args ...string) *exec.Cmd {
		reg.mu.Lock()
		reg.binarios = append(reg.binarios, nome)
		reg.mu.Unlock()
		return mockExecCommandContext(ctx, nome, args...)
	}
	t.Cleanup(func() { execCommandContext = original })
	return reg
}

// projetoClonado cria uma home vazia e um projeto com .yby/plugins/<arquivo>, como um
// repositório clonado, entra no projeto e devolve o caminho do binário.
func projetoClonado(t *testing.T, arquivo string) string {
	t.Helper()
	home, projeto := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	pasta := filepath.Join(projeto, ".yby", "plugins")
	require.NoError(t, os.MkdirAll(pasta, 0o755))
	binario := filepath.Join(pasta, arquivo)
	require.NoError(t, os.WriteFile(binario, []byte("binário de outra pessoa"), 0o755))
	t.Chdir(projeto)
	return binario
}

func TestNomeDoArquivo(t *testing.T) {
	casos := map[string]string{
		"yby-plugin-sentinel":                  "sentinel",
		"yby-plugin-sentinel.exe":              "sentinel",
		"yby-plugin-sentinel.EXE":              "sentinel",
		filepath.Join("a", "yby-plugin-x.exe"): "x",
		"outro":                                "outro",
		"yby-plugin-":                          "",
	}
	for arquivo, esperado := range casos {
		assert.Equal(t, esperado, nomeDoArquivo(arquivo), arquivo)
	}
}

func TestDiscover_ComConfiancaNaoExecutaBinarioNaoConfiavel(t *testing.T) {
	projetoClonado(t, "yby-plugin-intruso.exe")
	reg := registrarExecucoes(t)

	m := NewManager()
	m.EnableTrustCheck()
	require.NoError(t, m.Discover())

	assert.Empty(t, reg.lista(), "a descoberta não pode rodar o manifest de binário não confiável")
	plugins := m.ListPlugins()
	require.Len(t, plugins, 1)
	assert.Equal(t, "intruso", plugins[0].Name, "o nome sai do arquivo")
	assert.True(t, plugins[0].NaoConfiavel)
	assert.Equal(t, []string{"command"}, plugins[0].Hooks)
	assert.Contains(t, plugins[0].Description, "não confiável")
	assert.Contains(t, plugins[0].Description, "yby plugin trust intruso")
	_, achado := m.GetPlugin("intruso")
	assert.True(t, achado, "o yby plugin trust procura pelo nome derivado")

	// Nenhum outro caminho roda o binário: assets, context e o command (que devolve o erro).
	assert.Empty(t, m.GetAssets())
	assert.NoError(t, m.ExecuteContextHook(context.Background(), &scaffold.BlueprintContext{}))
	err := m.ExecuteCommandHook("intruso", nil)
	require.Error(t, err)
	var yerr *ybyerrors.YbyError
	require.True(t, errors.As(err, &yerr))
	assert.Equal(t, "confira a origem do binário e rode yby plugin trust intruso", yerr.Hint)
	assert.Empty(t, reg.lista(), "nenhum hook pode rodar o binário não confiável")
}

func TestDiscover_ComConfiancaExecutaManifestDoConfiavel(t *testing.T) {
	binario := projetoClonado(t, "yby-plugin-manifesto.exe")
	require.NoError(t, TrustPlugin(binario))
	reg := registrarExecucoes(t)

	m := NewManager()
	m.EnableTrustCheck()
	require.NoError(t, m.Discover())

	assert.Equal(t, []string{binario}, reg.lista())
	p, achado := m.GetPlugin("manifesto")
	require.True(t, achado)
	assert.False(t, p.Manifest.NaoConfiavel)
	assert.Equal(t, "1.0.0", p.Manifest.Version)

	// O binário mudou depois do trust: volta a ser não confiável e não roda.
	require.NoError(t, os.WriteFile(binario, []byte("binário trocado"), 0o755))
	reg2 := registrarExecucoes(t)
	m2 := NewManager()
	m2.EnableTrustCheck()
	require.NoError(t, m2.Discover())
	assert.Empty(t, reg2.lista())
	p, achado = m2.GetPlugin("manifesto")
	require.True(t, achado)
	assert.True(t, p.Manifest.NaoConfiavel)
}

func TestDiscover_SemConfiancaRodaOManifestComoAntes(t *testing.T) {
	binario := projetoClonado(t, "yby-plugin-manifesto.exe")
	reg := registrarExecucoes(t)

	m := NewManager()
	require.NoError(t, m.Discover())

	assert.Equal(t, []string{binario}, reg.lista())
	p, achado := m.GetPlugin("manifesto")
	require.True(t, achado)
	assert.False(t, p.Manifest.NaoConfiavel)
}

func TestCheckTrust_DicaComONomeQueOTrustAceita(t *testing.T) {
	binario := projetoClonado(t, "yby-plugin-sentinel.exe")
	reg := registrarExecucoes(t)

	e := &Executor{Timeout: 3 * time.Second}
	_, err := e.Run(context.Background(), binario, PluginRequest{Hook: "manifest"})
	require.Error(t, err)
	var yerr *ybyerrors.YbyError
	require.True(t, errors.As(err, &yerr))
	assert.Equal(t, ybyerrors.ErrCodePlugin, yerr.Code)
	assert.Contains(t, yerr.Message, "o plugin sentinel não é confiável")
	assert.Equal(t, "confira a origem do binário e rode yby plugin trust sentinel", yerr.Hint)
	assert.NotContains(t, yerr.Hint, "yby-plugin-")
	assert.Equal(t, binario, yerr.Context["binario"])
	assert.Empty(t, reg.lista())

	err = e.RunInteractive(context.Background(), binario, PluginRequest{Hook: "command"})
	require.True(t, errors.As(err, &yerr))
	assert.Equal(t, "confira a origem do binário e rode yby plugin trust sentinel", yerr.Hint)
	assert.Empty(t, reg.lista())
}
