//go:build k8s

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/pkg/ai"
	yerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/backends"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/checks"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// homeDeTeste é a home dos testes do pacote: o relatório (~/.yby/reports) e o cache
// (~/.yby/sentinel/cache) nunca caem na home de verdade.
var homeDeTeste string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "sentinel-home-")
	if err != nil {
		panic(err)
	}
	homeDeTeste = dir
	os.Setenv("HOME", dir)
	os.Setenv("USERPROFILE", dir)
	os.Setenv("KUBECONFIG", filepath.Join(dir, "kubeconfig-inexistente"))
	for _, k := range []string{"YBY_THEME", "YBY_ASCII", "NO_COLOR", "YBY_LOG_LEVEL", "YBY_LOG_FORMAT"} {
		os.Unsetenv(k)
	}
	codigo := m.Run()
	os.RemoveAll(dir)
	os.Exit(codigo)
}

// quandoGolden é o instante fixo das etapas e do nome do relatório nas goldens.
var quandoGolden = time.Date(2026, 10, 25, 9, 30, 0, 0, time.UTC)

// relogioDePasso devolve um relógio que avança 1,5 s a cada leitura a partir de
// quandoGolden: a coluna de tempo das etapas entra nas goldens sempre igual, e o dia do
// nome do relatório não muda.
func relogioDePasso() func() time.Time {
	var mu sync.Mutex
	agora := quandoGolden
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		agora = agora.Add(1500 * time.Millisecond)
		return agora
	}
}

// usarVisual fixa o modo visual e o perfil de cor, e os restaura no fim do teste.
func usarVisual(t *testing.T, o ui.Opcoes, perfil termenv.Profile) {
	t.Helper()
	perfilAnterior, opcoesAnteriores := lipgloss.ColorProfile(), ui.Atual()
	escuroAnterior := lipgloss.HasDarkBackground()
	t.Cleanup(func() {
		ui.Configurar(opcoesAnteriores)
		lipgloss.SetColorProfile(perfilAnterior)
		lipgloss.SetHasDarkBackground(escuroAnterior)
	})
	ui.Configurar(o)
	lipgloss.SetColorProfile(perfil)
}

type backendFalso struct {
	nome    string
	achados []backends.Finding
	err     error
}

func (b backendFalso) Name() string      { return b.nome }
func (b backendFalso) IsAvailable() bool { return true }
func (b backendFalso) ScanCluster(context.Context, kubernetes.Interface, string) ([]backends.Finding, error) {
	return b.achados, b.err
}

type checkFalso struct {
	achados []checks.SecurityFinding
	err     error
}

func (c checkFalso) ID() string                { return "FALSO" }
func (c checkFalso) Name() string              { return "falso" }
func (c checkFalso) Category() checks.Category { return checks.CategoryPodSecurity }
func (c checkFalso) Severity() checks.Severity { return checks.SeverityHigh }
func (c checkFalso) Description() string       { return "check de teste" }
func (c checkFalso) Run(context.Context, kubernetes.Interface, string) ([]checks.SecurityFinding, error) {
	return c.achados, c.err
}

// provedorFalso responde sempre o mesmo texto, conta as chamadas e guarda o que recebeu
// de contexto; nunca há IA de verdade.
type provedorFalso struct {
	ai.Provider
	resposta  string
	err       error
	chamadas  int
	recebidos []string
}

func (p *provedorFalso) Name() string { return "falso" }
func (p *provedorFalso) Completion(_ context.Context, _, contexto string) (string, error) {
	p.chamadas++
	p.recebidos = append(p.recebidos, contexto)
	return p.resposta, p.err
}

