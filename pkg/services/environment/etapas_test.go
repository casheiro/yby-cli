package environment

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/services/bootstrap"
)

// gravadorEtapas registra os eventos de etapa como texto e conta os avanços de cota.
type gravadorEtapas struct {
	mu       sync.Mutex
	eventos  []string
	avancos  int
	ultimaFr float64
}

func (g *gravadorEtapas) add(e string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.eventos = append(g.eventos, e)
}

func (g *gravadorEtapas) Iniciar(r string) { g.add("iniciar " + r) }
func (g *gravadorEtapas) Avancar(f float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.avancos++
	g.ultimaFr = f
}
func (g *gravadorEtapas) Concluir(n string) { g.add("concluir " + n) }
func (g *gravadorEtapas) Falhar(n string)   { g.add("falhar " + n) }
func (g *gravadorEtapas) Detalhe(t string)  { g.add("detalhe " + t) }
func (g *gravadorEtapas) Avisar(t string)   { g.add("avisar " + t) }

func (g *gravadorEtapas) texto() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return strings.Join(g.eventos, "\n")
}

// espelhoComPorta é um MirrorService que expõe a porta do túnel.
type espelhoComPorta struct {
	MockMirrorService
	porta int
}

func (e *espelhoComPorta) Porta() int { return e.porta }

func TestPlanoLocal(t *testing.T) {
	got := strings.Join(PlanoLocal("yby-local"), ",")
	esperado := "cluster yby-local,git server,túnel,sync inicial,argo cd,secrets,root-app,convergência"
	if got != esperado {
		t.Errorf("PlanoLocal = %s; esperava %s", got, esperado)
	}
}

func TestRunLocalUp_Etapas(t *testing.T) {
	anterior := intervaloCota
	intervaloCota = time.Millisecond
	t.Cleanup(func() { intervaloCota = anterior })

	g := &gravadorEtapas{}
	var opcoesBootstrap bootstrap.BootstrapOptions
	bs := &MockBootstrapService{
		RunFunc: func(ctx context.Context, o bootstrap.BootstrapOptions) error {
			opcoesBootstrap = o
			return nil
		},
		WaitHealthyFunc: func(ctx context.Context, name, ns string, timeout int) error {
			// espera o bastante para a cota avançar ao menos uma vez
			time.Sleep(30 * time.Millisecond)
			return nil
		},
	}
	espelho := &espelhoComPorta{porta: 51234}
	espelho.SyncFunc = func() error {
		return fmt.Errorf("exit status 1: error: src refspec HEAD does not match any · error: failed to push some refs")
	}
	svc := NewEnvironmentService(&MockRunner{}, nil, &MockClusterManager{
		ExistsFunc: func(context.Context, string) (bool, error) { return false, nil },
	}, espelho, bs)
	svc.Etapas = g

	if err := svc.Up(context.Background(), UpOptions{Environment: "local", ClusterName: "yby-local"}); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	esperado := strings.Join([]string{
		"iniciar cluster yby-local", "concluir criado",
		"iniciar git server", "concluir ",
		"iniciar túnel", "concluir localhost:51234",
		// a falha do sync não interrompe o up: alerta com o motivo
		"iniciar sync inicial", "concluir adiado",
		"avisar o sync inicial falhou · src refspec HEAD does not match any · o auto-sync tenta de novo",
		"iniciar convergência", "concluir root-app healthy",
	}, "\n")
	if g.texto() != esperado {
		t.Errorf("etapas:\n%s\nesperado:\n%s", g.texto(), esperado)
	}
	if opcoesBootstrap.Etapas != g {
		t.Error("o bootstrap deveria receber as mesmas etapas para marcar as suas")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.avancos == 0 || g.ultimaFr <= 0 {
		t.Errorf("a convergência deveria avançar a cota: %d avanços, última %v", g.avancos, g.ultimaFr)
	}
}

func TestRunLocalUp_EtapasFalhas(t *testing.T) {
	casos := []struct {
		nome     string
		svc      func() *EnvironmentService
		esperado string
	}{
		{"start", func() *EnvironmentService {
			return NewEnvironmentService(&MockRunner{}, nil, &MockClusterManager{
				StartFunc: func(context.Context, string) error { return fmt.Errorf("docker parado") },
			}, &MockMirrorService{}, &MockBootstrapService{})
		}, "iniciar cluster yby-local\nfalhar "},
		{"create", func() *EnvironmentService {
			return NewEnvironmentService(&MockRunner{}, nil, &MockClusterManager{
				ExistsFunc: func(context.Context, string) (bool, error) { return false, nil },
				CreateFunc: func(context.Context, string, string) error { return fmt.Errorf("porta ocupada") },
			}, &MockMirrorService{}, &MockBootstrapService{})
		}, "iniciar cluster yby-local\nfalhar "},
		{"git server", func() *EnvironmentService {
			return NewEnvironmentService(&MockRunner{}, nil, &MockClusterManager{}, &MockMirrorService{
				EnsureGitServerFunc: func() error { return fmt.Errorf("sem rollout") },
			}, &MockBootstrapService{})
		}, "iniciar git server\nfalhar "},
		{"túnel", func() *EnvironmentService {
			return NewEnvironmentService(&MockRunner{}, nil, &MockClusterManager{}, &MockMirrorService{
				SetupTunnelFunc: func(context.Context) error { return fmt.Errorf("sem pod") },
			}, &MockBootstrapService{})
		}, "iniciar túnel\nfalhar "},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			g := &gravadorEtapas{}
			svc := c.svc()
			svc.Etapas = g
			if err := svc.Up(context.Background(), UpOptions{Environment: "local", ClusterName: "yby-local"}); err == nil {
				t.Fatal("esperava erro")
			}
			if !strings.HasSuffix(g.texto(), c.esperado) {
				t.Errorf("etapas:\n%s\nesperava terminar em %q", g.texto(), c.esperado)
			}
		})
	}
}

func TestRunLocalUp_ConvergenciaFalhaNaoInterrompe(t *testing.T) {
	g := &gravadorEtapas{}
	svc := NewEnvironmentService(&MockRunner{}, nil, &MockClusterManager{}, &MockMirrorService{},
		&MockBootstrapService{WaitHealthyFunc: func(context.Context, string, string, int) error {
			return fmt.Errorf("timeout")
		}})
	svc.Etapas = g
	if err := svc.Up(context.Background(), UpOptions{Environment: "local", ClusterName: "c"}); err != nil {
		t.Fatalf("a convergência atrasada não interrompe o up: %v", err)
	}
	if !strings.HasSuffix(g.texto(), "concluir tempo esgotado\navisar a root-app ainda não convergiu · confira com yby status") {
		t.Errorf("etapas:\n%s", g.texto())
	}
}
