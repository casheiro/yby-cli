package shared

import (
	"sync"
	"testing"
	"time"
)

// etapasGravadas guarda as frações recebidas por Avancar.
type etapasGravadas struct {
	EtapasMudas
	mu      sync.Mutex
	fracoes []float64
}

func (e *etapasGravadas) Avancar(f float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.fracoes = append(e.fracoes, f)
}

func (e *etapasGravadas) total() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.fracoes)
}

func TestEtapasMudas(t *testing.T) {
	var e Etapas = EtapasMudas{}
	e.Iniciar("a")
	e.Avancar(0.5)
	e.Concluir("")
	e.Falhar("")
	e.Detalhe("")
	e.Avisar("")
	if _, ok := OuMudas(nil).(EtapasMudas); !ok {
		t.Error("OuMudas(nil) deveria devolver EtapasMudas")
	}
	g := &etapasGravadas{}
	if OuMudas(g) != g {
		t.Error("OuMudas deveria devolver as etapas recebidas")
	}
}

func TestAcompanharLimite(t *testing.T) {
	g := &etapasGravadas{}
	parar := AcompanharLimite(g, time.Second, time.Millisecond)
	limite := time.Now().Add(2 * time.Second)
	for g.total() < 2 && time.Now().Before(limite) {
		time.Sleep(time.Millisecond)
	}
	parar()
	parar() // parar duas vezes é seguro
	if g.total() < 2 {
		t.Fatalf("esperava ao menos 2 avanços, veio %d", g.total())
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, f := range g.fracoes {
		if f <= 0 {
			t.Errorf("fração deveria ser positiva: %v", f)
		}
	}
}

func TestAcompanharLimiteSemEfeito(t *testing.T) {
	// sem etapas, limite ou intervalo, não há goroutine e parar não trava
	AcompanharLimite(nil, time.Second, time.Millisecond)()
	AcompanharLimite(EtapasMudas{}, 0, time.Millisecond)()
	AcompanharLimite(EtapasMudas{}, time.Second, 0)()
}