// usarDependencias troca o cluster, os backends, os checks e a IA por falsos, fixa o
// relógio, a largura (64) e a detecção de terminal (fora dele), e restaura tudo no fim.
func usarDependencias(t *testing.T, client kubernetes.Interface, bs []backends.SecurityBackend, cs []checks.SecurityCheck, prov *provedorFalso) {
	t.Helper()
	k, b, c, p, r, te, l := novoClienteKube, backendsDoScan, checksDoScan, provedorDeIA, relogio, terminalEm, larguraSaida
	t.Cleanup(func() {
		novoClienteKube, backendsDoScan, checksDoScan, provedorDeIA, relogio, terminalEm, larguraSaida = k, b, c, p, r, te, l
		os.RemoveAll(filepath.Join(homeDeTeste, ".yby"))
	})
	// Os namespaces dos testes existem no cluster falso: o scan confere o namespace antes
	// das etapas.
	if fc, ok := client.(*fake.Clientset); ok {
		for _, ns := range []string{"payments", "default"} {
			_ = fc.Tracker().Add(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
		}
	}
	novoClienteKube = func() (kubernetes.Interface, error) { return client, nil }
	backendsDoScan = func() []backends.SecurityBackend { return bs }
	checksDoScan = func() []checks.SecurityCheck { return cs }
	provedorDeIA = func(context.Context) ai.Provider {
		if prov == nil {
			return nil
		}
		return prov
	}
	relogio = relogioDePasso()
	terminalEm = func(io.Writer) bool { return false }
	larguraSaida = func(io.Writer) int { return larguraPadrao }
}

// backendsDeExemplo divide os achados de exemplo entre um Polaris e um OPA falsos.
func backendsDeExemplo() []backends.SecurityBackend {
	polaris, opa := backendFalso{nome: "polaris"}, backendFalso{nome: "opa"}
	for _, f := range achadosDeExemplo() {
		bf := backends.Finding{ID: f.CheckID, Severity: string(f.Severity), Category: string(f.Category), Resource: f.Resource, Namespace: f.Namespace, Message: f.Message, Recommendation: f.Recommendation}
		if strings.HasPrefix(f.CheckID, "opa/") {
			opa.achados = append(opa.achados, bf)
		} else {
			polaris.achados = append(polaris.achados, bf)
		}
	}
	return []backends.SecurityBackend{polaris, opa}
}

// normalizarHome troca a home dos testes por ~ e as barras do Windows, para a golden ser
// a mesma em qualquer sistema.
func normalizarHome(s string) string {
	s = strings.ReplaceAll(s, homeDeTeste, "~")
	return strings.ReplaceAll(s, `\`, "/")
}

// rodar chama o subcomando como o core o chamaria e devolve stdout e stderr.
func rodar(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var saida, erros bytes.Buffer
	err := comando(context.Background(), args, &saida, &erros)
	return normalizarHome(saida.String()), normalizarHome(erros.String()), err
}

func TestScan_Golden(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.TrueColor)
	usarDependencias(t, fake.NewSimpleClientset(), backendsDeExemplo(), nil, &provedorFalso{resposta: "1. Tire o cluster-admin do ci-deployer."})

	saida, erros, err := rodar(t, "scan", "-n", "payments")
	if err != nil {
		t.Fatal(err)
	}
	if erros != "" {
		t.Errorf("nada no stderr no formato terminal, obtido %q", erros)
	}
	conferirGolden(t, "scan.golden", saida)

	// o relatório completo continua no formato de sempre
	md, err := os.ReadFile(filepath.Join(homeDeTeste, ".yby", "reports", "sentinel-scan-payments-2026-10-25.md"))
	if err != nil {
		t.Fatalf("relatório não foi salvo: %v", err)
	}
	if !strings.HasPrefix(string(md), "# Relatório de Segurança Sentinel — payments\n\n**Resumo:** 2 críticos, 1 alto, 1 médio, 1 baixo\n") {
		t.Errorf("relatório fora do formato:\n%s", md)
	}
}

func TestScan_GoldenASCIISemCor(t *testing.T) {
	usarVisual(t, ui.Opcoes{}, termenv.TrueColor)
	configurarModoVisual(func(k string) string {
		return map[string]string{"YBY_ASCII": "1", "NO_COLOR": "1", "YBY_THEME": "dark"}[k]
	})
	if o := ui.Atual(); !o.ASCII || !o.SemCor {
		t.Fatalf("YBY_ASCII e NO_COLOR valem no plugin: %+v", o)
	}
	usarDependencias(t, fake.NewSimpleClientset(), backendsDeExemplo(), nil, nil)

	saida, _, err := rodar(t, "scan", "-n", "payments")
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(saida, "◆✓✕▲○─") {
		t.Errorf("YBY_ASCII=1 sem glifos Unicode:\n%s", saida)
	}
	conferirGolden(t, "scan-ascii.golden", saida)
}

func TestScan_JSONNoStdoutSemTituloNemEtapas(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	usarDependencias(t, fake.NewSimpleClientset(), backendsDeExemplo(), nil, nil)

	saida, erros, err := rodar(t, "scan", "-n", "payments", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	var rel ScanReport
	if err := json.Unmarshal([]byte(saida), &rel); err != nil {
		t.Fatalf("stdout tem de ser só o JSON: %v\n%s", err, saida)
	}
	if rel.Namespace != "payments" || len(rel.Findings) != 5 || strings.Join(rel.Sources, ",") != "polaris,opa" {
		t.Errorf("relatório JSON inesperado: %+v", rel)
	}
	if !strings.Contains(erros, "✓ polaris") || strings.Contains(erros, "YBY") {
		t.Errorf("as etapas vão para o stderr, sem título:\n%s", erros)
	}
	if _, err := os.Stat(filepath.Join(homeDeTeste, ".yby", "reports")); err == nil {
		t.Error("com -o json o relatório global não é gravado")
	}
}

func TestScan_JSONEmArquivo(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	usarDependencias(t, fake.NewSimpleClientset(), backendsDeExemplo(), nil, nil)
	arquivo := filepath.Join(t.TempDir(), "scan.json")

	saida, _, err := rodar(t, "scan", "-n", "payments", "-o", "json", "-f", arquivo)
	if err != nil {
		t.Fatal(err)
	}
	dados, err := os.ReadFile(arquivo)
	if err != nil {
		t.Fatal(err)
	}
	var rel ScanReport
	if err := json.Unmarshal(dados, &rel); err != nil || len(rel.Findings) != 5 {
		t.Errorf("arquivo JSON inesperado (%v): %s", err, dados)
	}
	if !strings.Contains(saida, "◆ YBY  SENTINEL") || !strings.Contains(saida, "✓ relatório") {
		t.Errorf("com -f, o título, as etapas e a confirmação saem na tela:\n%s", saida)
	}
}

func TestScan_ChecksInternosComDryRun(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	achado := checks.SecurityFinding{
		CheckID: "POD_ROOT_CONTAINER", Severity: checks.SeverityCritical, Type: "critical",
		Category: checks.CategoryPodSecurity, Pod: "api-7c9f", Container: "app", Namespace: "payments",
		Resource: "api-7c9f/app", Message: "Container 'app' no pod 'api-7c9f' roda como root (UID 0)",
	}
	usarDependencias(t, fake.NewSimpleClientset(), nil, []checks.SecurityCheck{checkFalso{achados: []checks.SecurityFinding{achado}}}, nil)

	saida, _, err := rodar(t, "scan", "-n", "payments", "--fix-dry-run")
	if err != nil {
		t.Fatal(err)
	}
	conferirGolden(t, "scan-dry-run.golden", saida)
}

func TestScan_SemAchados(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	prov := &provedorFalso{resposta: "nada"}
	usarDependencias(t, fake.NewSimpleClientset(), nil, []checks.SecurityCheck{checkFalso{}}, prov)

	saida, _, err := rodar(t, "scan")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saida, "scan · namespace default") || !strings.Contains(saida, "✓ nenhum problema de segurança encontrado") {
		t.Errorf("saída inesperada:\n%s", saida)
	}
	if prov.chamadas != 0 {
		t.Error("sem achados, a IA não é chamada")
	}
}

func TestScan_ClusterSemResposta(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	falha := errors.New("dial tcp 127.0.0.1:6443: connect: connection refused")
	usarDependencias(t, fake.NewSimpleClientset(),
		[]backends.SecurityBackend{backendFalso{nome: "polaris", err: falha}},
		[]checks.SecurityCheck{checkFalso{err: falha}}, nil)

	saida, _, err := rodar(t, "scan", "-n", "payments")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeClusterOffline || ye.Hint == "" {
		t.Fatalf("esperava ERR_CLUSTER_OFFLINE com hint, obtido %v", err)
	}
	if !strings.Contains(saida, "✕ polaris") || !strings.Contains(saida, "o cluster parou de responder") || strings.Contains(saida, "connection refused") {
		t.Errorf("a etapa que falhou aparece com o motivo em PT-BR, sem o erro cru:\n%s", saida)
	}
}

func TestScan_PerfilInexistente(t *testing.T) {
	usarDependencias(t, fake.NewSimpleClientset(), nil, nil, nil)
	_, _, err := rodar(t, "scan", "--profile", "iso-27001")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeValidation || !strings.Contains(ye.Hint, "cis-l1") {
		t.Fatalf("esperava ERR_VALIDATION com os perfis no hint, obtido %v", err)
	}
}

// podComProblema é o worker em CrashLoopBackOff da prancha, com o evento de BackOff.
func podComProblema() (*corev1.Pod, *corev1.Event) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-5d8b6-q7tvn", Namespace: "payments"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "worker", RestartCount: 12,
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
			}},
		},
	}
	ev := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "worker.1", Namespace: "payments"},
		InvolvedObject: corev1.ObjectReference{Name: pod.Name, Namespace: "payments"},
		Type:           "Warning", Reason: "BackOff", Message: "Back-off restarting failed container",
	}
	return pod, ev
}

const respostaDaIA = "```json\n" + `{
  "root_cause": "O worker não encontra o banco de dados",
  "technical_detail": "DB_HOST aponta para localhost; o contêiner sai com código 1 ao abrir a conexão e o kubelet o reinicia a cada 30 s.",
  "confidence": 82,
  "suggested_fix": "Aponte DB_HOST para o Service do Postgres no namespace payments.",
  "kubectl_patch": "kubectl -n payments set env deploy/worker DB_HOST=postgres.payments.svc"
}` + "\n```"

func TestInvestigate_Golden(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.TrueColor)
	pod, ev := podComProblema()
	prov := &provedorFalso{resposta: respostaDaIA}
	usarDependencias(t, fake.NewSimpleClientset(pod, ev), nil, nil, prov)

	saida, erros, err := rodar(t, "investigate", "worker-5d8b6-q7tvn", "-n", "payments")
	if err != nil {
		t.Fatal(err)
	}
	if erros != "" {
		t.Errorf("nada no stderr no formato terminal, obtido %q", erros)
	}
	conferirGolden(t, "investigate.golden", saida)

	// a segunda vez vem da análise guardada, sem chamar a IA
	saida, _, err = rodar(t, "investigate", "worker-5d8b6-q7tvn", "-n", "payments")
	if err != nil {
		t.Fatal(err)
	}
	if prov.chamadas != 1 || !strings.Contains(saida, "análise guardada") {
		t.Errorf("esperava o cache na segunda vez (chamadas=%d):\n%s", prov.chamadas, saida)
	}
	if _, _, err := rodar(t, "investigate", "worker-5d8b6-q7tvn", "-n", "payments", "--no-cache"); err != nil || prov.chamadas != 2 {
		t.Errorf("--no-cache pergunta à IA de novo (chamadas=%d, err=%v)", prov.chamadas, err)
	}
}

func TestInvestigate_GoldenASCIISemCor(t *testing.T) {
	usarVisual(t, ui.Opcoes{}, termenv.TrueColor)
	configurarModoVisual(func(k string) string {
		return map[string]string{"YBY_ASCII": "1", "NO_COLOR": "1", "YBY_THEME": "dark"}[k]
	})
	pod, ev := podComProblema()
	usarDependencias(t, fake.NewSimpleClientset(pod, ev), nil, nil, &provedorFalso{resposta: respostaDaIA})

	saida, _, err := rodar(t, "investigate", "worker-5d8b6-q7tvn", "-n", "payments", "--no-cache")
	if err != nil {
		t.Fatal(err)
	}
	conferirGolden(t, "investigate-ascii.golden", saida)
}

func TestInvestigate_MarkdownNoStdout(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	pod, ev := podComProblema()
	usarDependencias(t, fake.NewSimpleClientset(pod, ev), nil, nil, &provedorFalso{resposta: respostaDaIA})

	saida, erros, err := rodar(t, "investigate", "worker-5d8b6-q7tvn", "-n", "payments", "-o", "markdown", "--no-cache")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(saida, "# Relatório Sentinel — payments/worker-5d8b6-q7tvn\n") {
		t.Errorf("stdout tem de ser só o markdown:\n%s", saida)
	}
	if !strings.Contains(erros, "✓ análise da IA") {
		t.Errorf("as etapas vão para o stderr:\n%s", erros)
	}
}

func TestInvestigate_PodSaudavelNaoChamaIA(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-7c9f", Namespace: "payments"},
		Status: corev1.PodStatus{
			Phase:             corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{Name: "app", Ready: true}},
		},
	}
	prov := &provedorFalso{resposta: respostaDaIA}
	usarDependencias(t, fake.NewSimpleClientset(pod), nil, nil, prov)

	saida, _, err := rodar(t, "investigate", "api-7c9f", "-n", "payments")
	if err != nil {
		t.Fatal(err)
	}
	if prov.chamadas != 0 {
		t.Error("pod saudável não chama a IA")
	}
	for _, trecho := range []string{"✓ pod saudável", "Running", "CONTÊINERES", "✓ app", "pronto", "0 reinícios"} {
		if !strings.Contains(saida, trecho) {
			t.Errorf("faltou %q:\n%s", trecho, saida)
		}
	}
}

