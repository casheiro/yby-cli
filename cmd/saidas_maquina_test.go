package cmd

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// Goldens das saídas para máquina (export do audit, telemetry export, arquivo do
// gen-docs): gravados antes da migração para a carta do terminal e conferidos depois,
// para provar que continuam byte a byte iguais.

// eventosAuditGolden são linhas JSONL de ~/.yby/audit.log com um sucesso e uma falha.
const eventosAuditGolden = `{"timestamp":"2026-10-20T08:15:00Z","action":"authenticate","provider":"aws","identity":"arn:aws:iam::123456789012:user/ana","method":"sso","success":true,"duration":"1.2s"}
{"timestamp":"2026-10-21T14:40:05Z","action":"refresh","provider":"gcp","identity":"ana@empresa.com.br","cluster":"prod-gke","region":"us-central1","success":false,"error":"token expirado, rode gcloud auth login"}
`

// usarHomeTemporaria aponta o diretório do usuário (HOME e USERPROFILE) para uma pasta
// temporária e devolve o caminho dela.
func usarHomeTemporaria(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".yby"), 0o755))
	return home
}

// capturarSaidaPadrao roda fn com os.Stdout e a saída de c ligados a um pipe e devolve o que
// foi escrito: vale tanto para o código que escreve em os.Stdout quanto em OutOrStdout.
func capturarSaidaPadrao(t *testing.T, c *cobra.Command, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	antigo := os.Stdout
	os.Stdout = w
	c.SetOut(w)
	lido := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		lido <- string(b)
	}()
	defer func() {
		os.Stdout = antigo
		c.SetOut(nil)
	}()
	fn()
	_ = w.Close()
	return <-lido
}

func TestCloudAuditExport_SaidaInalterada(t *testing.T) {
	for _, formato := range []string{"json", "csv"} {
		t.Run(formato, func(t *testing.T) {
			home := usarHomeTemporaria(t)
			require.NoError(t, os.WriteFile(filepath.Join(home, ".yby", "audit.log"), []byte(eventosAuditGolden), 0o600))
			require.NoError(t, cloudAuditCmd.Flags().Set("export", formato))
			t.Cleanup(func() { _ = cloudAuditCmd.Flags().Set("export", "") })

			var errRun error
			saida := capturarSaidaPadrao(t, cloudAuditCmd, func() { errRun = runCloudAudit(cloudAuditCmd, nil) })
			require.NoError(t, errRun)
			conferirGolden(t, "maquina/cloud_audit_export_"+formato, saida)
		})
	}
}

func TestTelemetryExport_SaidaInalterada(t *testing.T) {
	home := usarHomeTemporaria(t)
	jsonl := `{"name":"up","duration_ms":3812,"success":true,"timestamp":"2026-10-20T08:15:00Z"}
{"name":"doctor","duration_ms":120,"success":false,"error":"kubectl ausente","timestamp":"2026-10-21T14:40:05Z"}
`
	require.NoError(t, os.WriteFile(filepath.Join(home, ".yby", "telemetry.jsonl"), []byte(jsonl), 0o600))

	var errRun error
	saida := capturarSaidaPadrao(t, telemetryExportCmd, func() { errRun = telemetryExportCmd.RunE(telemetryExportCmd, nil) })
	require.NoError(t, errRun)
	conferirGolden(t, "maquina/telemetry_export", saida)
}

func TestGenDocs_CabecalhoDoArquivoInalterado(t *testing.T) {
	dir := t.TempDir()
	capturarSaidaPadrao(t, genDocsCmd, func() { require.NoError(t, genDocsCmd.RunE(genDocsCmd, []string{dir})) })

	bruto, err := os.ReadFile(filepath.Join(dir, "CLI-Reference.md"))
	require.NoError(t, err)
	// as duas primeiras linhas não vêm da ajuda dos comandos, que muda com eles
	cabecalho := strings.Join(strings.SplitAfterN(string(bruto), "\n", 5)[:4], "")
	conferirGolden(t, "maquina/gen_docs_cabecalho", cabecalho)
}

func TestVersion_SaidaInalterada(t *testing.T) {
	antVersao, antCommit, antData := Version, commit, date
	t.Cleanup(func() { Version, commit, date = antVersao, antCommit, antData })
	Version, commit, date = "v5.6.1", "48bb44a", "2026-10-20T08:15:00Z"
	t.Cleanup(func() { _ = rootCmd.PersistentFlags().Set("log-format", "text") })

	for _, formato := range []string{"text", "json"} {
		t.Run(formato, func(t *testing.T) {
			require.NoError(t, rootCmd.PersistentFlags().Set("log-format", formato))
			var errRun error
			saida := capturarSaidaPadrao(t, versionCmd, func() { errRun = versionCmd.RunE(versionCmd, nil) })
			require.NoError(t, errRun)
			// o sistema e a arquitetura mudam com a máquina que roda o teste
			saida = strings.ReplaceAll(saida, runtime.GOOS, "<os>")
			saida = strings.ReplaceAll(saida, runtime.GOARCH, "<arch>")
			conferirGolden(t, "maquina/version_"+formato, saida)
		})
	}
}
