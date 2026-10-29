package plugin

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	ybyerrors "github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeZipComArquivos cria um zip com os arquivos dados (nome → conteúdo).
func makeZipComArquivos(t *testing.T, arquivos map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for nome, conteudo := range arquivos {
		w, err := zw.Create(nome)
		require.NoError(t, err)
		_, err = w.Write([]byte(conteudo))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// pacoteDoRelease monta o pacote como o GoReleaser publica para este sistema: zip com o
// .exe no Windows, tar.gz nos outros.
func pacoteDoRelease(t *testing.T, plugin, conteudo string) (binario string, pacote []byte, extensao string) {
	t.Helper()
	binario = "yby-plugin-" + plugin
	if runtime.GOOS == "windows" {
		binario += ".exe"
		return binario, makeZipComArquivos(t, map[string]string{binario: conteudo, "README.md": "leia"}), "zip"
	}
	return binario, makeTarGzWithBinary(t, binario, conteudo), "tar.gz"
}

// O binário do go install …@v5.8.4 traz a versão do build info com "v": a tag do release
// fica com um "v" só e o nome do pacote sem, como o GoReleaser publica.
func TestInstallNative_VersaoDoGoInstall(t *testing.T) {
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	binario, pacote, extensao := pacoteDoRelease(t, "atlas", "atlas v5.8.4")
	var caminho string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caminho = r.URL.Path
		_, _ = w.Write(pacote)
	}))
	defer srv.Close()
	releaseBaseURL = srv.URL

	require.NoError(t, NewManager().installNative("atlas", "v5.8.4"))

	esperado := fmt.Sprintf("/v5.8.4/yby-plugin-atlas_5.8.4_%s_%s.%s", runtime.GOOS, runtime.GOARCH, extensao)
	assert.Equal(t, esperado, caminho)
	dados, err := os.ReadFile(filepath.Join(tmp, ".yby", "plugins", binario))
	require.NoError(t, err)
	assert.Equal(t, "atlas v5.8.4", string(dados))
}

// O Version do GoReleaser não tem o "v" (5.8.3): o caminho é o mesmo.
func TestInstallNative_VersaoDoGoReleaser(t *testing.T) {
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	_, pacote, extensao := pacoteDoRelease(t, "viz", "viz")
	var caminho string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caminho = r.URL.Path
		_, _ = w.Write(pacote)
	}))
	defer srv.Close()
	releaseBaseURL = srv.URL

	require.NoError(t, NewManager().installNative("viz", "5.8.3"))
	assert.Equal(t, fmt.Sprintf("/v5.8.3/yby-plugin-viz_5.8.3_%s_%s.%s", runtime.GOOS, runtime.GOARCH, extensao), caminho)
}

// Versão sem release (dev, pseudo-versão, build com alterações) é recusada antes de
// qualquer download, com a dica do --version.
func TestInstallNative_VersaoSemReleaseRecusada(t *testing.T) {
	saveAndRestoreGlobals(t)
	httpGet = func(url string) (*http.Response, error) {
		t.Fatalf("não deveria baixar nada: %s", url)
		return nil, nil
	}
	for _, v := range []string{
		"dev",
		"(devel)",
		"",
		"v5.8.4-0.20261028120000-0123456789ab",
		"v5.8.3-rc.1.0.20261028120000-0123456789ab",
		"v0.0.0-20261028120000-0123456789ab",
		"v5.8.4+dirty",
		"v5.8.4-0.20261028120000-0123456789ab+dirty",
	} {
		t.Run(v, func(t *testing.T) {
			err := NewManager().installNative("atlas", v)
			require.Error(t, err)
			var yerr *ybyerrors.YbyError
			require.ErrorAs(t, err, &yerr)
			assert.Contains(t, yerr.Hint, "--version <tag>")
			assert.Contains(t, yerr.Hint, "releases")
			// a versão pode ter vindo do --version, não do yby; e o exemplo não envelhece
			assert.NotContains(t, yerr.Message, "do yby")
			assert.NotRegexp(t, `--version v\d`, yerr.Hint)
		})
	}
}

func TestVersaoSemRelease(t *testing.T) {
	for _, v := range []string{"5.8.3", "v5.8.4", "v5.9.0-rc.1"} {
		assert.False(t, versaoSemRelease(v), v)
	}
	for _, v := range []string{"dev", "(devel)", "", "v5.8.4-0.20261028120000-0123456789ab", "v5.8.4+dirty", "latest"} {
		assert.True(t, versaoSemRelease(v), v)
	}
}

// O formato sai do conteúdo: o zip do Windows extrai em qualquer sistema, numa pasta
// dentro do pacote também.
func TestExtrairPacote_ZipComPasta(t *testing.T) {
	dest := t.TempDir()
	pacote := makeZipComArquivos(t, map[string]string{"pasta/yby-plugin-atlas.exe": "binário"})

	require.NoError(t, extrairPacote(bytes.NewReader(pacote), "x.tar.gz", dest))
	dados, err := os.ReadFile(filepath.Join(dest, "pasta", "yby-plugin-atlas.exe"))
	require.NoError(t, err)
	assert.Equal(t, "binário", string(dados))
}

func TestExtrairPacote_TarGzComNomeZip(t *testing.T) {
	dest := t.TempDir()
	pacote := makeTarGzWithBinary(t, "yby-plugin-atlas", "binário")

	require.NoError(t, extrairPacote(bytes.NewReader(pacote), "x.zip", dest))
	_, err := os.Stat(filepath.Join(dest, "yby-plugin-atlas"))
	assert.NoError(t, err)
}

func TestExtrairPacote_FormatoDesconhecido(t *testing.T) {
	err := extrairPacote(bytes.NewReader([]byte("não é pacote")), "x.rar", t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "formato de pacote não suportado")
}

func TestExtrairPacote_ZipCorrompido(t *testing.T) {
	err := extrairPacote(bytes.NewReader([]byte("PK\x03\x04lixo")), "x.zip", t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "falha ao extrair plugin")
}

// Zip Slip: entrada que sai da pasta de destino é recusada.
func TestExtractZip_ZipSlip(t *testing.T) {
	base := t.TempDir()
	dest := filepath.Join(base, "dest")
	require.NoError(t, os.MkdirAll(dest, 0755))
	pacote := makeZipComArquivos(t, map[string]string{"../fora.exe": "x"})

	err := extractZip(pacote, dest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entrada inválida no zip")
	_, errStat := os.Stat(filepath.Join(base, "fora.exe"))
	assert.True(t, os.IsNotExist(errStat), "nada pode ser escrito fora do destino")
}