func TestInvestigate_PodInexistente(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	prov := &provedorFalso{resposta: respostaDaIA}
	usarDependencias(t, fake.NewSimpleClientset(), nil, nil, prov)

	saida, _, err := rodar(t, "investigate", "nao-existe", "-n", "payments")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeValidation || !strings.Contains(ye.Message, "não existe") {
		t.Fatalf("esperava ERR_VALIDATION de pod inexistente, obtido %v", err)
	}
	// nem o título sai: o erro de uma verificação rápida não fica separado dele por duas
	// linhas em branco
	if saida != "" || prov.chamadas != 0 {
		t.Errorf("pod inexistente para antes do título, das etapas e da IA:\n%s", saida)
	}
}

// clienteQueFalha devolve um clientset falso cujo get de pods falha com err.
func clienteQueFalha(err error) *fake.Clientset {
	c := fake.NewSimpleClientset()
	c.PrependReactor("get", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, err
	})
	return c
}

func TestInvestigate_ClusterSemResposta(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	prov := &provedorFalso{resposta: respostaDaIA}
	usarDependencias(t, clienteQueFalha(errors.New("dial tcp 127.0.0.1:6443: connect: connection refused")), nil, nil, prov)

	saida, _, err := rodar(t, "investigate", "worker-5d8b6-q7tvn", "-n", "payments")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeClusterOffline || !strings.Contains(ye.Hint, "kubectl get pods -n payments") {
		t.Fatalf("esperava ERR_CLUSTER_OFFLINE com o hint do cluster, obtido %v", err)
	}
	if saida != "" || prov.chamadas != 0 {
		t.Errorf("com o cluster fora do ar, nada de título, etapas ou IA:\n%s", saida)
	}
}

