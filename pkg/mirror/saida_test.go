package mirror

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/casheiro/yby-cli/pkg/ui"
)

// saidaSegura é um buffer que aceita escrita do laço de sync em outra goroutine.
type saidaSegura struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *saidaSegura) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *saidaSegura) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// esperarTexto espera até a saída conter texto (ou 3 s).
func esperarTexto(t *testing.T, s *saidaSegura, texto string) {
	t.Helper()
	limite := time.Now().Add(3 * time.Second)
	for !strings.Contains(s.String(), texto) && time.Now().Before(limite) {
		time.Sleep(5 * time.Millisecond)
	}
	if !strings.Contains(s.String(), texto) {
		t.Fatalf("a saída não chegou a ter %q:\n%s", texto, s.String())
	}
}

// semCor deixa as linhas de ui.Steps com glifos Unicode durante o teste; sem terminal no
// go test, elas já saem sem cor.
func semCor(t *testing.T) {
	t.Helper()
	opcoes := ui.Atual()
	t.Cleanup(func() { ui.Configurar(opcoes) })
	ui.Configurar(ui.Opcoes{})
}

func rodarLaco(m *MirrorManager) (cancelar func()) {
	ctx, cancel := context.WithCancel(context.Background())
	pronto := make(chan struct{})
	go func() {
		m.StartSyncLoop(ctx)
		close(pronto)
	}()
	return func() {
		cancel()
		<-pronto
	}
}

func TestStartSyncLoop_LinhasDoSync(t *testing.T) {
	semCor(t)
	saida := &saidaSegura{}
	var mu sync.Mutex
	pushes := 0
	m := &MirrorManager{
		Namespace: "yby-system",
		Runner: &MockRunner{RunFunc: func(ctx context.Context, name string, args ...string) error {
			mu.Lock()
			defer mu.Unlock()
			pushes++
			if pushes == 1 {
				return nil // o sync inicial passa; o seguinte falha
			}
			// como o runner quieto do yby up devolve a falha do git push
			return errors.New("exit status 1: error: src refspec HEAD does not match any · error: failed to push some refs")
		}},
		localPort:      12345,
		healthInterval: time.Hour,
		syncInterval:   5 * time.Millisecond,
		Saida:          saida,
	}
	cancelar := rodarLaco(m)
	esperarTexto(t, saida, "tentativa 1")
	cancelar()

	// o motivo vem primeiro: Steps corta a nota na largura (64)
	esperado := "" +
		"  ◈ auto-sync   envia o código a cada 5 ms · ctrl+c encerra\n" +
		"  ✕ sync   src refspec HEAD does not match any · tentativa 1 · …\n"
	// FormatarDuracao arredonda 5 ms para 0,0 s; a linha do auto-sync usa o mesmo texto
	esperado = strings.Replace(esperado, "5 ms", ui.FormatarDuracao(5*time.Millisecond), 1)
	assert.Equal(t, esperado, saida.String())
	for _, emoji := range []string{"🔄", "✅", "❌", "⚠️"} {
		assert.NotContains(t, saida.String(), emoji)
	}
}

func TestStartSyncLoop_SyncInicialFalhaLinha(t *testing.T) {
	semCor(t)
	saida := &saidaSegura{}
	m := &MirrorManager{
		Runner:         &MockRunner{},
		healthInterval: time.Hour,
		Saida:          saida,
	}
	// sem túnel, o sync inicial falha
	cancelar := rodarLaco(m)
	esperarTexto(t, saida, "✕ sync")
	cancelar()
	assert.Contains(t, saida.String(), "  ✕ sync   o túnel até o git server não foi aberto · tenta de n…\n")
}

func TestStartSyncLoop_ReconexaoLinhas(t *testing.T) {
	semCor(t)
	saida := &saidaSegura{}
	m := &MirrorManager{
		Namespace:      "yby-system",
		Runner:         &MockRunner{},
		localPort:      12345,
		forwarder:      &mockForwarder{healthCheckFunc: func(context.Context) error { return errors.New("caiu") }},
		healthInterval: 5 * time.Millisecond,
		syncInterval:   time.Hour,
		Saida:          saida,
		ForwarderFactory: func(namespace, service string, targetPort int) (Forwarder, error) {
			return &mockForwarder{
				startFunc:       func(context.Context) (int, error) { return 55555, nil },
				healthCheckFunc: func(context.Context) error { return nil },
			}, nil
		},
	}
	cancelar := rodarLaco(m)
	esperarTexto(t, saida, "reconectado")
	cancelar()
	assert.Contains(t, saida.String(), "  ◈ túnel   conexão perdida · reconectando\n")
	assert.Contains(t, saida.String(), "  ✓ túnel   reconectado · localhost:55555\n")
	assert.Equal(t, 55555, m.Porta())
}

func TestStartSyncLoop_ReconexaoDesiste(t *testing.T) {
	semCor(t)
	saida := &saidaSegura{}
	m := &MirrorManager{
		Namespace:      "yby-system",
		Runner:         &MockRunner{},
		localPort:      12345,
		forwarder:      &mockForwarder{healthCheckFunc: func(context.Context) error { return errors.New("caiu") }},
		healthInterval: 5 * time.Millisecond,
		syncInterval:   time.Hour,
		Saida:          saida,
		ForwarderFactory: func(namespace, service string, targetPort int) (Forwarder, error) {
			return nil, errors.New("sem pod do git-server")
		},
	}
	cancelar := rodarLaco(m)
	esperarTexto(t, saida, "tentativa 1 de reconexão")
	// cancelar interrompe a espera da próxima tentativa: a reconexão desiste
	cancelar()
	assert.Contains(t, saida.String(), "  ✕ túnel   sem pod do git-server · tentativa 1 de reconexão fa…\n")
	// a desistência pelo ctrl+c não ganha "context canceled" como motivo
	assert.Contains(t, saida.String(), "  ✕ túnel   não reconectou · rode yby up de novo\n")
}

func TestComCausa(t *testing.T) {
	assert.Equal(t, "nota", comCausa(nil, "nota"))
	assert.Equal(t, "nota", comCausa(context.Canceled, "nota"))
	assert.Equal(t, "sem rede · nota", comCausa(errors.New("sem rede"), "nota"))
}

func TestMirrorManager_LinhaPadraoNoStdout(t *testing.T) {
	semCor(t)
	m := &MirrorManager{}
	// sem Saida, a linha vai para os.Stdout sem pânico
	m.linha(ui.EtapaFeita, "sync", "")
	assert.Equal(t, 0, m.Porta())
}
