package ai

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	ybyerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// isolarConfig aponta o home para um diretório vazio e limpa as variáveis de modelo: o
// config.yaml de quem roda os testes não entra na conta.
func isolarConfig(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for _, v := range []string{"YBY_AI_MODEL", "YBY_AI_PROVIDER", "GEMINI_MODEL", "GEMINI_API_KEY", "OPENAI_API_KEY"} {
		t.Setenv(v, "")
	}
}

// cadeia desce pelos decorators do wrapProvider e devolve o provider de dentro, falhando
// se faltar algum.
func cadeia(t *testing.T, p Provider) Provider {
	t.Helper()
	retry, ok := p.(*RetryProvider)
	require.True(t, ok, "RetryProvider por fora: %T", p)
	rate, ok := retry.inner.(*RateLimitProvider)
	require.True(t, ok, "RateLimitProvider: %T", retry.inner)
	cost, ok := rate.inner.(*CostTrackingProvider)
	require.True(t, ok, "CostTrackingProvider: %T", rate.inner)
	token, ok := cost.inner.(*TokenAwareProvider)
	require.True(t, ok, "TokenAwareProvider: %T", cost.inner)
	cached, ok := token.inner.(*CachedEmbeddingProvider)
	require.True(t, ok, "CachedEmbeddingProvider: %T", token.inner)
	assert.Equal(t, cost.model, token.model, "o mesmo modelo nos decorators")
	return cached.inner
}

func codigoErro(t *testing.T, err error) *ybyerrors.YbyError {
	t.Helper()
	var ye *ybyerrors.YbyError
	require.True(t, stderrors.As(err, &ye), "YbyError: %v", err)
	return ye
}

func TestGetProviderWithModel_GeminiComModeloEscolhido(t *testing.T) {
	isolarConfig(t)
	t.Setenv("GEMINI_API_KEY", "chave-de-teste")

	p, modelo, err := GetProviderWithModel(context.Background(), "gemini", "Gemini-2.5-PRO")
	require.NoError(t, err)
	assert.Equal(t, "gemini-2.5-pro", modelo)
	g, ok := cadeia(t, p).(*GeminiProvider)
	require.True(t, ok)
	assert.Equal(t, "gemini-2.5-pro", g.Model, "o nome volta como o provider conhece")
	cost := p.(*RetryProvider).inner.(*RateLimitProvider).inner.(*CostTrackingProvider)
	assert.Equal(t, "gemini-2.5-pro", cost.model, "os decorators levam o modelo escolhido")

	// Sem modelo, fica o padrão; o modelo da sessão não vaza para o config nem para a env.
	p, modelo, err = GetProviderWithModel(context.Background(), "gemini", "")
	require.NoError(t, err)
	assert.Empty(t, modelo, "sem escolha, sem modelo da sessão")
	assert.Equal(t, "gemini-2.5-flash", cadeia(t, p).(*GeminiProvider).Model)
	assert.Empty(t, os.Getenv("YBY_AI_MODEL"))
	_, err = os.Stat(filepath.Join(os.Getenv("HOME"), ".yby", "config.yaml"))
	assert.True(t, os.IsNotExist(err), "o config do usuário não é gravado")
}

func TestGetProviderWithModel_OpenAIAceitaPelaRegra(t *testing.T) {
	isolarConfig(t)
	t.Setenv("OPENAI_API_KEY", "sk-teste")

	p, _, err := GetProviderWithModel(context.Background(), "openai", "gpt-4o")
	require.NoError(t, err)
	assert.Equal(t, "gpt-4o", cadeia(t, p).(*OpenAIProvider).Model)

	_, _, err = GetProviderWithModel(context.Background(), "openai", "gemini-2.5-pro")
	ye := codigoErro(t, err)
	assert.Equal(t, ybyerrors.ErrCodeValidation, ye.Code)
	assert.Equal(t, "o openai não tem o modelo gemini-2.5-pro", ye.Message)
	assert.Contains(t, ye.Hint, "gpt-4o-mini, gpt-4o, gpt-4-turbo")
}

func TestGetProviderWithModel_ModeloInvalido(t *testing.T) {
	isolarConfig(t)
	t.Setenv("GEMINI_API_KEY", "chave-de-teste")

	_, _, err := GetProviderWithModel(context.Background(), "gemini", "gpt-4o")
	ye := codigoErro(t, err)
	assert.Equal(t, "o gemini não tem o modelo gpt-4o", ye.Message)
	assert.NotEmpty(t, ye.Hint)

	_, _, err = GetProviderWithModel(context.Background(), "provider-xyz", "qualquer")
	assert.Equal(t, "o provider provider-xyz não aceita escolha de modelo", codigoErro(t, err).Message)
}