func TestInvestigate_SemPermissao(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	proibido := apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "worker-5d8b6-q7tvn", errors.New("RBAC"))
	usarDependencias(t, clienteQueFalha(proibido), nil, nil, nil)

	_, _, err := rodar(t, "investigate", "worker-5d8b6-q7tvn", "-n", "payments")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeValidation || !strings.Contains(ye.Hint, "kubectl auth can-i get pods -n payments") {
		t.Fatalf("esperava ERR_VALIDATION de permissão, obtido %v", err)
	}
}

// podSaudavel é um pod Running com o contêiner pronto.
func podSaudavel() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-7c9f", Namespace: "payments"},
		Status: corev1.PodStatus{
			Phase:             corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{Name: "app", Ready: true}},
		},
	}
}

func TestInvestigate_PodSaudavelNosFormatosParaScripts(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	prov := &provedorFalso{resposta: respostaDaIA}
	usarDependencias(t, fake.NewSimpleClientset(podSaudavel()), nil, nil, prov)

	saida, erros, err := rodar(t, "investigate", "api-7c9f", "-n", "payments", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	var rel HealthyReport
	if err := json.Unmarshal([]byte(saida), &rel); err != nil {
		t.Fatalf("stdout tem de ser um JSON: %v\n%q", err, saida)
	}
	if !rel.Healthy || rel.Phase != "Running" || rel.Metadata.Pod != "api-7c9f" || rel.Metadata.Namespace != "payments" {
		t.Errorf("relatório de pod saudável inesperado: %+v", rel)
	}
	if !strings.Contains(erros, "✓ pod saudável") {
		t.Errorf("a mensagem de pod saudável vai para o stderr:\n%s", erros)
	}

	saida, _, err = rodar(t, "investigate", "api-7c9f", "-n", "payments", "-o", "markdown")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(saida, "# Relatório Sentinel — payments/api-7c9f\n") || !strings.Contains(saida, "saudável (Running)") {
		t.Errorf("stdout tem de ser o markdown de pod saudável:\n%s", saida)
	}
	if prov.chamadas != 0 {
		t.Error("pod saudável não chama a IA")
	}
}

func TestInvestigate_GoldenEstreito(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	pod, ev := podComProblema()
	usarDependencias(t, fake.NewSimpleClientset(pod, ev), nil, nil, &provedorFalso{resposta: respostaDaIA})
	larguraSaida = func(io.Writer) int { return 40 }

	saida, _, err := rodar(t, "investigate", "worker-5d8b6-q7tvn", "-n", "payments", "--no-cache")
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(saida, "\n") {
		// o comando sugerido fica inteiro, para copiar, e o título segue a largura que o
		// ui.Title mede no próprio terminal; o resto cabe nas 40 colunas
		if !strings.Contains(l, "kubectl") && !strings.HasPrefix(l, "◆") && lipgloss.Width(l) > 40 {
			t.Errorf("linha passa de 40 colunas (%d): %q", lipgloss.Width(l), l)
		}
	}
	conferirGolden(t, "investigate-estreito.golden", saida)
}

// backendQueLoga escreve um aviso no log no meio da etapa, como o OPA quando falta
// permissão de RBAC.
type backendQueLoga struct{}

func (backendQueLoga) Name() string      { return "opa" }
func (backendQueLoga) IsAvailable() bool { return true }
func (backendQueLoga) ScanCluster(context.Context, kubernetes.Interface, string) ([]backends.Finding, error) {
	slog.Warn("sem permissão para listar roles", "namespace", "payments")
	return nil, nil
}

func TestScan_NoTerminalRedesenhaEDesviaOLog(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	anterior := slog.Default()
	t.Cleanup(func() { slog.SetDefault(anterior) })
	configurarLog()
	usarDependencias(t, fake.NewSimpleClientset(), []backends.SecurityBackend{backendQueLoga{}}, []checks.SecurityCheck{checkFalso{}}, nil)
	terminalEm = func(io.Writer) bool { return true }

	saida, erros, err := rodar(t, "scan", "-n", "payments")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saida, "\x1b[1A\r\x1b[J") {
		t.Errorf("no terminal, a etapa atual é redesenhada no lugar:\n%q", saida)
	}
	for _, etapa := range []string{"✓ opa", "✓ checks internos"} {
		if n := strings.Count(saida, etapa); n != 1 {
			t.Errorf("%q sai uma vez só, de vez, obtido %d:\n%q", etapa, n, saida)
		}
	}
	// o log passa pelo escritor das etapas e vai para o stderr no formato do yby, fora do
	// trecho redesenhado
	if !strings.Contains(erros, "WARN") || !strings.Contains(erros, "sem permissão para listar roles") {
		t.Errorf("o aviso do log vai para o stderr pelo slog do yby:\n%q", erros)
	}
	if strings.Contains(saida, "sem permissão") {
		t.Errorf("o log não cai no meio das etapas:\n%q", saida)
	}
}

