package analysis

import (
	"testing"

	"github.com/casheiro/yby-cli/plugins/atlas/discovery"
)

func TestDetectCycles_SemRelacoes(t *testing.T) {
	bp := &discovery.Blueprint{
		Components: []discovery.Component{
			{Name: "a", Path: "a"},
		},
	}
	cycles := DetectCycles(bp)
	if len(cycles) != 0 {
		t.Errorf("esperado 0 ciclos, obtido %d", len(cycles))
	}
}

func TestDetectCycles_Nil(t *testing.T) {
	cycles := DetectCycles(nil)
	if cycles != nil {
		t.Errorf("esperado nil para blueprint nil, obtido %v", cycles)
	}
}

func TestDetectCycles_GrafoAciclico(t *testing.T) {
	bp := &discovery.Blueprint{
		Components: []discovery.Component{
			{Name: "a", Path: "a"},
			{Name: "b", Path: "b"},
			{Name: "c", Path: "c"},
		},
		Relations: []discovery.Relation{
			{From: "a", To: "b", Type: "imports"},
			{From: "b", To: "c", Type: "imports"},
		},
	}
	cycles := DetectCycles(bp)
	if len(cycles) != 0 {
		t.Errorf("esperado 0 ciclos em grafo acíclico, obtido %d: %v", len(cycles), cycles)
	}
}

func TestDetectCycles_CicloSimples(t *testing.T) {
	bp := &discovery.Blueprint{
		Relations: []discovery.Relation{
			{From: "a", To: "b", Type: "imports"},
			{From: "b", To: "c", Type: "imports"},
			{From: "c", To: "a", Type: "imports"},
		},
	}
	cycles := DetectCycles(bp)
	if len(cycles) != 1 {
		t.Fatalf("esperado 1 ciclo, obtido %d: %v", len(cycles), cycles)
	}

	cycle := cycles[0]
	// O ciclo deve ter 4 elementos (3 nós + repetição do primeiro)
	if len(cycle) != 4 {
		t.Errorf("esperado ciclo com 4 elementos, obtido %d: %v", len(cycle), cycle)
	}

	// O primeiro e último elemento devem ser iguais
	if cycle[0] != cycle[len(cycle)-1] {
		t.Errorf("ciclo deve ser fechado: primeiro=%s, último=%s", cycle[0], cycle[len(cycle)-1])
	}
}

func TestDetectCycles_AutoReferencia(t *testing.T) {
	bp := &discovery.Blueprint{
		Relations: []discovery.Relation{
			{From: "a", To: "a", Type: "imports"},
		},
	}
	cycles := DetectCycles(bp)
	if len(cycles) != 1 {
		t.Fatalf("esperado 1 ciclo para auto-referência, obtido %d: %v", len(cycles), cycles)
	}

	cycle := cycles[0]
	if len(cycle) != 2 || cycle[0] != "a" || cycle[1] != "a" {
		t.Errorf("esperado ciclo [a, a], obtido %v", cycle)
	}
}

func TestDetectCycles_MultiploCiclos(t *testing.T) {
	bp := &discovery.Blueprint{
		Relations: []discovery.Relation{
			// Ciclo 1: a -> b -> a
			{From: "a", To: "b", Type: "imports"},
			{From: "b", To: "a", Type: "imports"},
			// Ciclo 2: c -> d -> c
			{From: "c", To: "d", Type: "imports"},
			{From: "d", To: "c", Type: "imports"},
		},
	}
	cycles := DetectCycles(bp)
	if len(cycles) < 2 {
		t.Errorf("esperado pelo menos 2 ciclos, obtido %d: %v", len(cycles), cycles)
	}
}

func TestDetectCycles_CicloFechado(t *testing.T) {
	// Verifica que todo ciclo retornado tem primeiro == último
	bp := &discovery.Blueprint{
		Relations: []discovery.Relation{
			{From: "x", To: "y", Type: "imports"},
			{From: "y", To: "z", Type: "imports"},
			{From: "z", To: "x", Type: "imports"},
		},
	}
	cycles := DetectCycles(bp)
	for i, cycle := range cycles {
		if len(cycle) < 2 {
			t.Errorf("ciclo %d tem menos de 2 elementos: %v", i, cycle)
			continue
		}
		if cycle[0] != cycle[len(cycle)-1] {
			t.Errorf("ciclo %d não é fechado: %v", i, cycle)
		}
	}
}
