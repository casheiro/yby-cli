//go:build k8s

package main

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/muesli/termenv"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	yerrors "github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
)

// TestInvestigate_SemIA_JSON: com --no-ai a IA não é chamada (nem o cache é lido) e o JSON
// traz os dados brutos da coleta: fase, contêineres, eventos e o fim dos logs.
func TestInvestigate_SemIA_JSON(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	podRuim, ev := podComProblema()
	prov := &provedorFalso{resposta: respostaDaIA}
	usarDependencias(t, fake.NewSimpleClientset(podRuim, ev), nil, nil, prov)

	// uma análise guardada não pode vencer o --no-ai
	if _, _, err := rodar(t, "investigate", "worker-5d8b6-q7tvn", "-n", "payments"); err != nil {
		t.Fatal(err)
	}
	prov.chamadas = 0

	saida, _, err := rodar(t, "investigate", "worker-5d8b6-q7tvn", "-n", "payments", "-o", "json", "--no-ai")
	if err != nil {
		t.Fatal(err)
	}
	if prov.chamadas != 0 {
		t.Errorf("--no-ai não chama a IA, chamou %d vez(es)", prov.chamadas)
	}
	var r DadosBrutos
	if err := json.Unmarshal([]byte(saida), &r); err != nil {
		t.Fatalf("JSON inválido: %v\n%s", err, saida)
	}
	if r.Metadata.Pod != "worker-5d8b6-q7tvn" || r.Metadata.Namespace != "payments" {
		t.Errorf("metadata: %+v", r.Metadata)
	}
	if r.Healthy || r.Phase != "Running" {
		t.Errorf("phase/healthy: %q %v", r.Phase, r.Healthy)
	}
	if len(r.Containers) != 1 || r.Containers[0].Name != "worker" || r.Containers[0].RestartCount != 12 ||
		r.Containers[0].Reason != "CrashLoopBackOff" {
		t.Errorf("contêineres: %+v", r.Containers)
	}
	if len(r.Events) != 1 || r.Events[0].Motivo != "BackOff" || r.Events[0].Mensagem == "" {
		t.Errorf("eventos: %+v", r.Events)
	}
	if r.Logs == "" {
		t.Errorf("o fim dos logs vai no JSON")
	}
	if strings.Contains(saida, "analysis") {
		t.Errorf("sem análise no JSON dos dados brutos:\n%s", saida)
	}
}

// TestInvestigate_SemIA_ContratoJSON fixa os nomes e os tipos das chaves do JSON do --no-ai,
// que o Bard lê: decodifica num map para que renomear uma tag quebre o teste.
func TestInvestigate_SemIA_ContratoJSON(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	podRuim, ev := podComProblema()
	usarDependencias(t, fake.NewSimpleClientset(podRuim, ev), nil, nil, nil)

	saida, _, err := rodar(t, "investigate", "worker-5d8b6-q7tvn", "-n", "payments", "-o", "json", "--no-ai")
	if err != nil {
		t.Fatal(err)
	}
	var raiz map[string]any
	if err := json.Unmarshal([]byte(saida), &raiz); err != nil {
		t.Fatalf("JSON inválido: %v\n%s", err, saida)
	}
	igual := func(onde string, m map[string]any, quer ...string) {
		t.Helper()
		var got []string
		for k := range m {
			got = append(got, k)
		}
		sort.Strings(got)
		sort.Strings(quer)
		if strings.Join(got, ",") != strings.Join(quer, ",") {
			t.Errorf("%s: chaves %v, esperado %v", onde, got, quer)
		}
	}
	igual("raiz", raiz, "metadata", "healthy", "phase", "containers", "events", "logs")

	meta, _ := raiz["metadata"].(map[string]any)
	igual("metadata", meta, "pod", "namespace", "timestamp")
	if meta["pod"] != "worker-5d8b6-q7tvn" || meta["namespace"] != "payments" {
		t.Errorf("metadata: %v", meta)
	}
	if ts, _ := meta["timestamp"].(string); ts == "" {
		t.Errorf("metadata.timestamp deve ser string RFC 3339: %v", meta["timestamp"])
	} else if _, err := time.Parse(time.RFC3339Nano, ts); err != nil {
		t.Errorf("metadata.timestamp: %v", err)
	}
	if raiz["healthy"] != false || raiz["phase"] != "Running" {
		t.Errorf("healthy/phase: %v %v", raiz["healthy"], raiz["phase"])
	}
	if l, ok := raiz["logs"].(string); !ok || l == "" {
		t.Errorf("logs deve ser string não vazia: %v", raiz["logs"])
	}

	conts, _ := raiz["containers"].([]any)
	if len(conts) != 1 {
		t.Fatalf("containers: %v", raiz["containers"])
	}
	c, _ := conts[0].(map[string]any)
	igual("containers[0]", c, "name", "ready", "restartCount", "state", "reason")
	if c["name"] != "worker" || c["ready"] != false || c["restartCount"] != float64(12) ||
		c["state"] != "waiting" || c["reason"] != "CrashLoopBackOff" {
		t.Errorf("containers[0]: %v", c)
	}

	evs, _ := raiz["events"].([]any)
	if len(evs) != 1 {
		t.Fatalf("events: %v", raiz["events"])
	}
	e, _ := evs[0].(map[string]any)
	if e["type"] != "Warning" || e["reason"] != "BackOff" || e["message"] == "" {
		t.Errorf("events[0]: %v", e)
	}
	if _, ok := e["count"].(float64); !ok {
		t.Errorf("events[0].count deve ser número: %v", e["count"])
	}
	for k := range e {
		switch k {
		case "type", "reason", "message", "count", "lastTimestamp":
		default:
			t.Errorf("events[0] com chave fora do contrato: %q", k)
		}
	}
}