func TestScan_MarkdownNoStdout(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	usarDependencias(t, fake.NewSimpleClientset(), backendsDeExemplo(), nil, nil)

	saida, erros, err := rodar(t, "scan", "-n", "payments", "-o", "markdown")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(saida, "# Relatório de Segurança Sentinel — payments\n") || strings.Contains(saida, "YBY") {
		t.Errorf("stdout tem de ser só o markdown:\n%s", saida)
	}
	if !strings.Contains(erros, "✓ polaris") {
		t.Errorf("as etapas vão para o stderr:\n%s", erros)
	}
}

// achadoRoot é o achado dos checks internos que o --fix sabe corrigir.
var achadoRoot = checks.SecurityFinding{
	CheckID: "POD_ROOT_CONTAINER", Severity: checks.SeverityCritical, Type: "critical",
	Category: checks.CategoryPodSecurity, Pod: "api-7c9f", Container: "app", Namespace: "payments",
	Resource: "api-7c9f/app", Message: "Container 'app' no pod 'api-7c9f' roda como root (UID 0)",
}

func TestScan_FixQueFalhaSaiComErro(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	cs := []checks.SecurityCheck{checkFalso{achados: []checks.SecurityFinding{achadoRoot}}}
	usarDependencias(t, fake.NewSimpleClientset(), nil, cs, nil)

	saida, _, err := rodar(t, "scan", "-n", "payments", "--fix")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodePlugin || !strings.Contains(ye.Message, "1 de 1") || !strings.Contains(ye.Hint, "kubectl auth can-i") {
		t.Fatalf("patches que falharam saem como erro com hint, obtido %v", err)
	}
	for _, trecho := range []string{"✕ aplicar patches", "1 de 1 falharam", "not found", "RESUMO"} {
		if !strings.Contains(saida, trecho) {
			t.Errorf("faltou %q:\n%s", trecho, saida)
		}
	}
	if strings.Contains(saida, "REMEDIAÇÃO") {
		t.Errorf("com --fix, os patches aparecem só na etapa:\n%s", saida)
	}

	// com -o json, o JSON sai inteiro no stdout e o erro continua
	saida, _, err = rodar(t, "scan", "-n", "payments", "--fix", "-o", "json")
	var rel ScanReport
	if jerr := json.Unmarshal([]byte(saida), &rel); jerr != nil || len(rel.Findings) != 1 {
		t.Errorf("stdout tem de ser o JSON (%v):\n%s", jerr, saida)
	}
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodePlugin {
		t.Errorf("com -o json, a falha dos patches também sai como erro, obtido %v", err)
	}
}

