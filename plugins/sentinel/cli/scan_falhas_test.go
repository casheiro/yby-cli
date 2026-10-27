//go:build k8s

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/pkg/ai"
	yerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/backends"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/checks"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// recusado é o erro do client-go com o cluster fora do ar.
var recusado = errors.New(`Get "https://127.0.0.1:6443/api/v1/namespaces/payments": dial tcp 127.0.0.1:6443: connect: connection refused`)

// clienteForaDoAr devolve um clientset falso em que as ações verbo (ou todas, com "*")
// falham como num cluster fora do ar.
func clienteForaDoAr(verbo string) *fake.Clientset {
	c := fake.NewSimpleClientset()
	c.PrependReactor(verbo, "*", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, recusado
	})
	return c
}

// proibido é o erro do Kubernetes de quem não tem permissão para o recurso.
func proibido(recurso string) error {
	return apierrors.NewForbidden(schema.GroupResource{Resource: recurso}, "", errors.New("RBAC"))
}

// logNoNivelInfo manda o slog, no nível info, para o buffer devolvido: um aviso cru que
// iria para a tela aparece nele.
func logNoNivelInfo(t *testing.T) *bytes.Buffer {
	t.Helper()
	var log bytes.Buffer
	anterior := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(anterior) })
	return &log
}

// semRelatorio confere que nenhum relatório foi gravado em ~/.yby/reports.
func semRelatorio(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(homeDeTeste, ".yby", "reports")); err == nil {
		t.Error("nenhum relatório é gravado quando o scan não roda")
	}
}

// TestScan_ClusterForaDoAr: com o cluster fora do ar, o scan para antes do título com
// ERR_CLUSTER_OFFLINE (como o investigate), sai com 1 e não escreve log cru.
func TestScan_ClusterForaDoAr(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	usarDependencias(t, clienteForaDoAr("*"), []backends.SecurityBackend{backends.NewOPABackend()}, checks.GetAll(), nil)
	log := logNoNivelInfo(t)

	saida, erros, err := rodar(t, "scan", "-n", "payments")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeClusterOffline || !strings.Contains(ye.Hint, "kubectl get pods -n payments") {
		t.Fatalf("esperava ERR_CLUSTER_OFFLINE com o hint do cluster, obtido %v", err)
	}
	if saida != "" || erros != "" || log.Len() > 0 {
		t.Errorf("nada de título, etapas ou log com o cluster fora do ar:\n%q\n%q\n%q", saida, erros, log.String())
	}
	var b bytes.Buffer
	if codigo := sairComErro(&b, err); codigo != 1 || !strings.Contains(b.String(), "ERR_CLUSTER_OFFLINE") || !strings.Contains(b.String(), "faça") {
		t.Errorf("sai com 1 no formato causa/faça/código (código %d):\n%s", codigo, b.String())
	}
	semRelatorio(t)
}

// TestScan_ClusterCaiDepoisDoNamespace: o namespace foi lido, mas toda listagem falha. O
// OPA não vira "0 achados" e nada diz "nenhum problema": o scan sai com
// ERR_CLUSTER_OFFLINE, sem log cru na tela.
func TestScan_ClusterCaiDepoisDoNamespace(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	polaris := backendFalso{nome: "polaris", err: recusado}
	usarDependencias(t, clienteForaDoAr("list"), []backends.SecurityBackend{polaris, backends.NewOPABackend()}, checks.GetAll(), &provedorFalso{resposta: "nada"})
	log := logNoNivelInfo(t)

	saida, erros, err := rodar(t, "scan", "-n", "payments")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeClusterOffline {
		t.Fatalf("esperava ERR_CLUSTER_OFFLINE, obtido %v\n%s", err, saida)
	}
	for _, vedado := range []string{"✓ opa", "0 achados", "nenhum problema", "level=WARN", "time=", "dial tcp", "connection refused", "forbidden"} {
		if strings.Contains(saida, vedado) || strings.Contains(erros, vedado) {
			t.Errorf("%q não pode aparecer com o cluster fora do ar:\n%s\n%s", vedado, saida, erros)
		}
	}
	if !strings.Contains(saida, "✕ opa") || !strings.Contains(saida, "✕ polaris") || !strings.Contains(saida, "    o cluster parou de responder") {
		t.Errorf("os backends aparecem como falha, com o motivo em PT-BR:\n%s", saida)
	}
	if log.Len() > 0 {
		t.Errorf("as falhas do OPA ficam no log de depuração:\n%s", log.String())
	}
	semRelatorio(t)
}