func TestGetProviderWithModel_ProviderIndisponivel(t *testing.T) {
	isolarConfig(t)
	_, _, err := GetProviderWithModel(context.Background(), "gemini", "gemini-2.5-pro")
	assert.Equal(t, "o provider gemini não está disponível", codigoErro(t, err).Message)
}

// cliFalso põe no PATH um executável com o nome do CLI, que nunca é executado: só o
// LookPath do IsAvailable o procura.
func cliFalso(t *testing.T, nome string) {
	t.Helper()
	dir := t.TempDir()
	arquivo := filepath.Join(dir, nome)
	if runtime.GOOS == "windows" {
		arquivo += ".bat"
	}
	require.NoError(t, os.WriteFile(arquivo, []byte("exit 1\n"), 0o755))
	t.Setenv("PATH", dir)
}

func TestGetProviderWithModel_CLIs(t *testing.T) {
	isolarConfig(t)
	cliFalso(t, "claude")

	p, _, err := GetProviderWithModel(context.Background(), "claude-cli", "Opus")
	require.NoError(t, err)
	c, ok := p.(*ClaudeCLIProvider)
	require.True(t, ok, "os CLIs seguem sem decorators: %T", p)
	assert.Equal(t, "opus", c.model)
	assert.Equal(t, []string{"--model", "opus", "-p", "oi"}, c.args("oi"))
	assert.Equal(t, []string{"-p", "oi"}, NewClaudeCLIProvider().args("oi"), "sem modelo, o CLI escolhe")

	_, _, err = GetProviderWithModel(context.Background(), "claude-cli", "gpt-4o")
	assert.Equal(t, "o claude-cli não tem o modelo gpt-4o", codigoErro(t, err).Message)

	g := &GeminiCLIProvider{command: "gemini", model: "gemini-2.5-pro"}
	assert.Equal(t, []string{"--model", "gemini-2.5-pro", "-p", "oi"}, g.args("oi"))
}

func TestListModels_ListasConhecidas(t *testing.T) {
	isolarConfig(t)
	ctx := context.Background()

	claude := ListModels(ctx, "claude-cli")
	assert.Equal(t, "", claude.Default, "o CLI escolhe o modelo")
	assert.Equal(t, []string{"sonnet", "opus", "haiku"}, claude.Models)

	gemini := ListModels(ctx, "gemini")
	assert.Equal(t, "gemini-2.5-flash", gemini.Default)
	assert.Equal(t, []string{"gemini-2.5-flash", "gemini-2.5-pro", "gemini-2.0-flash"}, gemini.Models)
	assert.Equal(t, []string{"gemini-2.5-flash", "gemini-2.5-pro", "gemini-2.0-flash"}, ListModels(ctx, "gemini-cli").Models)

	openai := ListModels(ctx, "openai")
	assert.Equal(t, "gpt-4o-mini", openai.Default)
	assert.Equal(t, []string{"gpt-4o-mini", "gpt-4o", "gpt-4-turbo"}, openai.Models)

	// O modelo configurado entra na frente da lista.
	t.Setenv("GEMINI_MODEL", "gemini-1.5-pro")
	assert.Equal(t, []string{"gemini-1.5-pro", "gemini-2.5-flash", "gemini-2.5-pro", "gemini-2.0-flash"}, ListModels(ctx, "gemini").Models)

	assert.Empty(t, ListModels(ctx, "provider-xyz").Models)
}

// ollamaFalso responde o /api/tags com os modelos dados.
func ollamaFalso(t *testing.T, modelos ...string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("o Ollama só deveria ser consultado em /api/tags: %s", r.URL.Path)
		}
		var tags ollamaTagsResponse
		for _, m := range modelos {
			tags.Models = append(tags.Models, struct {
				Name string `json:"name"`
			}{m})
		}
		_ = json.NewEncoder(w).Encode(tags)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestListModels_OllamaLocal(t *testing.T) {
	isolarConfig(t)
	srv := ollamaFalso(t, "qwen2.5:7b", "llama3:latest")

	cat := listOllamaModels(context.Background(), &OllamaProvider{Model: defaultOllamaModel, Endpoints: []string{srv.URL}})
	assert.True(t, cat.Local)
	assert.Empty(t, cat.Warning)
	assert.Equal(t, "qwen2.5:7b", cat.Default, "sem config, o Ollama usa o primeiro baixado")
	assert.Equal(t, []string{"qwen2.5:7b", "llama3:latest"}, cat.Models)

	// Com modelo no config, ele é o padrão.
	cat = listOllamaModels(context.Background(), &OllamaProvider{Model: "mistral", modelConfigured: true, Endpoints: []string{srv.URL}})
	assert.Equal(t, []string{"mistral", "qwen2.5:7b", "llama3:latest"}, cat.Models)
}