func TestScan_FixAplicado(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-7c9f", Namespace: "payments"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
	}
	cs := []checks.SecurityCheck{checkFalso{achados: []checks.SecurityFinding{achadoRoot}}}
	usarDependencias(t, fake.NewSimpleClientset(pod), nil, cs, nil)

	saida, _, err := rodar(t, "scan", "-n", "payments", "--fix")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saida, "✓ aplicar patches   1 aplicado") {
		t.Errorf("a etapa mostra os patches aplicados:\n%s", saida)
	}
}

func TestScan_ClienteSemConfigNaoMostraTitulo(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	usarDependencias(t, nil, nil, nil, nil)
	novoClienteKube = func() (kubernetes.Interface, error) { return nil, errors.New("kubeconfig inválido") }

	saida, _, err := rodar(t, "scan", "-n", "payments")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeClusterOffline {
		t.Fatalf("esperava ERR_CLUSTER_OFFLINE, obtido %v", err)
	}
	if saida != "" {
		t.Errorf("o título só sai depois de montar o cliente:\n%s", saida)
	}
}

func TestScan_FlagComIgual(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	usarDependencias(t, fake.NewSimpleClientset(), nil, []checks.SecurityCheck{checkFalso{}}, nil)

	saida, _, err := rodar(t, "scan", "--namespace=payments")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saida, "scan · namespace payments") {
		t.Errorf("--namespace=payments vale como --namespace payments:\n%s", saida)
	}
	op, err := lerOpcoes("investigate", []string{"worker", "--output=json", "--file=w.json", "--no-cache"})
	if err != nil || op.pod != "worker" || op.formato != "json" || op.arquivo != "w.json" || !op.semCache {
		t.Errorf("flags com = no investigate: %+v (%v)", op, err)
	}
}