// TestScan_NamespaceInexistente: um namespace que não existe é erro de validação em PT-BR
// com hint, antes do título, sem achados do OPA nem relatório.
func TestScan_NamespaceInexistente(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	opa := backendFalso{nome: "opa", achados: []backends.Finding{{ID: "opa/netpol_default_deny", Severity: "medium", Category: "network", Resource: "nao-existe", Message: "Namespace 'nao-existe' sem default-deny"}}}
	usarDependencias(t, fake.NewSimpleClientset(), []backends.SecurityBackend{opa}, nil, nil)

	saida, _, err := rodar(t, "scan", "-n", "nao-existe")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeValidation {
		t.Fatalf("esperava ERR_VALIDATION, obtido %v", err)
	}
	if ye.Message != "o namespace nao-existe não existe no cluster" || ye.Cause != nil || !strings.Contains(ye.Hint, "kubectl get ns") || !strings.Contains(ye.Hint, "yby viz") {
		t.Errorf("mensagem em PT-BR, sem a causa em inglês, com o hint dos namespaces: %+v", ye)
	}
	if saida != "" {
		t.Errorf("o erro sai antes do título e das etapas:\n%s", saida)
	}
	var b bytes.Buffer
	if codigo := sairComErro(&b, err); codigo != 1 || strings.Contains(b.String(), "not found") {
		t.Errorf("sai com 1, sem texto em inglês (código %d):\n%s", codigo, b.String())
	}
	semRelatorio(t)
}

// TestScan_SemPermissaoParaNamespacesSegue: sem permissão para ler namespaces, o scan
// segue como antes.
func TestScan_SemPermissaoParaNamespacesSegue(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	c := fake.NewSimpleClientset()
	c.PrependReactor("get", "namespaces", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, proibido("namespaces")
	})
	usarDependencias(t, c, nil, []checks.SecurityCheck{checkFalso{}}, nil)

	saida, _, err := rodar(t, "scan", "-n", "payments")
	if err != nil || !strings.Contains(saida, "✓ checks internos") {
		t.Errorf("sem permissão para o namespace, o scan segue (err=%v):\n%s", err, saida)
	}
}

// opaParcial é o OPA sem permissão para os ClusterRoles: um achado e a falha parcial.
type opaParcial struct{}

func (opaParcial) Name() string      { return "opa" }
func (opaParcial) IsAvailable() bool { return true }
func (opaParcial) ScanCluster(context.Context, kubernetes.Interface, string) ([]backends.Finding, error) {
	achados := []backends.Finding{
		{ID: "opa/netpol_default_deny", Severity: "medium", Category: "network", Resource: "payments", Message: "Namespace 'payments' não possui NetworkPolicy default-deny para Ingress"},
	}
	return achados, &backends.FalhaParcial{Falhas: []string{"acesso a secrets", "wildcards"}, Causa: proibido("clusterroles")}
}

func TestScan_FalhaParcialDoBackendViraAviso(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	usarDependencias(t, fake.NewSimpleClientset(), []backends.SecurityBackend{opaParcial{}}, nil, nil)

	saida, _, err := rodar(t, "scan", "-n", "payments")
	if err != nil {
		t.Fatal(err)
	}
	for _, trecho := range []string{"✓ opa   1 achado", "▲ 2 verificações não rodaram: acesso a secrets, wildcards", "REDE", "RESUMO  1 médio"} {
		if !strings.Contains(saida, trecho) {
			t.Errorf("faltou %q:\n%s", trecho, saida)
		}
	}
	if strings.Contains(saida, "forbidden") {
		t.Errorf("o motivo em inglês fica no log de depuração:\n%s", saida)
	}
}

