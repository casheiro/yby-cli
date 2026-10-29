package cmd

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestZip cria um zip com um arquivo, como o pacote do Windows no release.
func createTestZip(t *testing.T, nome string, conteudo []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(nome)
	require.NoError(t, err)
	_, err = w.Write(conteudo)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// releaseFalsoPara serve o release disponivel para sistema/amd64, com o pacote no formato
// e no nome que o GoReleaser publica, e marca em baixou se o pacote foi baixado.
func releaseFalsoPara(t *testing.T, sistema, atual, disponivel string, baixou *bool) {
	t.Helper()
	origGet, origInstalar, origVersao := httpGet, instalarBinario, Version
	origSistema, origArq := sistemaUpgrade, arquiteturaUpgrade
	t.Cleanup(func() {
		httpGet, instalarBinario, Version = origGet, origInstalar, origVersao
		sistemaUpgrade, arquiteturaUpgrade = origSistema, origArq
		for _, f := range []string{"check", "force"} {
			_ = upgradeCmd.Flags().Set(f, "false")
		}
		_ = upgradeCmd.Flags().Set("version", "")
	})
	Version = atual
	sistemaUpgrade, arquiteturaUpgrade = sistema, "amd64"

	asset := nomeDoAsset(disponivel, sistema, "amd64")
	pacote := createTestTarGz(t, "yby", []byte("binário novo"))
	if sistema == "windows" {
		pacote = createTestZip(t, "yby.exe", []byte("binário novo"))
	}
	soma := sha256.Sum256(pacote)
	release := fmt.Sprintf(`{"tag_name":%q,"assets":[{"name":%q,"browser_download_url":"https://exemplo/asset"},{"name":"checksums.txt","browser_download_url":"https://exemplo/checksums"}]}`, disponivel, asset)
	httpGet = func(url string) (*http.Response, error) {
		corpo := []byte(release)
		switch url {
		case "https://exemplo/asset":
			*baixou = true
			corpo = pacote
		case "https://exemplo/checksums":
			corpo = []byte(hex.EncodeToString(soma[:]) + "  " + asset + "\n")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(corpo))}, nil
	}
	instalarBinario = func(binario []byte) (string, error) {
		if string(binario) != "binário novo" {
			return "", fmt.Errorf("binário inesperado: %q", binario)
		}
		return "/usr/local/bin/yby", nil
	}
}

// A tag vem com "v" (v5.8.3) e o Version do GoReleaser sem (5.8.3): é a mesma versão.
func TestUpgradeCmd_EmDia_TagComVEVersionSem(t *testing.T) {
	usarSaidaTexto(t, false)
	baixou := false
	releaseFalsoPara(t, "linux", "5.8.3", "v5.8.3", &baixou)

	b := saidaDoComando(t, upgradeCmd)
	require.NoError(t, upgradeCmd.RunE(upgradeCmd, []string{}))
	assert.Contains(t, b.String(), "já é a mais recente")
	assert.False(t, baixou, "em dia, nada é baixado")
}

// No go install, a versão do build info (v5.8.3) também conta como em dia.
func TestUpgradeCmd_EmDia_VersaoDoBuildInfo(t *testing.T) {
	usarSaidaTexto(t, false)
	baixou := false
	releaseFalsoPara(t, "linux", "dev", "v5.8.3", &baixou)
	usarBuildInfo(t, "v5.8.3", nil)

	b := saidaDoComando(t, upgradeCmd)
	require.NoError(t, upgradeCmd.RunE(upgradeCmd, []string{}))
	assert.Contains(t, b.String(), "v5.8.3")
	assert.NotContains(t, b.String(), "dev")
	assert.Contains(t, b.String(), "já é a mais recente")
	assert.False(t, baixou)
}

func TestUpgradeCmd_DevNuncaEstaEmDia(t *testing.T) {
	usarSaidaTexto(t, false)
	baixou := false
	releaseFalsoPara(t, "linux", "dev", "v5.8.3", &baixou)
	usarBuildInfo(t, "", nil)
	require.NoError(t, upgradeCmd.Flags().Set("check", "true"))

	b := saidaDoComando(t, upgradeCmd)
	require.NoError(t, upgradeCmd.RunE(upgradeCmd, []string{}))
	assert.Contains(t, b.String(), "dev")
	assert.Contains(t, b.String(), "yby upgrade atualiza")
	assert.False(t, baixou, "com --check, nada é baixado")
}

// O pacote do release tem o nome sem o "v" da tag: yby_5.8.4_linux_amd64.tar.gz.
func TestUpgradeCmd_BaixaOPacoteSemV(t *testing.T) {
	usarSaidaTexto(t, false)
	baixou := false
	releaseFalsoPara(t, "linux", "5.8.3", "v5.8.4", &baixou)

	b := saidaDoComando(t, upgradeCmd)
	require.NoError(t, upgradeCmd.RunE(upgradeCmd, []string{}))
	assert.True(t, baixou)
	assert.Contains(t, b.String(), "yby_5.8.4_linux_amd64.tar.gz")
	assert.Contains(t, b.String(), "atualizado para v5.8.4")
}

