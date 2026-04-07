package analysis

import (
	"github.com/casheiro/yby-cli/plugins/atlas/discovery"
)

// ComponentMetrics contém métricas de acoplamento para um componente.
type ComponentMetrics struct {
	Path        string  `json:"path"`
	Ca          int     `json:"ca"`          // Afferent coupling: quantos dependem deste
	Ce          int     `json:"ce"`          // Efferent coupling: de quantos este depende
	Instability float64 `json:"instability"` // Ce / (Ca + Ce): 0 = estável, 1 = instável
}

// CalculateMetrics calcula métricas de acoplamento para cada componente do blueprint.
func CalculateMetrics(bp *discovery.Blueprint) []ComponentMetrics {
	if bp == nil || len(bp.Components) == 0 {
		return nil
	}

	// Inicializar contadores
	ca := make(map[string]int) // afferent: incoming
	ce := make(map[string]int) // efferent: outgoing

	for _, comp := range bp.Components {
		ca[comp.Path] = 0
		ce[comp.Path] = 0
	}

	// Contar relações
	for _, rel := range bp.Relations {
		ce[rel.From]++
		ca[rel.To]++
	}

	// Gerar métricas
	var metrics []ComponentMetrics
	for _, comp := range bp.Components {
		afferent := ca[comp.Path]
		efferent := ce[comp.Path]

		var instability float64
		total := afferent + efferent
		if total > 0 {
			instability = float64(efferent) / float64(total)
		}

		metrics = append(metrics, ComponentMetrics{
			Path:        comp.Path,
			Ca:          afferent,
			Ce:          efferent,
			Instability: instability,
		})
	}

	return metrics
}