// TestScan_RecursoLongoNaColuna: um nome longo é encurtado no meio, com a contagem
// inteira, e a severidade e a descrição ficam na coluna, na mesma linha.
func TestScan_RecursoLongoNaColuna(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	achados := []backends.Finding{
		{ID: "polaris/missingNetworkPolicy", Severity: "high", Category: "pod-security", Resource: "Deployment/local-path-provisioner (+6)", Message: "x"},
		{ID: "polaris/priorityClassNotSet", Severity: "high", Category: "pod-security", Resource: "Deployment/api (+5)", Message: "y"},
	}
	usarDependencias(t, fake.NewSimpleClientset(), []backends.SecurityBackend{backendFalso{nome: "polaris", achados: achados}}, nil, nil)

	saida, _, err := rodar(t, "scan", "-n", "payments")
	if err != nil {
		t.Fatal(err)
	}
	var linhas []string
	for _, l := range strings.Split(saida, "\n") {
		if strings.HasPrefix(l, "  ▲ Deployment/") {
			linhas = append(linhas, l)
		}
	}
	if len(linhas) != 2 {
		t.Fatalf("esperava as duas linhas de achado:\n%s", saida)
	}
	if !strings.Contains(linhas[0], "Deployment/api (+5)") || !strings.Contains(linhas[1], "Deployment/l…-provisioner (+6)") {
		t.Errorf("o nome longo é encurtado no meio, com a contagem inteira:\n%s", strings.Join(linhas, "\n"))
	}
	coluna := func(l string) int { return lipgloss.Width(l[:strings.Index(l, "alto")]) }
	if c0, c1 := coluna(linhas[0]), coluna(linhas[1]); c0 != c1 {
		t.Errorf("a severidade fica na mesma coluna (%d e %d):\n%s", c0, c1, strings.Join(linhas, "\n"))
	}
	if !strings.Contains(linhas[1], "alto  Nenhuma NetworkPolicy seleciona os pods") {
		t.Errorf("a descrição fica na mesma linha:\n%s", linhas[1])
	}
}

func TestEncurtarRecurso(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	casos := map[string]string{
		"Deployment/api (+5)":                     "Deployment/api (+5)",
		"Deployment/local-path-provisioner (+6)":  "Deployment/l…-provisioner (+6)",
		"ClusterRole/local-path-provisioner-role": "ClusterRole/loc…ovisioner-role",
		"ClusterRoleBinding/ci-deployer":          "ClusterRoleBinding/ci-deployer",
	}
	for entrada, esperado := range casos {
		got := encurtarRecurso(entrada, 30)
		if got != esperado || lipgloss.Width(got) > 30 {
			t.Errorf("encurtarRecurso(%q) = %q (%d células), esperado %q", entrada, got, lipgloss.Width(got), esperado)
		}
	}
}

// TestScan_RecomendacoesNaTela: o começo das recomendações da IA aparece no terminal, sem
// a marcação do markdown, e o resto fica no relatório.
func TestScan_RecomendacoesNaTela(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	texto := "## Segurança dos pods\n\n1. **Tire o root** do `api`.\n2. Defina limites.\n\n---\n\n## RBAC\n\n* Tire o cluster-admin do ci-deployer.\n4. a\n5. b\n6. c"
	usarDependencias(t, fake.NewSimpleClientset(), backendsDeExemplo(), nil, &provedorFalso{resposta: texto})

	saida, _, err := rodar(t, "scan", "-n", "payments")
	if err != nil {
		t.Fatal(err)
	}
	esperado := "RECOMENDAÇÕES DA IA\n" +
		"  Segurança dos pods\n" +
		"  1. Tire o root do api.\n" +
		"  2. Defina limites.\n" +
		"  RBAC\n" +
		"  - Tire o cluster-admin do ci-deployer.\n" +
		"  4. a\n" +
		"  ○ o texto inteiro está no relatório\n"
	if !strings.Contains(saida, esperado) {
		t.Errorf("recomendações fora do formato:\n%s", saida)
	}
	if strings.Index(saida, "RECOMENDAÇÕES DA IA") > strings.Index(saida, "RESUMO") {
		t.Errorf("as recomendações vêm antes do resumo:\n%s", saida)
	}
}

// TestScan_SemIA: com --no-ai (o Bard pede assim) a IA não é chamada, a etapa das
// recomendações não aparece e o relatório sai sem elas.
func TestScan_SemIA(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	prov := &provedorFalso{resposta: "recomendações"}
	usarDependencias(t, fake.NewSimpleClientset(), backendsDeExemplo(), nil, prov)

	saida, erros, err := rodar(t, "scan", "-n", "payments", "-o", "json", "--no-ai")
	if err != nil {
		t.Fatal(err)
	}
	var rel ScanReport
	if err := json.Unmarshal([]byte(saida), &rel); err != nil {
		t.Fatalf("stdout tem de ser só o JSON: %v\n%s", err, saida)
	}
	if prov.chamadas != 0 || rel.Recommendations != "" || len(rel.Findings) != len(achadosDeExemplo()) {
		t.Errorf("a IA foi chamada %d vezes; relatório: %+v", prov.chamadas, rel)
	}
	if strings.Contains(erros, "recomendações da IA") {
		t.Errorf("sem IA, a etapa das recomendações não aparece:\n%s", erros)
	}
}

