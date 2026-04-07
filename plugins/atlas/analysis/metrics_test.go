package analysis

import (
	"math"
	"testing"

	"github.com/casheiro/yby-cli/plugins/atlas/discovery"
)

func TestCalculateMetrics_Nil(t *testing.T) {
	metrics := CalculateMetrics(nil)
	if metrics != nil {
		t.Errorf("esperado nil para blueprint nil, obtido %v", metrics)
	}
}

func TestCalculateMetrics_SemComponentes(t *testing.T) {
	bp := &discovery.Blueprint{}
	metrics := CalculateMetrics(bp)
	if metrics != nil {
		t.Errorf("esperado nil para blueprint sem componentes, obtido %v", metrics)
	}
}

func TestCalculateMetrics_SemRelacoes(t *testing.T) {
	bp := &discovery.Blueprint{
		Components: []discovery.Component{
			{Name: "a", Path: "a"},
			{Name: "b", Path: "b"},
		},
	}
	metrics := CalculateMetrics(bp)
	if len(metrics) != 2 {
		t.Fatalf("esperado 2 métricas, obtido %d", len(metrics))
	}

	for _, m := range metrics {
		if m.Ca != 0 || m.Ce != 0 || m.Instability != 0 {
			t.Errorf("componente %s sem relações deve ter Ca=0, Ce=0, Instability=0, obtido Ca=%d Ce=%d I=%f",
				m.Path, m.Ca, m.Ce, m.Instability)
		}
	}
}

func TestCalculateMetrics_ComRelacoes(t *testing.T) {
	bp := &discovery.Blueprint{
		Components: []discovery.Component{
			{Name: "api", Path: "api"},
			{Name: "lib", Path: "lib"},
			{Name: "db", Path: "db"},
		},
		Relations: []discovery.Relation{
			{From: "api", To: "lib", Type: "imports"},
			{From: "api", To: "db", Type: "imports"},
			{From: "db", To: "lib", Type: "imports"},
		},
	}
	metrics := CalculateMetrics(bp)
	if len(metrics) != 3 {
		t.Fatalf("esperado 3 métricas, obtido %d", len(metrics))
	}

	// Mapear por path
	byPath := make(map[string]ComponentMetrics)
	for _, m := range metrics {
		byPath[m.Path] = m
	}

	// api: Ca=0 (ninguém depende), Ce=2 (depende de lib + db)
	apiM := byPath["api"]
	if apiM.Ca != 0 || apiM.Ce != 2 {
		t.Errorf("api: esperado Ca=0 Ce=2, obtido Ca=%d Ce=%d", apiM.Ca, apiM.Ce)
	}
	// Instability = 2/(0+2) = 1.0
	if math.Abs(apiM.Instability-1.0) > 0.001 {
		t.Errorf("api: esperado Instability=1.0, obtido %f", apiM.Instability)
	}

	// lib: Ca=2 (api + db dependem), Ce=0
	libM := byPath["lib"]
	if libM.Ca != 2 || libM.Ce != 0 {
		t.Errorf("lib: esperado Ca=2 Ce=0, obtido Ca=%d Ce=%d", libM.Ca, libM.Ce)
	}
	// Instability = 0/(2+0) = 0.0
	if math.Abs(libM.Instability-0.0) > 0.001 {
		t.Errorf("lib: esperado Instability=0.0, obtido %f", libM.Instability)
	}

	// db: Ca=1 (api depende), Ce=1 (depende de lib)
	dbM := byPath["db"]
	if dbM.Ca != 1 || dbM.Ce != 1 {
		t.Errorf("db: esperado Ca=1 Ce=1, obtido Ca=%d Ce=%d", dbM.Ca, dbM.Ce)
	}
	// Instability = 1/(1+1) = 0.5
	if math.Abs(dbM.Instability-0.5) > 0.001 {
		t.Errorf("db: esperado Instability=0.5, obtido %f", dbM.Instability)
	}
}

func TestCalculateMetrics_ComponenteIsolado(t *testing.T) {
	bp := &discovery.Blueprint{
		Components: []discovery.Component{
			{Name: "a", Path: "a"},
			{Name: "b", Path: "b"},
		},
		Relations: []discovery.Relation{
			{From: "a", To: "b", Type: "imports"},
		},
	}
	metrics := CalculateMetrics(bp)

	byPath := make(map[string]ComponentMetrics)
	for _, m := range metrics {
		byPath[m.Path] = m
	}

	// a: Ce=1, Ca=0 → Instability=1.0
	if byPath["a"].Instability != 1.0 {
		t.Errorf("a: esperado Instability=1.0, obtido %f", byPath["a"].Instability)
	}

	// b: Ce=0, Ca=1 → Instability=0.0
	if byPath["b"].Instability != 0.0 {
		t.Errorf("b: esperado Instability=0.0, obtido %f", byPath["b"].Instability)
	}
}
