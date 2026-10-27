package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ybyerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// prefixoPluginDeContexto marca os plugins falsos destes testes no TestHelperProcess: o
// processo auxiliar grava o que recebeu na pasta do "binário", que não existe.
const prefixoPluginDeContexto = "k01-"

// pluginDeContexto é o corpo do plugin falso. No hook context (pedido pelo stdin) acrescenta
// uma linha em <nome>.hooks e devolve {"fornecido_por": <nome>}; no command (pedido pela
// variável, como no RunInteractive) grava o pedido em <nome>.pedido.json.
func pluginDeContexto(binario string) {
	pasta, nome := filepath.Dir(binario), filepath.Base(binario)
	if pedido, err := PedidoDoAmbiente(os.Getenv); err == nil && pedido != nil {
		_ = os.WriteFile(filepath.Join(pasta, nome+".pedido.json"), pedido, 0o644)
		return
	}
	_, _ = io.ReadAll(os.Stdin)
	f, err := os.OpenFile(filepath.Join(pasta, nome+".hooks"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err == nil {
		fmt.Fprintln(f, "context")
		f.Close()
	}
	fmt.Fprintf(os.Stdout, `{"data": {"fornecido_por": %q}}`, nome)
}

// projetoDeContexto cria um projeto com dois ambientes (dev é o current) e entra nele.
func projetoDeContexto(t *testing.T) string {
	t.Helper()
	raiz := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(raiz, ".yby"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(raiz, "config"), 0o755))
	ambientes := `
current: dev
environments:
  dev:
    type: local
    kube_context: dev-ctx
    namespace: dev-ns
  staging:
    type: remote
    kube_config: kube/staging.yaml
    kube_context: staging-ctx
    namespace: staging-ns
    values: config/values-staging.yaml
`
	require.NoError(t, os.WriteFile(filepath.Join(raiz, ".yby", "environments.yaml"), []byte(ambientes), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(raiz, "config", "values-staging.yaml"), []byte("replicas: 3\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(raiz, "README.md"), []byte("# Projeto de teste\n"), 0o644))
	t.Chdir(raiz)
	return raiz
}

// managerDeContexto monta um Manager com os plugins falsos dados, em pasta, com o
// processo auxiliar no lugar dos binários.
func managerDeContexto(t *testing.T, pasta string, manifestos ...PluginManifest) *Manager {
	t.Helper()
	original := execCommandContext
	execCommandContext = mockExecCommandContext
	t.Cleanup(func() { execCommandContext = original })

	m := NewManager()
	for _, mf := range manifestos {
		m.plugins = append(m.plugins, LoadedPlugin{Manifest: mf, Path: filepath.Join(pasta, prefixoPluginDeContexto+mf.Name)})
	}
	return m
}

// pedidoRecebido lê o pedido que o plugin falso nome recebeu no command.
func pedidoRecebido(t *testing.T, pasta, nome string) PluginRequest {
	t.Helper()
	dados, err := os.ReadFile(filepath.Join(pasta, prefixoPluginDeContexto+nome+".pedido.json"))
	require.NoError(t, err, "o plugin %s não rodou o command", nome)
	var req PluginRequest
	require.NoError(t, json.Unmarshal(dados, &req))
	return req
}

// hooksRodados conta quantas vezes o hook context do plugin falso nome rodou.
func hooksRodados(pasta, nome string) int {
	dados, err := os.ReadFile(filepath.Join(pasta, prefixoPluginDeContexto+nome+".hooks"))
	if err != nil {
		return 0
	}
	return strings.Count(string(dados), "context")
}

var (
	fornecedor  = PluginManifest{Name: "fornecedor", Hooks: []string{"context"}}
	comoAtlas   = PluginManifest{Name: "atlas", Hooks: []string{"context", "manifest", "command"}}
	consumidor  = PluginManifest{Name: "consumidor", Hooks: []string{"context", "command"}, ConsomeContexto: true}
	soComando   = PluginManifest{Name: "sentinel", Hooks: []string{"command"}}
	argsDeAjuda = [][]string{{"--help"}, {"-h"}, {"index", "--help"}}
)

func TestExecuteCommandHook_QuemSoFornecContextoNaoRodaHooks(t *testing.T) {
	pasta := projetoDeContexto(t)
	m := managerDeContexto(t, pasta, fornecedor, comoAtlas)

	require.NoError(t, m.ExecuteCommandHook("atlas", nil))

	assert.Zero(t, hooksRodados(pasta, "fornecedor"), "o hook context de outro plugin rodou")
	assert.Zero(t, hooksRodados(pasta, "atlas"), "o hook context do próprio plugin rodou")
	assert.Nil(t, pedidoRecebido(t, pasta, "atlas").Context, "sem ambiente pedido e sem consome_contexto, nada de contexto")
}

func TestExecuteCommandHook_AjudaNaoMontaContexto(t *testing.T) {
	for _, args := range argsDeAjuda {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			pasta := projetoDeContexto(t)
			m := managerDeContexto(t, pasta, fornecedor, consumidor)
			m.DefinirAmbiente("staging")

			require.NoError(t, m.ExecuteCommandHook("consumidor", args))

			assert.Zero(t, hooksRodados(pasta, "fornecedor"))
			req := pedidoRecebido(t, pasta, "consumidor")
			assert.Equal(t, args, req.Args)
			assert.Nil(t, req.Context)
		})
	}
}

func TestExecuteCommandHook_ConsumidorRecebeOsHooksDosOutros(t *testing.T) {
	pasta := projetoDeContexto(t)
	m := managerDeContexto(t, pasta, fornecedor, consumidor)

	require.NoError(t, m.ExecuteCommandHook("consumidor", []string{"x"}))

	assert.Equal(t, 1, hooksRodados(pasta, "fornecedor"))
	assert.Zero(t, hooksRodados(pasta, "consumidor"), "o consumidor não fornece contexto a si mesmo")
	ctx := pedidoRecebido(t, pasta, "consumidor").Context
	require.NotNil(t, ctx)
	dados, _ := ctx["data"].(map[string]interface{})
	assert.Equal(t, "k01-fornecedor", dados["fornecido_por"])
	assert.Contains(t, dados["overview"], "Projeto de teste")
	assert.NotContains(t, dados, "uki_index")
	// sem ambiente pedido, o current do environments.yaml não vira infra
	assert.Empty(t, ctx["environment"])
	infra, _ := ctx["infra"].(map[string]interface{})
	assert.Empty(t, infra["kube_context"])
}

func TestExecuteCommandHook_AmbientePedidoChegaATodoPlugin(t *testing.T) {
	pasta := projetoDeContexto(t)
	m := managerDeContexto(t, pasta, fornecedor, soComando)
	m.DefinirAmbiente("staging")

	require.NoError(t, m.ExecuteCommandHook("sentinel", []string{"scan"}))

	assert.Zero(t, hooksRodados(pasta, "fornecedor"), "o contexto barato não roda hooks")
	raw := pedidoRecebido(t, pasta, "sentinel").Context
	require.NotNil(t, raw)
	var ctx PluginFullContext
	b, _ := json.Marshal(raw)
	require.NoError(t, json.Unmarshal(b, &ctx))
	assert.Equal(t, "staging", ctx.Environment)
	assert.Equal(t, filepath.Base(pasta), ctx.ProjectName)
	assert.Equal(t, PluginInfrastructure{
		KubeConfig:  filepath.Join(pasta, "kube", "staging.yaml"),
		KubeContext: "staging-ctx",
		Namespace:   "staging-ns",
	}, ctx.Infra)
	assert.EqualValues(t, 3, ctx.Values["replicas"])
	assert.Nil(t, ctx.Data)
}

func TestExecuteCommandHook_SemAmbientePedidoNadaDoCurrent(t *testing.T) {
	pasta := projetoDeContexto(t)
	m := managerDeContexto(t, pasta, soComando)

	require.NoError(t, m.ExecuteCommandHook("sentinel", []string{"scan"}))

	assert.Nil(t, pedidoRecebido(t, pasta, "sentinel").Context, "o current (dev) não pode mudar o yby sentinel puro")
}

func TestExecuteCommandHook_AmbientePedidoQueNaoExiste(t *testing.T) {
	pasta := projetoDeContexto(t)
	m := managerDeContexto(t, pasta, soComando)
	m.DefinirAmbiente("prod")

	err := m.ExecuteCommandHook("sentinel", []string{"scan"})

	var yerr *ybyerrors.YbyError
	require.ErrorAs(t, err, &yerr)
	assert.Equal(t, ybyerrors.ErrCodeConfig, yerr.Code)
	assert.Contains(t, yerr.Message, "prod")
	_, errPedido := os.Stat(filepath.Join(pasta, prefixoPluginDeContexto+"sentinel.pedido.json"))
	assert.True(t, os.IsNotExist(errPedido), "o plugin não pode rodar no cluster errado")
}

func TestExecuteContextHook_UsaOContextoDoChamador(t *testing.T) {
	pasta := t.TempDir()
	m := managerDeContexto(t, pasta, fornecedor)
	cancelado, cancelar := context.WithCancel(context.Background())
	cancelar()

	bp := &scaffold.BlueprintContext{Data: map[string]interface{}{}}
	require.NoError(t, m.ExecuteContextHook(cancelado, bp))

	assert.Zero(t, hooksRodados(pasta, "fornecedor"), "o hook rodou com o contexto já cancelado")
	assert.NotContains(t, bp.Data, "fornecido_por")

	require.NoError(t, m.ExecuteContextHook(context.Background(), bp))
	assert.Equal(t, "k01-fornecedor", bp.Data["fornecido_por"], "o yby init continua usando o hook context")
}

func TestPluginManifest_ConsomeContextoNoJSON(t *testing.T) {
	var mf PluginManifest
	require.NoError(t, json.Unmarshal([]byte(`{"name":"x","hooks":["command"],"consome_contexto":true}`), &mf))
	assert.True(t, mf.ConsomeContexto)

	b, err := json.Marshal(PluginManifest{Name: "x", Hooks: []string{"command"}})
	require.NoError(t, err)
	assert.NotContains(t, string(b), "consome_contexto", "quem não consome não precisa declarar")
}