// TestInvestigate_JSONInvalidoEmPTBR: a causa de uma resposta que não é JSON sai em PT-BR;
// o erro do encoding/json fica no log de depuração.
func TestInvestigate_JSONInvalidoEmPTBR(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	pod, ev := podComProblema()
	usarDependencias(t, fake.NewSimpleClientset(pod, ev), nil, nil, &provedorFalso{resposta: "Desculpe, não consegui"})

	_, _, err := rodar(t, "investigate", "worker-5d8b6-q7tvn", "-n", "payments")
	if !errors.Is(err, ai.ErrRespostaNaoEJSON) {
		t.Fatalf("esperava a causa em PT-BR, obtido %v", err)
	}
	var b bytes.Buffer
	sairComErro(&b, err)
	if strings.Contains(b.String(), "invalid character") || !strings.Contains(b.String(), "a IA respondeu fora do formato esperado") {
		t.Errorf("a causa na tela é em PT-BR:\n%s", b.String())
	}
}

// TestInvestigate_PodInexistenteSemCausaEmIngles: o título já diz que o pod não existe; a
// causa 'pods "x" not found' do Kubernetes não aparece.
func TestInvestigate_PodInexistenteSemCausaEmIngles(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	usarDependencias(t, fake.NewSimpleClientset(), nil, nil, nil)

	_, _, err := rodar(t, "investigate", "worker-nao-existe", "-n", "payments")
	var b bytes.Buffer
	sairComErro(&b, err)
	if strings.Contains(b.String(), "not found") || !strings.Contains(b.String(), "o pod worker-nao-existe não existe no namespace payments") {
		t.Errorf("pod inexistente com mensagem própria, sem a causa em inglês:\n%s", b.String())
	}
}

// negadoNoRBAC é o 403 que o Polaris devolve, embrulhado, sem permissão para listar os
// deployments.
func negadoNoRBAC() error {
	return fmt.Errorf("falha ao executar auditoria polaris: %w", apierrors.NewForbidden(
		schema.GroupResource{Group: "apps", Resource: "deployments"}, "",
		errors.New(`User "dev" cannot list resource "deployments" in API group "apps" in the namespace "payments"`)))
}

func TestMotivoNaTela(t *testing.T) {
	casos := []struct {
		err      error
		esperado string
	}{
		{negadoNoRBAC(), "sem permissão para listar os deployments"},
		{fmt.Errorf("falha ao listar ClusterRoles: %w", proibido("clusterroles")), "sem permissão para acessar os clusterroles"},
		{errors.New(`deployments.apps is forbidden: sem detalhes`), "sem permissão para acessar os deployments"},
		{fmt.Errorf("falha ao listar ClusterRoleBindings: %w", recusado), "o cluster parou de responder"},
		{apierrors.NewUnauthorized("token expirado"), "o cluster recusou as credenciais do kubeconfig"},
		{context.DeadlineExceeded, "o cluster parou de responder"},
		{errors.New("algo inesperado"), dicaDoLogDoPlugin + " para ver o motivo"},
	}
	for _, c := range casos {
		if got := motivoNaTela(c.err); got != c.esperado {
			t.Errorf("motivoNaTela(%v) = %q, esperado %q", c.err, got, c.esperado)
		}
	}
}

// TestScan_BackendSemPermissao: o polaris barrado no RBAC mostra o motivo em PT-BR, sem o
// 403 cru, e o scan sem achados não diz "nenhum problema" como se tudo tivesse rodado.
func TestScan_BackendSemPermissao(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	bs := []backends.SecurityBackend{backendFalso{nome: "polaris", err: negadoNoRBAC()}, backendFalso{nome: "opa"}}
	usarDependencias(t, fake.NewSimpleClientset(), bs, []checks.SecurityCheck{checkFalso{}}, nil)
	log := logNoNivelInfo(t)

	saida, erros, err := rodar(t, "scan", "-n", "payments")
	if err != nil {
		t.Fatalf("o scan rodou em parte e sai com 0: %v", err)
	}
	for _, trecho := range []string{"✕ polaris", "    sem permissão para listar os deployments", "▲ nenhum problema nas verificações que rodaram · 1 não rodou"} {
		if !strings.Contains(saida, trecho) {
			t.Errorf("faltou %q:\n%s", trecho, saida)
		}
	}
	for _, vedado := range []string{"forbidden", "User", "nenhum problema de segurança encontrado", "✓ nenhum"} {
		if strings.Contains(saida, vedado) || strings.Contains(erros, vedado) {
			t.Errorf("%q não pode aparecer:\n%s", vedado, saida)
		}
	}
	if log.Len() > 0 {
		t.Errorf("o erro cru fica no log de depuração:\n%s", log.String())
	}
}