// TestInvestigate_SemIA_Terminal: sem provider nenhum, o --no-ai ainda funciona e a tela
// mostra os dados coletados.
func TestInvestigate_SemIA_Terminal(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	podRuim, ev := podComProblema()
	usarDependencias(t, fake.NewSimpleClientset(podRuim, ev), nil, nil, nil)

	saida, _, err := rodar(t, "investigate", "worker-5d8b6-q7tvn", "-n", "payments", "--no-ai")
	if err != nil {
		t.Fatal(err)
	}
	for _, quer := range []string{"worker", "CrashLoopBackOff", "BackOff", "12 reinícios", "dados brutos"} {
		if !strings.Contains(saida, quer) {
			t.Errorf("falta %q na tela:\n%s", quer, saida)
		}
	}
}

// TestDadosBrutos_Redacao: segredos dos logs e das mensagens dos eventos não saem.
func TestDadosBrutos_Redacao(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "n"},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	evs := []corev1.Event{{
		Type: "Warning", Reason: "Failed", Count: 2,
		Message:       "pull de https://usuario:senhasecreta123@registro.exemplo/x falhou",
		LastTimestamp: metav1.NewTime(time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)),
	}}
	r := montarDadosBrutos(pod, evs, "Authorization: Bearer abcdef1234567890abcdef\n", false, "p", "n")
	b, _ := json.Marshal(r)
	for _, vaza := range []string{"senhasecreta123", "abcdef1234567890abcdef"} {
		if strings.Contains(string(b), vaza) {
			t.Errorf("%q vazou nos dados brutos: %s", vaza, b)
		}
	}
	if !strings.Contains(string(b), `"lastTimestamp":"2026-10-26T10:00:00Z"`) {
		t.Errorf("chave lastTimestamp ausente do JSON: %s", b)
	}
	if r.Events[0].UltimaOcorrencia != "2026-10-26T10:00:00Z" || r.Events[0].Contagem != 2 {
		t.Errorf("evento: %+v", r.Events[0])
	}
}

func TestLerOpcoes_NoAIInvestigate(t *testing.T) {
	op, err := lerOpcoes("investigate", []string{"pod", "--no-ai"})
	if err != nil || !op.semIA || op.pod != "pod" {
		t.Errorf("--no-ai no investigate: %+v (%v)", op, err)
	}
	_, err = lerOpcoes("investigate", []string{"pod", "--no-ai=x"})
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeValidation {
		t.Errorf("--no-ai=x devolve erro de validação: %v", err)
	}
}