func TestInvestigate_SemProvider(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	pod, ev := podComProblema()
	usarDependencias(t, fake.NewSimpleClientset(pod, ev), nil, nil, nil)

	_, _, err := rodar(t, "investigate", "worker-5d8b6-q7tvn", "-n", "payments")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeConfig || ye.Hint == "" {
		t.Fatalf("esperava ERR_CONFIG_INVALID com hint, obtido %v", err)
	}
}

func TestInvestigate_RespostaInvalida(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	pod, ev := podComProblema()
	usarDependencias(t, fake.NewSimpleClientset(pod, ev), nil, nil, &provedorFalso{resposta: "não sei"})

	saida, _, err := rodar(t, "investigate", "worker-5d8b6-q7tvn", "-n", "payments")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodePlugin {
		t.Fatalf("esperava ERR_PLUGIN, obtido %v", err)
	}
	if !strings.Contains(saida, "✕ análise da IA") {
		t.Errorf("a etapa da IA aparece como falha:\n%s", saida)
	}
	// o --log-level do core não chega ao plugin: a dica aponta o YBY_LOG_LEVEL, e o começo
	// da resposta vai no contexto do erro
	if strings.Contains(ye.Hint, "--log-level") || !strings.Contains(ye.Hint, "YBY_LOG_LEVEL=debug") {
		t.Errorf("hint aponta um jeito de ver o log que funciona no plugin: %q", ye.Hint)
	}
	if ye.Context["resposta"] != "não sei" {
		t.Errorf("o começo da resposta vai no contexto do erro: %v", ye.Context)
	}
}