// TestScan_SemAchadosComFalhas: sem achados, a falha parcial de um backend e os checks
// internos que falharam também viram aviso, com quantas verificações não rodaram.
func TestScan_SemAchadosComFalhas(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	t.Run("falha parcial", func(t *testing.T) {
		parcial := backendFalso{nome: "opa", err: &backends.FalhaParcial{Falhas: []string{"acesso a secrets", "wildcards"}, Causa: proibido("clusterroles")}}
		usarDependencias(t, fake.NewSimpleClientset(), []backends.SecurityBackend{parcial}, []checks.SecurityCheck{checkFalso{}}, nil)
		saida, _, err := rodar(t, "scan", "-n", "payments")
		if err != nil || !strings.Contains(saida, "▲ nenhum problema nas verificações que rodaram · 2 não rodaram") || strings.Contains(saida, "✓ nenhum") {
			t.Errorf("falha parcial sem achados vira aviso (err=%v):\n%s", err, saida)
		}
	})
	t.Run("checks internos", func(t *testing.T) {
		usarDependencias(t, fake.NewSimpleClientset(), nil, []checks.SecurityCheck{checkFalso{}, checkFalso{err: recusado}}, nil)
		saida, _, err := rodar(t, "scan", "-n", "payments")
		if err != nil || !strings.Contains(saida, "▲ 1 check falhou: falso") || !strings.Contains(saida, "▲ nenhum problema nas verificações que rodaram · 1 não rodou") {
			t.Errorf("check que falhou sem achados vira aviso (err=%v):\n%s", err, saida)
		}
	})
}

// TestScan_IAIndisponivelEmPTBR: a falha da IA aparece em PT-BR embaixo da etapa.
func TestScan_IAIndisponivelEmPTBR(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	for _, c := range []struct {
		err      error
		esperado string
	}{
		{errors.New(`Post "http://localhost:11434/api/chat": dial tcp [::1]:11434: connect: connection refused`), "o provider de IA não respondeu"},
		{errors.New("status 404: model not found"), dicaDoLogDoPlugin + " para ver o motivo"},
	} {
		usarDependencias(t, fake.NewSimpleClientset(), backendsDeExemplo(), nil, &provedorFalso{err: c.err})
		saida, _, err := rodar(t, "scan", "-n", "payments")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(saida, "✕ recomendações da IA") || !strings.Contains(saida, "    "+c.esperado) || strings.Contains(saida, c.err.Error()) {
			t.Errorf("a falha da IA sai em PT-BR (%q):\n%s", c.esperado, saida)
		}
	}
}

// usarLarguraTerminal finge um terminal de larg colunas para as linhas que o scan quebra
// por conta própria.
func usarLarguraTerminal(t *testing.T, larg int) {
	t.Helper()
	anterior := larguraTerminal
	t.Cleanup(func() { larguraTerminal = anterior })
	larguraTerminal = func(io.Writer) int { return larg }
}