func TestListModels_OllamaForaDoAr(t *testing.T) {
	isolarConfig(t)
	// Uma porta que acabou de fechar: ninguém escuta nela.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	fechado := "http://" + l.Addr().String()
	require.NoError(t, l.Close())

	inicio := time.Now()
	cat := listOllamaModels(context.Background(), &OllamaProvider{Model: defaultOllamaModel, Endpoints: []string{fechado}})
	assert.Less(t, time.Since(inicio), 2*time.Second, "o Ollama fora do ar não prende a lista")
	assert.False(t, cat.Local)
	assert.Equal(t, "o Ollama não respondeu", cat.Warning)
	assert.Equal(t, []string{"llama3"}, cat.Models, "fica só o modelo atual")

	// O aviso e a regra dizem o mesmo: fora do ar, nenhum nome digitado é aceito.
	ye := codigoErro(t, erroListaOllama(cat, "llama3"))
	assert.Equal(t, "o Ollama não respondeu", ye.Message)
	assert.Contains(t, ye.Hint, "ollama list")
}

// Um Ollama que aceita a conexão e não responde ao /api/tags não prende a lista: o ping
// passa e a leitura dos modelos fica presa até o limite.
func TestListModels_OllamaTravado(t *testing.T) {
	isolarConfig(t)
	var pedidos atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if pedidos.Add(1) == 1 {
			_ = json.NewEncoder(w).Encode(ollamaTagsResponse{})
			return
		}
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	inicio := time.Now()
	cat := listOllamaModels(context.Background(), &OllamaProvider{Model: defaultOllamaModel, Endpoints: []string{srv.URL}})
	assert.Less(t, time.Since(inicio), 2*time.Second, "o Ollama travado não prende a lista")
	assert.Equal(t, int32(2), pedidos.Load(), "o ping e a leitura dos modelos")
	assert.False(t, cat.Local)
	assert.Equal(t, "o Ollama não respondeu", cat.Warning)
	assert.Equal(t, []string{"llama3"}, cat.Models)
}

func TestGetProviderWithModel_OllamaSemModelos(t *testing.T) {
	isolarConfig(t)
	srv := ollamaFalso(t)
	t.Setenv("OLLAMA_HOST", srv.URL)

	cat := ListModels(context.Background(), "ollama")
	assert.Equal(t, "nenhum modelo baixado no Ollama", cat.Warning)
	_, _, err := GetProviderWithModel(context.Background(), "ollama", "llama3")
	ye := codigoErro(t, err)
	assert.Equal(t, "nenhum modelo baixado no Ollama", ye.Message, "a leitura deu certo e voltou vazia")
	assert.Contains(t, ye.Hint, "ollama pull")
}

func TestGetProviderWithModel_Ollama(t *testing.T) {
	isolarConfig(t)
	srv := ollamaFalso(t, "qwen2.5:7b", "llama3:latest")
	t.Setenv("OLLAMA_HOST", srv.URL)

	p, modelo, err := GetProviderWithModel(context.Background(), "ollama", "llama3")
	require.NoError(t, err)
	assert.Equal(t, "llama3:latest", modelo, "a sessão guarda o nome que o Ollama conhece")
	o, ok := cadeia(t, p).(*OllamaProvider)
	require.True(t, ok)
	assert.Equal(t, "llama3:latest", o.Model, "llama3 é o llama3:latest do /api/tags")
	assert.True(t, o.modelConfigured, "o modelo escolhido não é trocado pelo primeiro baixado")

	_, _, err = GetProviderWithModel(context.Background(), "ollama", "mistral")
	ye := codigoErro(t, err)
	assert.Equal(t, "o ollama não tem o modelo mistral", ye.Message)
	assert.Contains(t, ye.Hint, "qwen2.5:7b, llama3:latest")
}
