package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// gravadorEtapas registra os eventos de etapa como texto ("iniciar argo cd", ...) e, à
// parte, quantos avanços de cota cada etapa recebeu (dependem do relógio, então não
// entram no texto).
type gravadorEtapas struct {
	mu      sync.Mutex
	eventos []string
	atual   string
	avancos map[string]int
}

func (g *gravadorEtapas) add(e string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.eventos = append(g.eventos, e)
}

func (g *gravadorEtapas) Iniciar(r string) {
	g.add("iniciar " + r)
	g.mu.Lock()
	g.atual = r
	g.mu.Unlock()
}
func (g *gravadorEtapas) Avancar(float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.avancos == nil {
		g.avancos = map[string]int{}
	}
	g.avancos[g.atual]++
}
func (g *gravadorEtapas) Concluir(n string) { g.add("concluir " + n) }
func (g *gravadorEtapas) Falhar(n string)   { g.add("falhar " + n) }
func (g *gravadorEtapas) Detalhe(t string)  { g.add("detalhe " + t) }
func (g *gravadorEtapas) Avisar(t string)   { g.add("avisar " + t) }

func (g *gravadorEtapas) avancosDe(rotulo string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.avancos[rotulo]
}

func (g *gravadorEtapas) texto() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return strings.Join(g.eventos, "\n")
}

func blueprintFS(checkpoint *BootstrapCheckpoint) *MockFilesystem {
	return &MockFilesystem{ReadFileFunc: func(name string) ([]byte, error) {
		if strings.Contains(name, "bootstrap-state.json") {
			if checkpoint == nil {
				return nil, fmt.Errorf("não existe")
			}
			return json.Marshal(checkpoint)
		}
		return []byte("prompts:\n  - id: git.repoURL\n    default: https://github.com/org/repo.git\n"), nil
	}}
}

func TestRun_EtapasDoBootstrap(t *testing.T) {
	g := &gravadorEtapas{}
	svc := NewService(&MockRunner{}, blueprintFS(nil), &MockK8sClient{})
	err := svc.Run(context.Background(), BootstrapOptions{Root: "/infra", Etapas: g})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	esperado := strings.Join([]string{
		"iniciar argo cd", "concluir argo-cd 5.51.6",
		"iniciar secrets", "concluir sealed-secrets",
		"iniciar root-app", "concluir aplicada",
	}, "\n")
	if g.texto() != esperado {
		t.Errorf("etapas:\n%s\nesperado:\n%s", g.texto(), esperado)
	}
}

func TestRun_EtapasPlainSecretsEControllerAusente(t *testing.T) {
	g := &gravadorEtapas{}
	svc := NewService(&MockRunner{}, blueprintFS(nil), &MockK8sClient{})
	if err := svc.Run(context.Background(), BootstrapOptions{Root: "/infra", PlainSecrets: true, Etapas: g}); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !strings.Contains(g.texto(), "concluir plain-secrets\navisar secrets sem encriptação · só para dev local\niniciar root-app") {
		t.Errorf("a etapa de secrets deveria ganhar o alerta da falta de encriptação:\n%s", g.texto())
	}

	g = &gravadorEtapas{}
	falhaKubectl := &MockRunner{RunFunc: func(ctx context.Context, name string, args ...string) error {
		if name == "kubectl" {
			return fmt.Errorf("not found")
		}
		return nil
	}}
	svc = NewService(falhaKubectl, blueprintFS(nil), &MockK8sClient{})
	if err := svc.Run(context.Background(), BootstrapOptions{Root: "/infra", Etapas: g}); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !strings.Contains(g.texto(), "concluir sealed-secrets\navisar controller do sealed-secrets ainda não instalado no cluster") {
		t.Errorf("a etapa de secrets deveria ganhar o alerta do controller ausente:\n%s", g.texto())
	}
}

func TestRun_CotaDoHelmNaEtapaArgoCD(t *testing.T) {
	anterior := intervaloCota
	intervaloCota = time.Millisecond
	t.Cleanup(func() { intervaloCota = anterior })

	g := &gravadorEtapas{}
	helmDemorado := &MockRunner{RunFunc: func(ctx context.Context, name string, args ...string) error {
		if name == "helm" && len(args) > 0 && args[0] == "upgrade" {
			// o helm --wait demora: a cota da etapa argo cd anda enquanto isso
			time.Sleep(30 * time.Millisecond)
		}
		return nil
	}}
	svc := NewService(helmDemorado, blueprintFS(nil), &MockK8sClient{})
	if err := svc.Run(context.Background(), BootstrapOptions{Root: "/infra", Etapas: g}); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if g.avancosDe(RotuloArgoCD) == 0 {
		t.Error("a etapa argo cd deveria receber a cota durante o helm --wait")
	}
	if n := g.avancosDe(RotuloSecrets) + g.avancosDe(RotuloRootApp); n != 0 {
		t.Errorf("a cota para quando o helm termina; as outras etapas receberam %d avanços", n)
	}
}

func TestRun_EtapasRetomadasDoCheckpoint(t *testing.T) {
	g := &gravadorEtapas{}
	cp := &BootstrapCheckpoint{Phase: PhaseSecrets, Root: "/infra", Environment: "local"}
	svc := NewService(&MockRunner{}, blueprintFS(cp), &MockK8sClient{})
	err := svc.Run(context.Background(), BootstrapOptions{Root: "/infra", Environment: "local", Etapas: g})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	esperado := strings.Join([]string{
		"iniciar argo cd", "concluir " + notaJaFeita,
		"iniciar secrets", "concluir " + notaJaFeita,
		"iniciar root-app", "concluir aplicada",
	}, "\n")
	if g.texto() != esperado {
		t.Errorf("etapas:\n%s\nesperado:\n%s", g.texto(), esperado)
	}
}

func TestRun_EtapaFalha(t *testing.T) {
	g := &gravadorEtapas{}
	// estratégia sops sem o sops no PATH: a fase de secrets falha sem retry
	fsys := &MockFilesystem{ReadFileFunc: func(name string) ([]byte, error) {
		if strings.Contains(name, "bootstrap-state.json") {
			return nil, fmt.Errorf("não existe")
		}
		return []byte("prompts:\n  - id: git.repoURL\n    default: https://x/r.git\n  - id: secrets.strategy\n    default: sops\n"), nil
	}}
	runner := &lookPathSelectiveRunner{available: map[string]bool{"kubectl": true, "helm": true}}
	svc := NewService(runner, fsys, &MockK8sClient{})
	if err := svc.Run(context.Background(), BootstrapOptions{Root: "/infra", Etapas: g}); err == nil {
		t.Fatal("esperava erro da fase de secrets")
	}
	if !strings.HasSuffix(g.texto(), "iniciar secrets\nfalhar ") {
		t.Errorf("a etapa secrets deveria falhar por último:\n%s", g.texto())
	}
}