// No Windows o pacote é zip e o executável dentro dele é yby.exe.
func TestUpgradeCmd_WindowsBaixaOZip(t *testing.T) {
	usarSaidaTexto(t, false)
	baixou := false
	releaseFalsoPara(t, "windows", "5.8.3", "v5.8.4", &baixou)

	b := saidaDoComando(t, upgradeCmd)
	require.NoError(t, upgradeCmd.RunE(upgradeCmd, []string{}))
	assert.True(t, baixou)
	assert.Contains(t, b.String(), "yby_5.8.4_windows_amd64.zip")
	assert.Contains(t, b.String(), "atualizado para v5.8.4")
}

func TestNomeDoAsset(t *testing.T) {
	assert.Equal(t, "yby_5.8.4_linux_amd64.tar.gz", nomeDoAsset("v5.8.4", "linux", "amd64"))
	assert.Equal(t, "yby_5.8.4_darwin_arm64.tar.gz", nomeDoAsset("5.8.4", "darwin", "arm64"))
	assert.Equal(t, "yby_5.8.4_windows_amd64.zip", nomeDoAsset("v5.8.4", "windows", "amd64"))
}

// Os testes do upgrade (nomes de pacote e goldens) são do yby padrão, também quando o
// pacote cmd é testado com as tags de nuvem; TestUpgrade_VarianteCloud troca para o
// yby-cloud.
func init() { pacoteDoUpgrade = "yby" }

// O yby-cloud baixa o pacote dele, não o do yby padrão, que viria sem os SDKs de nuvem.
func TestUpgrade_VarianteCloud(t *testing.T) {
	anterior := pacoteDoUpgrade
	pacoteDoUpgrade = "yby-cloud"
	t.Cleanup(func() { pacoteDoUpgrade = anterior })

	assert.Equal(t, "yby-cloud_5.8.4_linux_amd64.tar.gz", nomeDoAsset("v5.8.4", "linux", "amd64"))
	assert.Equal(t, "yby-cloud_5.8.4_windows_arm64.zip", nomeDoAsset("v5.8.4", "windows", "arm64"))

	dados, err := extrairBinario(createTestZip(t, "yby-cloud.exe", []byte("exe")), "windows")
	require.NoError(t, err)
	assert.Equal(t, "exe", string(dados))

	dados, err = extrairBinario(createTestTarGz(t, "yby-cloud", []byte("elf")), "linux")
	require.NoError(t, err)
	assert.Equal(t, "elf", string(dados))

	_, err = extrairBinario(createTestTarGz(t, "yby", []byte("elf")), "linux")
	assert.ErrorContains(t, err, "yby-cloud", "o pacote do yby padrão não serve ao yby-cloud")
}

func TestExtrairBinario_ZipDoWindows(t *testing.T) {
	pacote := createTestZip(t, "pasta/yby.exe", []byte("exe"))
	dados, err := extrairBinario(pacote, "windows")
	require.NoError(t, err)
	assert.Equal(t, "exe", string(dados))

	_, err = extrairBinario(createTestZip(t, "outro.exe", []byte("x")), "windows")
	assert.ErrorContains(t, err, "yby.exe")

	_, err = extrairBinario([]byte("não é zip"), "windows")
	assert.ErrorContains(t, err, "falha ao abrir zip")
}

func TestExtrairBinario_TarGzForaDoWindows(t *testing.T) {
	dados, err := extrairBinario(createTestTarGz(t, "yby", []byte("elf")), "linux")
	require.NoError(t, err)
	assert.Equal(t, "elf", string(dados))
}

// ========================================================
// trocarExecutavel: backup, rollback e o .bak no Windows
// ========================================================

// salvarTroca restaura as variáveis que trocarExecutavel usa e liga as reais.
func salvarTroca(t *testing.T) {
	t.Helper()
	origRemove, origRename, origEscrever, origDepois, origSistema := osRemove, osRename, osWriteFile, removerDepois, sistemaUpgrade
	t.Cleanup(func() {
		osRemove, osRename, osWriteFile, removerDepois, sistemaUpgrade = origRemove, origRename, origEscrever, origDepois, origSistema
	})
	osRemove, osRename, osWriteFile = os.Remove, os.Rename, os.WriteFile
	removerDepois = func(caminho string) error {
		t.Fatalf("não deveria agendar a remoção de %s", caminho)
		return nil
	}
}

func executavelFalso(t *testing.T) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "yby.exe")
	require.NoError(t, os.WriteFile(exe, []byte("binário velho"), 0o755))
	return exe
}

func TestTrocarExecutavel_Sucesso(t *testing.T) {
	salvarTroca(t)
	exe := executavelFalso(t)

	require.NoError(t, trocarExecutavel(exe, []byte("binário novo")))
	dados, err := os.ReadFile(exe)
	require.NoError(t, err)
	assert.Equal(t, "binário novo", string(dados))
	_, err = os.Stat(exe + ".bak")
	assert.True(t, os.IsNotExist(err), "o backup é apagado no fim")
}

