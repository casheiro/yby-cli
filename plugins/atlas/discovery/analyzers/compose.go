package analyzers

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// composeFile representa a estrutura de um arquivo Docker Compose.
type composeFile struct {
	Services map[string]composeService `yaml:"services"`
	Networks map[string]interface{}    `yaml:"networks"`
	Volumes  map[string]interface{}    `yaml:"volumes"`
}

// composeService representa um serviço dentro do Docker Compose.
type composeService struct {
	Image     string      `yaml:"image"`
	Build     interface{} `yaml:"build"`
	DependsOn interface{} `yaml:"depends_on"` // pode ser []string ou map[string]interface{}
	Ports     []string    `yaml:"ports"`
	Networks  interface{} `yaml:"networks"` // pode ser []string ou map[string]interface{}
	Volumes   []string    `yaml:"volumes"`
}

// ComposeAnalyzer descobre recursos e relações em arquivos Docker Compose.
type ComposeAnalyzer struct{}

// NewComposeAnalyzer cria uma nova instância do ComposeAnalyzer.
func NewComposeAnalyzer() *ComposeAnalyzer {
	return &ComposeAnalyzer{}
}

// Name retorna o identificador do analyzer.
func (a *ComposeAnalyzer) Name() string {
	return "compose"
}

// Analyze processa arquivos Docker Compose e retorna recursos e relações.
func (a *ComposeAnalyzer) Analyze(rootPath string, files []string) (*AnalyzerResult, error) {
	result := &AnalyzerResult{Type: "compose"}

	composeFiles := filterComposeFiles(files)
	if len(composeFiles) == 0 {
		return result, nil
	}

	for _, f := range composeFiles {
		if err := a.analyzeFile(rootPath, f, result); err != nil {
			slog.Warn("erro ao analisar arquivo compose", "path", f, "error", err)
			continue
		}
	}

	return result, nil
}

// filterComposeFiles filtra apenas arquivos Docker Compose válidos.
func filterComposeFiles(files []string) []string {
	validNames := map[string]bool{
		"docker-compose.yml":  true,
		"docker-compose.yaml": true,
		"compose.yml":         true,
		"compose.yaml":        true,
	}

	var matched []string
	for _, f := range files {
		base := filepath.Base(f)
		if validNames[strings.ToLower(base)] {
			matched = append(matched, f)
		}
	}
	return matched
}

// analyzeFile analisa um único arquivo Docker Compose.
func (a *ComposeAnalyzer) analyzeFile(rootPath, filePath string, result *AnalyzerResult) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var cf composeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return err
	}

	relPath, _ := filepath.Rel(rootPath, filePath)

	// Mapeia network → serviços conectados (para gerar relações "connects")
	networkServices := make(map[string][]string)

	// Processa serviços
	for svcName, svc := range cf.Services {
		res := InfraResource{
			Kind:     "ComposeService",
			APIGroup: "compose",
			Name:     svcName,
			Path:     relPath,
			Metadata: make(map[string]string),
		}

		if svc.Image != "" {
			res.Metadata["image"] = svc.Image
		}
		if svc.Build != nil {
			res.Metadata["build"] = "true"
		}
		if len(svc.Ports) > 0 {
			res.Metadata["ports"] = strings.Join(svc.Ports, ",")
		}

		result.Resources = append(result.Resources, res)

		// Processa depends_on (pode ser []interface{} ou map[string]interface{})
		deps := parseDependsOn(svc.DependsOn)
		for _, dep := range deps {
			result.Relations = append(result.Relations, InfraRelation{
				From: res.ID(),
				To:   InfraResource{Kind: "ComposeService", Name: dep}.ID(),
				Type: "depends_on",
			})
		}

		// Processa networks do serviço
		nets := parseServiceNetworks(svc.Networks)
		for _, net := range nets {
			networkServices[net] = append(networkServices[net], svcName)
		}
	}

	// Processa networks nomeadas
	for netName := range cf.Networks {
		result.Resources = append(result.Resources, InfraResource{
			Kind:     "ComposeNetwork",
			APIGroup: "compose",
			Name:     netName,
			Path:     relPath,
		})
	}

	// Processa volumes nomeados
	for volName := range cf.Volumes {
		result.Resources = append(result.Resources, InfraResource{
			Kind:     "ComposeVolume",
			APIGroup: "compose",
			Name:     volName,
			Path:     relPath,
		})
	}

	// Gera relações "connects" entre serviços na mesma rede
	for _, svcs := range networkServices {
		for i := 0; i < len(svcs); i++ {
			for j := i + 1; j < len(svcs); j++ {
				result.Relations = append(result.Relations, InfraRelation{
					From: InfraResource{Kind: "ComposeService", Name: svcs[i]}.ID(),
					To:   InfraResource{Kind: "ComposeService", Name: svcs[j]}.ID(),
					Type: "connects",
				})
			}
		}
	}

	return nil
}

// parseDependsOn extrai a lista de dependências, tratando ambos os formatos:
// - lista: ["db", "redis"]
// - mapa: {"db": {"condition": "service_healthy"}}
func parseDependsOn(v interface{}) []string {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case []interface{}:
		var deps []string
		for _, item := range val {
			if s, ok := item.(string); ok {
				deps = append(deps, s)
			}
		}
		return deps
	case map[string]interface{}:
		var deps []string
		for k := range val {
			deps = append(deps, k)
		}
		return deps
	}

	return nil
}

// parseServiceNetworks extrai a lista de redes de um serviço, tratando ambos os formatos:
// - lista: ["frontend", "backend"]
// - mapa: {"frontend": {"aliases": ["app"]}}
func parseServiceNetworks(v interface{}) []string {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case []interface{}:
		var nets []string
		for _, item := range val {
			if s, ok := item.(string); ok {
				nets = append(nets, s)
			}
		}
		return nets
	case map[string]interface{}:
		var nets []string
		for k := range val {
			nets = append(nets, k)
		}
		return nets
	}

	return nil
}
