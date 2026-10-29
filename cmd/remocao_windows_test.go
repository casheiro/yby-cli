//go:build windows

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A remoção agendada roda de verdade: um cmd.exe sem janela apaga o arquivo em poucos
// segundos. As pastas têm o que o cmd trataria como sintaxe se o caminho estivesse na
// linha: espaço, &, parênteses, ^, ! e um %PATH% que ele expandiria.
func TestAgendarRemocao_ApagaOArquivo(t *testing.T) {
	for _, pasta := range []string{"pasta com espaço & e", "Program Files (x86)", "a%PATH%b", "c^d!e", "pasta (1) & x"} {
		t.Run(pasta, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(t.TempDir(), pasta)
			require.NoError(t, os.MkdirAll(dir, 0o755))
			alvo := filepath.Join(dir, ".yby.exe.removido-123")
			require.NoError(t, os.WriteFile(alvo, []byte("binário"), 0o755))

			require.NoError(t, agendarRemocao(alvo))

			prazo := time.Now().Add(20 * time.Second)
			for time.Now().Before(prazo) {
				if _, err := os.Stat(alvo); os.IsNotExist(err) {
					return
				}
				time.Sleep(200 * time.Millisecond)
			}
			t.Fatalf("%s não foi apagado", alvo)
		})
	}
}

// O caminho não entra na linha do cmd.exe: vai pelo ambiente e a linha só cita a variável.
func TestLinhaRemocao_CaminhoPeloAmbiente(t *testing.T) {
	linha := linhaRemocao(`C:\Windows\system32\cmd.exe`)
	assert.True(t, strings.HasPrefix(linha, `"C:\Windows\system32\cmd.exe" /d /v:off /s /c "`), linha)
	assert.Contains(t, linha, `del /f /q "%YBY_REMOVER%"`)
	assert.Contains(t, linha, `if not exist "%YBY_REMOVER%" exit`)
	assert.Contains(t, linha, `for /l %i in (1,1,60) do`)
	assert.Contains(t, linha, `ping -n 2 127.0.0.1 >nul`)
	assert.True(t, strings.HasSuffix(linha, `exit)"`), linha)
}