// TestScan_DescricaoQuebraNaColuna: com a coluna alargada por um recurso longo do OPA, a
// descrição longa quebra dentro da coluna dela, com as continuações sob o começo da
// descrição, e nenhuma linha passa da largura do terminal.
func TestScan_DescricaoQuebraNaColuna(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	for _, larg := range []int{120, 100} {
		usarLarguraTerminal(t, larg)
		polaris := backendFalso{nome: "polaris", achados: []backends.Finding{
			{ID: "polaris/linuxHardening", Severity: "medium", Category: "pod-security", Resource: "Deployment/api (+5)", Message: "x"},
			{ID: "polaris/metadataAndInstanceMismatched", Severity: "medium", Category: "pod-security", Resource: "Deployment/web", Message: "y"},
		}}
		opa := backendFalso{nome: "opa", achados: []backends.Finding{
			{ID: "opa/rbac_wildcard", Severity: "high", Category: "rbac", Resource: "ClusterRole/local-path-provisioner-role", Message: "ClusterRole com curinga"},
		}}
		usarDependencias(t, fake.NewSimpleClientset(), []backends.SecurityBackend{polaris, opa}, nil, nil)

		saida, _, err := rodar(t, "scan", "-n", "payments")
		if err != nil {
			t.Fatal(err)
		}
		linhas := strings.Split(saida, "\n")
		achou := false
		for i, l := range linhas {
			if lipgloss.Width(l) > larg {
				t.Errorf("%d colunas: a linha passa da largura (%d):\n%s", larg, lipgloss.Width(l), l)
			}
			if !strings.HasPrefix(l, "  ▲ Deployment/") {
				continue
			}
			achou = true
			inicio := strings.Index(l, "médio  ") + len("médio  ")
			recuo := lipgloss.Width(l[:inicio])
			if i+1 < len(linhas) && strings.HasPrefix(linhas[i+1], "     ") {
				prox := linhas[i+1]
				if lipgloss.Width(prox)-lipgloss.Width(strings.TrimLeft(prox, " ")) != recuo {
					t.Errorf("%d colunas: a continuação fica sob o começo da descrição:\n%s\n%s", larg, l, prox)
				}
			} else if strings.Contains(l, "Restrinja") {
				t.Errorf("%d colunas: a descrição do linuxHardening quebra na coluna:\n%s", larg, saida)
			}
			if !strings.Contains(l[inicio:], "Restrinja") && !strings.Contains(l[inicio:], "O rótulo") {
				t.Errorf("%d colunas: a descrição começa na linha do recurso:\n%s", larg, l)
			}
		}
		if !achou {
			t.Fatalf("%d colunas: faltaram os achados do Polaris:\n%s", larg, saida)
		}
	}
}

// TestScan_RelatorioComTil confere, sem a normalização de rodar, que o caminho do
// relatório sai com ~ e barras normais em qualquer sistema.
func TestScan_RelatorioComTil(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	usarDependencias(t, fake.NewSimpleClientset(), backendsDeExemplo(), nil, nil)

	var saida, erros bytes.Buffer
	if err := comando(context.Background(), []string{"scan", "-n", "payments"}, &saida, &erros); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saida.String(), "relatório  ~/.yby/reports/sentinel-scan-payments-2026-10-25.md") || strings.Contains(saida.String(), homeDeTeste) {
		t.Errorf("o relatório sai como ~/.yby/reports/…:\n%s", saida.String())
	}
}

// TestScan_CaminhoDoArquivoQuebraNosSeparadores: com -f, um caminho mais largo que o
// terminal desce para baixo do nome e quebra só nos separadores.
func TestScan_CaminhoDoArquivoQuebraNosSeparadores(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	usarDependencias(t, fake.NewSimpleClientset(), backendsDeExemplo(), nil, nil)
	usarLarguraTerminal(t, 40)
	pasta := filepath.Join(t.TempDir(), "relatorios-de-seguranca", "cluster-de-producao")
	if err := os.MkdirAll(pasta, 0o755); err != nil {
		t.Fatal(err)
	}
	arquivo := filepath.Join(pasta, "scan.json")

	var saida, erros bytes.Buffer
	if err := comando(context.Background(), []string{"scan", "-n", "payments", "-o", "json", "-f", arquivo}, &saida, &erros); err != nil {
		t.Fatal(err)
	}
	linhas := strings.Split(strings.TrimRight(saida.String(), "\n"), "\n")
	i := len(linhas) - 1
	for i >= 0 && strings.HasPrefix(linhas[i], "    ") {
		i--
	}
	if i < 0 || strings.TrimRight(linhas[i], " ") != "  ✓ relatório   json" {
		t.Fatalf("a linha do relatório fica sem o caminho, que desce:\n%s", saida.String())
	}
	var partes []string
	for j, l := range linhas[i+1:] {
		p := strings.TrimPrefix(l, "    ")
		if j < len(linhas[i+1:])-1 && !strings.HasSuffix(p, "/") {
			t.Errorf("a linha quebra só depois de um separador: %q", l)
		}
		partes = append(partes, p)
	}
	if len(partes) < 2 || strings.Join(partes, "") != filepath.ToSlash(arquivo) {
		t.Errorf("o caminho inteiro, em mais de uma linha, com barras normais:\n%s", saida.String())
	}
}
