package cmd

import (
	"testing"

	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gerenciadorQueAnotaVersao guarda a versão que o plugin install passou ao Install.
type gerenciadorQueAnotaVersao struct {
	fakePluginManager
	versao string
}

func (g *gerenciadorQueAnotaVersao) Install(_, versao string, _ bool) error {
	g.versao = versao
	return nil
}

// versaoPedidaAoInstall roda o plugin install de atlas (com --version, se dado) e devolve
// a versão que chegou ao Install.
func versaoPedidaAoInstall(t *testing.T, flagVersao string) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	g := &gerenciadorQueAnotaVersao{}
	usarPluginsFalsos(t, &g.fakePluginManager)
	newPluginManager = func() gerenciadorPlugins { return g }
	t.Cleanup(func() { _ = pluginInstallCmd.Flags().Set("version", "") })
	require.NoError(t, pluginInstallCmd.Flags().Set("version", flagVersao))

	saidaDoComando(t, pluginInstallCmd)
	require.NoError(t, pluginInstallCmd.RunE(pluginInstallCmd, []string{"atlas"}))
	return g.versao
}

// O binário do go install …@v5.8.4 não tem os -ldflags: a versão sai do build info.
func TestPluginInstallCmd_VersaoDoBuildInfo(t *testing.T) {
	usarBuildInfo(t, "v5.8.4", nil)
	assert.Equal(t, "v5.8.4", versaoPedidaAoInstall(t, ""))
}

func TestPluginInstallCmd_VersaoDoGoReleaser(t *testing.T) {
	usarBuildInfo(t, "v9.9.9", nil)
	Version = "5.8.3"
	assert.Equal(t, "5.8.3", versaoPedidaAoInstall(t, ""))
}

func TestPluginInstallCmd_FlagVersionVence(t *testing.T) {
	usarBuildInfo(t, "v5.8.4", nil)
	assert.Equal(t, "v5.8.2", versaoPedidaAoInstall(t, "v5.8.2"))
}

// Build sem release (pseudo-versão do go build no repositório): o erro manda informar um
// release com --version.
func TestPluginInstallCmd_PseudoVersaoPedeORelease(t *testing.T) {
	usarBuildInfo(t, "v5.8.4-0.20261028120000-0123456789ab+dirty", nil)
	teardown := mockPluginManagerFactory(t.TempDir())
	defer teardown()

	err := pluginInstallCmd.RunE(pluginInstallCmd, []string{"atlas"})
	var yerr *errors.YbyError
	require.ErrorAs(t, err, &yerr)
	assert.Contains(t, yerr.Hint, "--version")
	assert.Contains(t, err.Error(), "não é de um release")
}
