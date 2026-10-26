package analyzers

import (
	"log/slog"
	"os"
	"path/filepath"
	"sort"
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
	Image     string       `yaml:"image"`
	Build     interface{}  `yaml:"build"`
	DependsOn interface{}  `yaml:"depends_on"` // pode ser []string ou map[string]interface{}
	Ports     composeLista `yaml:"ports"`
	Networks  interface{}  `yaml:"networks"` // pode ser []string ou map[string]interface{}
	Volumes   composeLista `yaml:"volumes"`
}

// composeLista lê ports e volumes nas duas sintaxes do Compose: texto curto
// ("80:80", "./a:/b") e mapa longo ({target: 80, published: 8080}). Item que
// não é nenhum dos dois é ignorado, para não derrubar o arquivo inteiro.
type composeLista []string

// UnmarshalYAML implementa yaml.Unmarshaler.
func (l *composeLista) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.SequenceNode {
		return nil
	}
	for _, item := range node.Content {
		switch item.Kind {
		case yaml.ScalarNode:
			*l = append(*l, item.Value)
		case yaml.MappingNode:
			campos := map[string]string{}
			for i := 0; i+1 < len(item.Content); i += 2 {
				campos[item.Content[i].Value] = item.Content[i+1].Value
			}
			if v := formatarMapaCompose(campos); v != "" {
				*l = append(*l, v)
			}
		}
	}
	return nil
}

// formatarMapaCompose reduz a sintaxe longa à forma curta equivalente:
// porta vira "publicada:alvo" e volume vira "origem:alvo".
func formatarMapaCompose(c map[string]string) string {
	if c["target"] == "" {
		return ""
	}
	origem := c["published"]
	if origem == "" {
		origem = c["source"]
	}
	if origem == "" {
		return c["target"]
	}
	return origem + ":" + c["target"]
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
		parcial := &AnalyzerResult{Type: "compose"}
		if err := a.analyzeFile(rootPath, f, parcial); err != nil {
			slog.Warn("erro ao analisar arquivo compose", "path", f, "error", err)
			continue
		}
		mesclarCompose(result, parcial)
	}

	return result, nil
}

// EhArquivoCompose diz se o nome de arquivo é de um Compose: compose.yml,
// docker-compose.yml e suas variantes (docker-compose.override.yml,
// docker-compose.prod.yaml, compose.dev.yml). Nomes que só contêm "compose"
// (compose-notes.yml) não contam.
func EhArquivoCompose(nome string) bool {
	nome = strings.ToLower(nome)
	var resto string
	switch {
	case strings.HasPrefix(nome, "docker-compose"):
		resto = strings.TrimPrefix(nome, "docker-compose")
	case strings.HasPrefix(nome, "compose"):
		resto = strings.TrimPrefix(nome, "compose")
	default:
		return false
	}
	for _, ext := range []string{".yml", ".yaml"} {
		if sem, ok := strings.CutSuffix(resto, ext); ok {
			return sem == "" || (sem[0] == '.' && len(sem) > 1)
		}
	}
	return false
}

// filterComposeFiles filtra apenas arquivos Docker Compose válidos. A base vem
// antes dos overrides da mesma pasta, que a sobrescrevem na mesclagem.
func filterComposeFiles(files []string) []string {
	var matched []string
	for _, f := range files {
		if EhArquivoCompose(filepath.Base(f)) {
			matched = append(matched, f)
		}
	}
	sort.SliceStable(matched, func(i, j int) bool {
		di, dj := filepath.Dir(matched[i]), filepath.Dir(matched[j])
		if di != dj {
			return di < dj
		}
		bi, bj := filepath.Base(matched[i]), filepath.Base(matched[j])
		if len(bi) != len(bj) {
			return len(bi) < len(bj)
		}
		return bi < bj
	})
	return matched
}

// mesclarCompose junta o resultado de um arquivo ao acumulado. O mesmo recurso
// em mais de um arquivo (base e override) vira um nó só: os metadados do
// arquivo mais novo prevalecem e o caminho é o do primeiro. Relações iguais
// não se repetem.
func mesclarCompose(dst, src *AnalyzerResult) {
	indice := make(map[string]int, len(dst.Resources))
	for i, r := range dst.Resources {
		indice[r.ID()] = i
	}
	for _, r := range src.Resources {
		i, ok := indice[r.ID()]
		if !ok {
			indice[r.ID()] = len(dst.Resources)
			dst.Resources = append(dst.Resources, r)
			continue
		}
		if len(r.Metadata) > 0 && dst.Resources[i].Metadata == nil {
			dst.Resources[i].Metadata = map[string]string{}
		}
		for k, v := range r.Metadata {
			dst.Resources[i].Metadata[k] = v
		}
	}
	vistas := make(map[InfraRelation]bool, len(dst.Relations))
	for _, rel := range dst.Relations {
		vistas[rel] = true
	}
	for _, rel := range src.Relations {
		if !vistas[rel] {
			vistas[rel] = true
			dst.Relations = append(dst.Relations, rel)
		}
	}
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

	relPath, _ := relSlash(rootPath, filePath)

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
		// A ordem do mapa de serviços muda a cada execução; ordenado, a direção
		// de cada par é sempre a mesma.
		sort.Strings(svcs)
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
