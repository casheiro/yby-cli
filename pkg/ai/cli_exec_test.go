package ai

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain deixa o binário de teste servir de CLI falso: com YBY_CLI_FALSO definido ele não
// roda os testes, só se comporta como o claude/gemini conforme o modo.
func TestMain(m *testing.M) {
	switch os.Getenv("YBY_CLI_FALSO") {
	case "eco":
		stdin := new(bytes.Buffer)
		_, _ = stdin.ReadFrom(os.Stdin)
		_ = os.WriteFile(os.Getenv("YBY_CLI_STDIN"), stdin.Bytes(), 0o600)
		_ = os.WriteFile(os.Getenv("YBY_CLI_ARGS"), []byte(strings.Join(os.Args[1:], "\x00")), 0o600)
		os.Stdout.WriteString("Loaded cached credentials.\nresposta\nUsing foo\nsegunda linha\nfim sem quebra")
		os.Exit(0)
	case "dormir":
		time.Sleep(30 * time.Second)
		os.Exit(0)
	case "falhar":
		os.Stderr.WriteString(strings.Repeat("x", 10000) + "MOTIVO")
		os.Exit(3)
	}
	os.Exit(m.Run())
}

func cliFalsoEco(t *testing.T) (comando, argsArq, stdinArq string) {
	t.Helper()
	exe, err := os.Executable()
	require.NoError(t, err)
	dir := t.TempDir()
	argsArq, stdinArq = dir+"/args", dir+"/stdin"
	t.Setenv("YBY_CLI_FALSO", "eco")
	t.Setenv("YBY_CLI_ARGS", argsArq)
	t.Setenv("YBY_CLI_STDIN", stdinArq)
	return exe, argsArq, stdinArq
}

func TestClaudeCLI_PromptGrandeVaiPeloStdin(t *testing.T) {
	exe, argsArq, stdinArq := cliFalsoEco(t)
	p := &ClaudeCLIProvider{command: exe, model: "opus"}
	sistema, usuario := strings.Repeat("s", 60_000), strings.Repeat("u", 60_000)

	resp, err := p.Completion(context.Background(), sistema, usuario)
	require.NoError(t, err)
	assert.Contains(t, resp, "resposta")

	args, _ := os.ReadFile(argsArq)
	assert.Equal(t, "--model\x00opus\x00-p", string(args), "o prompt não vai nos argumentos")
	stdin, _ := os.ReadFile(stdinArq)
	assert.Equal(t, sistema+"\n\n"+usuario, string(stdin))
}

func TestGeminiCLI_StreamPromptPeloStdinSemLinhasDeStatus(t *testing.T) {
	exe, argsArq, stdinArq := cliFalsoEco(t)
	p := &GeminiCLIProvider{command: exe}
	var out strings.Builder

	require.NoError(t, p.StreamCompletion(context.Background(), "sis", strings.Repeat("u", 100_000), &out))

	assert.Equal(t, "resposta\nsegunda linha\nfim sem quebra", out.String())
	args, _ := os.ReadFile(argsArq)
	assert.Equal(t, "-p\x00"+instrucaoStdinGemini, string(args))
	stdin, _ := os.ReadFile(stdinArq)
	assert.Equal(t, "sis\n\n"+strings.Repeat("u", 100_000), string(stdin))
}

func TestGeminiCLI_CompletionLimpaStatus(t *testing.T) {
	exe, _, _ := cliFalsoEco(t)
	p := &GeminiCLIProvider{command: exe}

	resp, err := p.Completion(context.Background(), "s", "u")
	require.NoError(t, err)
	assert.Equal(t, "resposta\nsegunda linha\nfim sem quebra", resp)
}

func TestCLIs_CancelamentoEncerraRapido(t *testing.T) {
	exe, err := os.Executable()
	require.NoError(t, err)
	t.Setenv("YBY_CLI_FALSO", "dormir")

	casos := map[string]interface {
		Completion(context.Context, string, string) (string, error)
	}{
		"claude": &ClaudeCLIProvider{command: exe},
		"gemini": &GeminiCLIProvider{command: exe},
	}
	for nome, p := range casos {
		t.Run(nome, func(t *testing.T) {
			ctx, cancelar := context.WithCancel(context.Background())
			go func() {
				time.Sleep(200 * time.Millisecond)
				cancelar()
			}()
			inicio := time.Now()
			_, err := p.Completion(ctx, "s", "u")
			require.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
			assert.Less(t, time.Since(inicio), time.Second)
		})
	}
}

func TestCLI_StderrLimitadoNoErro(t *testing.T) {
	exe, err := os.Executable()
	require.NoError(t, err)
	t.Setenv("YBY_CLI_FALSO", "falhar")

	_, err = (&ClaudeCLIProvider{command: exe}).Completion(context.Background(), "s", "u")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MOTIVO", "o fim do stderr fica")
	assert.Less(t, len(err.Error()), 3000)
}

// contaEscritas guarda cada Write recebido, para conferir quantas vezes o filtro escreve.
type contaEscritas struct{ chamadas [][]byte }

func (c *contaEscritas) Write(b []byte) (int, error) {
	c.chamadas = append(c.chamadas, append([]byte(nil), b...))
	return len(b), nil
}

func TestFiltroStatusGemini_UmWritePorWriteDeEntrada(t *testing.T) {
	var c contaEscritas
	f := &filtroStatusGemini{out: &c}
	entrada := "Loaded cached credentials.\n" + strings.Repeat("ação de resposta longa\n", 20) + "fim sem quebra"

	n, err := f.Write([]byte(entrada))
	require.NoError(t, err)
	assert.Equal(t, len(entrada), n)
	assert.LessOrEqual(t, len(c.chamadas), 1, "um Write de entrada gera um Write de saída")
	require.NoError(t, f.fechar())
	assert.LessOrEqual(t, len(c.chamadas), 2)

	var todo []byte
	for _, ch := range c.chamadas {
		todo = append(todo, ch...)
	}
	assert.Equal(t, strings.TrimPrefix(entrada, "Loaded cached credentials.\n"), string(todo))
}

func TestFiltroStatusGemini_NaoPartePeloMeioDeCaractere(t *testing.T) {
	var c contaEscritas
	f := &filtroStatusGemini{out: &c}
	bs := []byte("resposta com acentuação para passar do prefixo é ótimo")
	for i := range bs { // um byte por vez, como num pipe fragmentado
		_, err := f.Write(bs[i : i+1])
		require.NoError(t, err)
	}
	require.NoError(t, f.fechar())

	var todo []byte
	for _, ch := range c.chamadas {
		assert.True(t, utf8.Valid(ch), "cada Write sai com UTF-8 inteiro: %q", ch)
		todo = append(todo, ch...)
	}
	assert.Equal(t, string(bs), string(todo))
}
