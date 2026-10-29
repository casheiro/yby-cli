//go:build k8s

package main

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/plugin"
	"github.com/casheiro/yby-cli/v5/pkg/plugin/sdk"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/casheiro/yby-cli/v5/plugins/sentinel/cli/backends"
	"github.com/muesli/termenv"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
)

// Contrato com as ferramentas sentinel_scan e sentinel_investigate do Bard
// (plugins/bard/tools): o Bard manda ao Sentinel os pedidos de testdata/contrato-*-pedido.json
// e lê o JSON gravado em testdata/contrato-scan.json e testdata/contrato-investigate.json.
// Este lado roda o hook command com o pedido, como o Bard roda, grava o golden (com -update)
// e confere que ele está atual; o Bard o passa pelo formatador dele. Mudar o JSON sem
// regravar o golden quebra este teste; regravar sem ajustar o Bard quebra o dele (go test
// ./plugins/bard/tools -run TestContrato).

// hookCommand roda o hook command com o pedido de testdata/<arquivoPedido> na variável do
// ambiente, como o Bard roda, e devolve o stdout. O relógio fica parado em quandoGolden,
// para o timestamp do golden não mudar.
func hookCommand(t *testing.T, arquivoPedido string) string {
	t.Helper()
	pedido, err := os.ReadFile(filepath.Join("testdata", arquivoPedido))
	if err != nil {
		t.Fatal(err)
	}
	relogio = func() time.Time { return quandoGolden }
	t.Setenv(plugin.EnvPedido, strings.TrimSpace(string(pedido)))
	t.Setenv(plugin.EnvPedidoArquivo, "")
	// O hook troca o log global (configurarLog) e o sdk guarda o pedido: os outros testes
	// não herdam nenhum dos dois.
	logAnterior := slog.Default()
	t.Cleanup(func() {
		slog.SetDefault(logAnterior)
		os.Setenv(plugin.EnvPedido, `{"hook":""}`)
		_ = sdk.Init()
	})
	if err := sdk.Init(); err != nil {
		t.Fatal(err)
	}
	var saida, erros bytes.Buffer
	if err := handlePluginRequest(&saida, &erros); err != nil {
		t.Fatalf("o hook command falhou: %v\nstdout: %s\nstderr: %s", err, saida.String(), erros.String())
	}
	return saida.String()
}

// TestContrato_Scan grava o JSON do scan --no-ai com os achados de exemplo: um achado com
// vários recursos, um de escopo cluster e uma verificação do OPA que não rodou por falta de
// permissão.
func TestContrato_Scan(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	bs := backendsDeExemplo()
	opa := bs[1].(backendFalso)
	for i, f := range opa.achados {
		if strings.HasPrefix(f.Resource, "ClusterRoleBinding/") {
			opa.achados[i].Scope = "cluster"
		}
	}
	semPermissao := apierrors.NewForbidden(schema.GroupResource{Group: "rbac.authorization.k8s.io", Resource: "clusterroles"}, "", errors.New("sem permissão"))
	opa.err = &backends.FalhaParcial{Falhas: []string{"rbac-wildcard"}, Causa: semPermissao}
	bs[1] = opa
	prov := &provedorFalso{resposta: "não deveria ser chamada"}
	usarDependencias(t, fake.NewSimpleClientset(), bs, nil, prov)

	conferirGolden(t, "contrato-scan.json", hookCommand(t, "contrato-scan-pedido.json"))
	if prov.chamadas != 0 {
		t.Errorf("o Bard pede o scan sem a IA do Sentinel, e ela foi chamada %d vez(es)", prov.chamadas)
	}
}

// TestContrato_Investigate grava o JSON do investigate --no-ai de um pod em
// CrashLoopBackOff: os dados brutos que o Bard manda à IA dele.
func TestContrato_Investigate(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	pod, ev := podComProblema()
	prov := &provedorFalso{resposta: respostaDaIA}
	usarDependencias(t, fake.NewSimpleClientset(pod, ev), nil, nil, prov)

	conferirGolden(t, "contrato-investigate.json", hookCommand(t, "contrato-investigate-pedido.json"))
	if prov.chamadas != 0 {
		t.Errorf("o investigate --no-ai não chama a IA, e ela foi chamada %d vez(es)", prov.chamadas)
	}
}