func TestInicioDaResposta(t *testing.T) {
	longa := strings.Repeat("a ", 100)
	got := inicioDaResposta("{\n  " + longa)
	if strings.Contains(got, "\n") || len([]rune(got)) != maxInicioResposta+1 || !strings.HasSuffix(got, "…") {
		t.Errorf("o começo da resposta fica numa linha e cortado: %q", got)
	}
}

func TestComando_Erros(t *testing.T) {
	usarDependencias(t, fake.NewSimpleClientset(), nil, nil, nil)
	casos := []struct {
		args []string
		dica string
	}{
		{[]string{"audit"}, "yby sentinel --help"},
		{[]string{"investigate"}, "yby sentinel investigate <pod>"},
		{[]string{"scan", "-o", "yaml"}, "terminal, json ou markdown"},
		{[]string{"scan", "-n"}, "yby sentinel scan --help"},
		{[]string{"scan", "--namespace="}, "yby sentinel scan --help"},
		{[]string{"scan", "--output=yaml"}, "terminal, json ou markdown"},
		{[]string{"scan", "--fix=true"}, "yby sentinel scan --help"},
	}
	for _, c := range casos {
		_, _, err := rodar(t, c.args...)
		var ye *yerrors.YbyError
		if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeValidation || !strings.Contains(ye.Hint, c.dica) {
			t.Errorf("%v: esperava ERR_VALIDATION com %q no hint, obtido %v", c.args, c.dica, err)
		}
	}
}

func TestSairComErro(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	var b bytes.Buffer
	codigo := sairComErro(&b, errors.New("explodiu"))
	if codigo != 1 {
		t.Errorf("código 1 na falha, obtido %d", codigo)
	}
	if !strings.Contains(b.String(), "✕ o Sentinel falhou") || !strings.Contains(b.String(), "ERR_PLUGIN") {
		t.Errorf("erro comum vira ERR_PLUGIN no formato do ui.WriteError:\n%s", b.String())
	}
	// o --log-level do core não chega ao plugin: nem o hint do registry nem a linha
	// "detalhes" podem mandar usá-lo
	for _, err := range []error{
		errors.New("explodiu"),
		yerrors.Wrap(errors.New("pods \"x\" not found"), yerrors.ErrCodeValidation, "o pod x não existe").WithHint("confira o nome"),
		yerrors.New(yerrors.ErrCodePlugin, "não deu para montar o relatório JSON"),
	} {
		b.Reset()
		sairComErro(&b, err)
		if strings.Contains(b.String(), "--log-level") || !strings.Contains(b.String(), "YBY_LOG_LEVEL=debug") {
			t.Errorf("a dica de depuração aponta o YBY_LOG_LEVEL:\n%s", b.String())
		}
	}
}

func TestAjuda_Golden(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	var b strings.Builder
	for _, args := range [][]string{{"--help"}, {"scan", "--help"}, {"investigate", "-h"}} {
		saida, _, err := rodar(t, args...)
		if err != nil {
			t.Fatal(err)
		}
		b.WriteString("$ yby sentinel " + strings.Join(args, " ") + "\n" + saida + "\n")
	}
	conferirGolden(t, "ajuda.golden", b.String())

	semArgs, _, _ := rodar(t)
	if !strings.HasPrefix(semArgs, "◆ YBY  SENTINEL") {
		t.Errorf("sem subcomando sai a ajuda:\n%s", semArgs)
	}
}