// O .bak de um upgrade anterior (no Windows, quando a remoção agendada não rodou) sai
// antes do novo backup.
func TestTrocarExecutavel_ApagaBackupAntigo(t *testing.T) {
	salvarTroca(t)
	exe := executavelFalso(t)
	require.NoError(t, os.WriteFile(exe+".bak", []byte("backup antigo"), 0o755))

	var removidos []string
	osRemove = func(nome string) error { removidos = append(removidos, nome); return os.Remove(nome) }
	var renomeou bool
	osRename = func(de, para string) error {
		assert.NotEmpty(t, removidos, "o backup antigo sai antes do rename")
		renomeou = true
		return os.Rename(de, para)
	}

	require.NoError(t, trocarExecutavel(exe, []byte("binário novo")))
	assert.True(t, renomeou)
	require.NotEmpty(t, removidos)
	assert.Equal(t, exe+".bak", removidos[0])
	_, err := os.Stat(exe + ".bak")
	assert.True(t, os.IsNotExist(err))
}

// No Windows, o .bak é o executável em uso: a remoção falha e vai para depois que o yby
// fechar.
func TestTrocarExecutavel_BackupEmUsoVaiParaDepois(t *testing.T) {
	salvarTroca(t)
	exe := executavelFalso(t)

	chamadas := 0
	osRemove = func(nome string) error {
		chamadas++
		if chamadas == 1 {
			return &fs.PathError{Op: "remove", Path: nome, Err: fs.ErrNotExist}
		}
		return &fs.PathError{Op: "remove", Path: nome, Err: fs.ErrPermission}
	}
	var agendado string
	removerDepois = func(caminho string) error { agendado = caminho; return nil }

	require.NoError(t, trocarExecutavel(exe, []byte("binário novo")))
	assert.Equal(t, exe+".bak", agendado)
	dados, err := os.ReadFile(exe)
	require.NoError(t, err)
	assert.Equal(t, "binário novo", string(dados))
}

// Agendar também pode falhar: o binário novo já está no lugar, o upgrade não falha.
func TestTrocarExecutavel_AgendamentoFalhaNaoDerrubaOUpgrade(t *testing.T) {
	salvarTroca(t)
	exe := executavelFalso(t)
	osRemove = func(nome string) error { return &fs.PathError{Op: "remove", Path: nome, Err: fs.ErrPermission} }
	removerDepois = func(string) error { return fmt.Errorf("cmd.exe não encontrado") }

	assert.NoError(t, trocarExecutavel(exe, []byte("binário novo")))
}

func TestTrocarExecutavel_FalhaNaEscritaRestaura(t *testing.T) {
	salvarTroca(t)
	exe := executavelFalso(t)
	osWriteFile = func(string, []byte, os.FileMode) error { return fmt.Errorf("disco cheio") }

	err := trocarExecutavel(exe, []byte("binário novo"))
	var yerr *errors.YbyError
	require.ErrorAs(t, err, &yerr)
	assert.Contains(t, yerr.Hint, "restaurado")
	dados, errLer := os.ReadFile(exe)
	require.NoError(t, errLer)
	assert.Equal(t, "binário velho", string(dados), "o rollback devolve o binário anterior")
}

func TestTrocarExecutavel_RenomearSemPermissao(t *testing.T) {
	for _, c := range []struct{ sistema, dica string }{
		{"windows", "administrador"},
		{"linux", "sudo"},
	} {
		t.Run(c.sistema, func(t *testing.T) {
			salvarTroca(t)
			sistemaUpgrade = c.sistema
			osRename = func(de, para string) error {
				return &os.LinkError{Op: "rename", Old: de, New: para, Err: fs.ErrPermission}
			}
			err := trocarExecutavel(filepath.Join(t.TempDir(), "yby"), []byte("novo"))
			var yerr *errors.YbyError
			require.ErrorAs(t, err, &yerr)
			assert.Contains(t, yerr.Hint, c.dica)
		})
	}
}

// --version 5.8.3 busca a tag v5.8.3, como o yby plugin install: a tag do GitHub tem o "v"
// e a versão do GoReleaser não.
func TestFetchRelease_VersaoSemV(t *testing.T) {
	origGet := httpGet
	t.Cleanup(func() { httpGet = origGet })
	for _, c := range []struct{ versao, tag string }{
		{"5.8.3", "v5.8.3"},
		{"v5.8.3", "v5.8.3"},
		{" 5.9.0-rc.1", "v5.9.0-rc.1"},
		{"latest", "latest"},
	} {
		t.Run(c.versao, func(t *testing.T) {
			var pedida string
			httpGet = func(url string) (*http.Response, error) {
				pedida = url
				return mockHTTPResponse(200, `{"tag_name":"`+c.tag+`"}`), nil
			}
			release, err := fetchRelease(c.versao)
			require.NoError(t, err)
			assert.Equal(t, "https://api.github.com/repos/casheiro/yby-cli/releases/tags/"+c.tag, pedida)
			assert.Equal(t, c.tag, release.TagName)
		})
	}
}
