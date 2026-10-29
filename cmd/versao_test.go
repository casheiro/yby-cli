package cmd

import (
	"encoding/json"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// usarBuildInfo simula um binário sem os -ldflags do GoReleaser (go install), com a
// versão do módulo e as configurações do build info dadas. mainVersion vazio simula o
// binário sem build info.
func usarBuildInfo(t *testing.T, mainVersion string, settings map[string]string) {
	t.Helper()
	origLer, origVersao, origCommit, origData := lerBuildInfo, Version, commit, date
	t.Cleanup(func() {
		lerBuildInfo, Version, commit, date = origLer, origVersao, origCommit, origData
	})
	Version, commit, date = "dev", "none", "unknown"
	lerBuildInfo = func() (*debug.BuildInfo, bool) {
		if mainVersion == "" && settings == nil {
			return nil, false
		}
		bi := &debug.BuildInfo{Main: debug.Module{Path: "github.com/casheiro/yby-cli/v5", Version: mainVersion}}
		for k, v := range settings {
			bi.Settings = append(bi.Settings, debug.BuildSetting{Key: k, Value: v})
		}
		return bi, true
	}
}

func TestVersaoInstalada_ReleaseDoGoReleaser(t *testing.T) {
	usarBuildInfo(t, "v9.9.9", map[string]string{"vcs.revision": "ffffffffffffffff"})
	Version, commit, date = "5.8.3", "abc1234def", "2026-10-27T10:00:00Z"

	v := versaoInstalada()
	assert.Equal(t, infoVersao{Versao: "5.8.3", Commit: "abc1234def", Data: "2026-10-27T10:00:00Z"}, v,
		"com os -ldflags, o build info não é consultado")
}

func TestVersaoInstalada_GoInstall(t *testing.T) {
	usarBuildInfo(t, "v5.8.4", nil)

	v := versaoInstalada()
	assert.Equal(t, "v5.8.4", v.Versao)
	assert.Equal(t, "none", v.Commit)
	assert.Equal(t, "unknown", v.Data)
	assert.True(t, v.DoBuildInfo)
}

func TestVersaoInstalada_BuildNoRepositorio(t *testing.T) {
	usarBuildInfo(t, "(devel)", map[string]string{
		"vcs.revision": "0123456789abcdef0123456789abcdef01234567",
		"vcs.time":     "2026-10-28T12:00:00Z",
		"vcs.modified": "true",
	})

	v := versaoInstalada()
	assert.Equal(t, "dev", v.Versao, "(devel) não é versão")
	assert.Equal(t, "0123456", v.Commit, "o commit vai curto")
	assert.Equal(t, "2026-10-28T12:00:00Z", v.Data)
	assert.True(t, v.DoBuildInfo)
}

func TestVersaoInstalada_SemBuildInfo(t *testing.T) {
	usarBuildInfo(t, "", nil)

	assert.Equal(t, infoVersao{Versao: "dev", Commit: "none", Data: "unknown"}, versaoInstalada())
}

func TestVersaoInstalada_VersaoVaziaNoBuildInfo(t *testing.T) {
	usarBuildInfo(t, "", map[string]string{})

	assert.Equal(t, "dev", versaoInstalada().Versao)
}

func TestVersionCmd_GoInstall_MostraAVersaoDoModulo(t *testing.T) {
	_ = rootCmd.PersistentFlags().Set("log-format", "text")
	usarBuildInfo(t, "v5.8.4", nil)

	saida := strings.TrimSpace(saidaDoVersion(t))
	assert.True(t, strings.HasPrefix(saida, "yby version v5.8.4 ["), saida)
	assert.NotContains(t, saida, "dev")
}

func TestVersionCmd_BuildNoRepositorio_MostraCommitEData(t *testing.T) {
	_ = rootCmd.PersistentFlags().Set("log-format", "text")
	usarBuildInfo(t, "v5.8.4-0.20261028120000-0123456789ab+dirty", map[string]string{
		"vcs.revision": "0123456789abcdef",
		"vcs.time":     "2026-10-28T12:00:00Z",
	})

	saida := saidaDoVersion(t)
	assert.Contains(t, saida, "yby version v5.8.4-0.20261028120000-0123456789ab+dirty (0123456, commit de 2026-10-28T12:00:00Z)")
	assert.NotContains(t, saida, "compilado em", "a data do build info é a do commit")
}

func TestVersionCmd_GoInstall_JSON(t *testing.T) {
	require.NoError(t, rootCmd.PersistentFlags().Set("log-format", "json"))
	defer func() { _ = rootCmd.PersistentFlags().Set("log-format", "text") }()
	usarBuildInfo(t, "v5.8.4", map[string]string{"vcs.revision": "0123456789abcdef"})

	var info map[string]string
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(saidaDoVersion(t))), &info))
	assert.Equal(t, "v5.8.4", info["version"])
	assert.Equal(t, "0123456", info["commit"])
	assert.Equal(t, "unknown", info["date"])
}

func TestLinhaVersao_Release(t *testing.T) {
	linha := linhaVersao(infoVersao{Versao: "5.8.3", Commit: "abc1234", Data: "2026-10-27"})
	assert.True(t, strings.HasPrefix(linha, "yby version 5.8.3 (abc1234) compilado em 2026-10-27 ["), linha)
}

func TestLinhaVersao_SoDataDoCommit(t *testing.T) {
	linha := linhaVersao(infoVersao{Versao: "dev", Commit: "none", Data: "2026-10-28", DoBuildInfo: true})
	assert.True(t, strings.HasPrefix(linha, "yby version dev (commit de 2026-10-28) ["), linha)
}

func TestMesmaVersao(t *testing.T) {
	assert.True(t, mesmaVersao("v5.8.3", "5.8.3"), "a tag tem v, o Version do GoReleaser não")
	assert.True(t, mesmaVersao("5.8.3", "v5.8.3"))
	assert.True(t, mesmaVersao("v5.8.3", "v5.8.3"))
	assert.False(t, mesmaVersao("v5.8.4", "5.8.3"))
	assert.False(t, mesmaVersao("", ""))
	assert.False(t, mesmaVersao("v5.8.4", "v5.8.4-0.20261028120000-0123456789ab"))
}
