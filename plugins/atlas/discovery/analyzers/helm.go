package analyzers

import (
	"bufio"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// chartMeta representa a estrutura de um Chart.yaml do Helm.
type chartMeta struct {
	APIVersion   string     `yaml:"apiVersion"`
	Name         string     `yaml:"name"`
	Version      string     `yaml:"version"`
	Description  string     `yaml:"description"`
	Dependencies []chartDep `yaml:"dependencies"`
}

// chartDep representa uma dependência declarada em Chart.yaml.
type chartDep struct {
	Name       string `yaml:"name"`
	Version    string `yaml:"version"`
	Repository string `yaml:"repository"`
	Condition  string `yaml:"condition"`
}

// HelmAnalyzer analisa Chart.yaml e descobre recursos e relações Helm.
type HelmAnalyzer struct{}

// NewHelmAnalyzer cria uma nova instância do analyzer Helm.
func NewHelmAnalyzer() *HelmAnalyzer {
	return &HelmAnalyzer{}
}

// Name retorna o identificador do analyzer.
func (h *HelmAnalyzer) Name() string {
	return "helm"
}

// Analyze recebe o path raiz e a lista de arquivos Chart.yaml,
// retornando os recursos e relações encontrados.
func (h *HelmAnalyzer) Analyze(rootPath string, files []string) (*AnalyzerResult, error) {
	result := &AnalyzerResult{
		Resources: []InfraResource{},
		Relations: []InfraRelation{},
		Type:      "helm",
	}

	// Índice de charts por diretório relativo para resolver dependências file://
	chartsByDir := make(map[string]InfraResource)

	// Primeira passada: parsear todos os charts e criar recursos
	chartMetas := make(map[string]*chartMeta)
	for _, file := range files {
		meta, err := parseChartYAML(file)
		if err != nil {
			slog.Warn("falha ao parsear Chart.yaml", "path", file, "erro", err)
			continue
		}

		relPath, err := filepath.Rel(rootPath, file)
		if err != nil {
			relPath = file
		}

		chartDir := filepath.Dir(relPath)

		resource := InfraResource{
			Kind:     "HelmChart",
			APIGroup: "helm",
			Name:     meta.Name,
			Path:     relPath,
			Metadata: map[string]string{
				"version": meta.Version,
			},
		}
		if meta.Description != "" {
			resource.Metadata["description"] = meta.Description
		}
		if meta.APIVersion != "" {
			resource.Metadata["apiVersion"] = meta.APIVersion
		}

		result.Resources = append(result.Resources, resource)
		chartsByDir[chartDir] = resource
		chartMetas[relPath] = meta
	}

	// Segunda passada: processar dependências e templates
	for chartPath, meta := range chartMetas {
		chartDir := filepath.Dir(chartPath)
		chartResource := chartsByDir[chartDir]
		chartID := chartResource.ID()

		// Processar dependências
		for _, dep := range meta.Dependencies {
			if strings.HasPrefix(dep.Repository, "file://") {
				// Dependência local
				localPath := strings.TrimPrefix(dep.Repository, "file://")
				targetDir := filepath.Join(chartDir, localPath)
				targetDir = filepath.Clean(targetDir)

				if targetChart, ok := chartsByDir[targetDir]; ok {
					result.Relations = append(result.Relations, InfraRelation{
						From: chartID,
						To:   targetChart.ID(),
						Type: "depends_on",
					})
				} else {
					// Chart local não encontrado na lista, criar recurso para referência
					depResource := InfraResource{
						Kind:     "HelmChart",
						APIGroup: "helm",
						Name:     dep.Name,
						Path:     targetDir,
						Metadata: map[string]string{
							"version": dep.Version,
						},
					}
					result.Resources = append(result.Resources, depResource)
					result.Relations = append(result.Relations, InfraRelation{
						From: chartID,
						To:   depResource.ID(),
						Type: "depends_on",
					})
				}
			} else if dep.Repository != "" {
				// Dependência de repositório remoto
				repoResource := InfraResource{
					Kind:     "HelmRepository",
					APIGroup: "helm",
					Name:     dep.Name,
					Path:     chartPath,
					Metadata: map[string]string{
						"repository": dep.Repository,
						"version":    dep.Version,
					},
				}
				if dep.Condition != "" {
					repoResource.Metadata["condition"] = dep.Condition
				}
				result.Resources = append(result.Resources, repoResource)
				result.Relations = append(result.Relations, InfraRelation{
					From: chartID,
					To:   repoResource.ID(),
					Type: "depends_on",
				})
			}
		}

		// Escanear diretório templates/
		absChartDir := filepath.Join(rootPath, chartDir)
		templatesDir := filepath.Join(absChartDir, "templates")
		templateResources := scanTemplatesDir(rootPath, templatesDir)
		for _, tmplRes := range templateResources {
			result.Resources = append(result.Resources, tmplRes)
			result.Relations = append(result.Relations, InfraRelation{
				From: chartID,
				To:   tmplRes.ID(),
				Type: "deploys",
			})
		}
	}

	return result, nil
}

// parseChartYAML lê e faz o parse de um arquivo Chart.yaml.
func parseChartYAML(path string) (*chartMeta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var meta chartMeta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}

// kindNameRe extrai kind e name de manifests K8s.
var kindNameRe = regexp.MustCompile(`(?m)^kind:\s*(\S+)`)
var nameRe = regexp.MustCompile(`(?m)^\s*name:\s*(\S+)`)

// scanTemplatesDir escaneia o diretório templates/ em busca de recursos K8s.
func scanTemplatesDir(rootPath string, templatesDir string) []InfraResource {
	var resources []InfraResource

	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		// Ignorar helpers e NOTES
		if name == "_helpers.tpl" || name == "NOTES.txt" {
			continue
		}

		filePath := filepath.Join(templatesDir, name)
		resources = append(resources, extractTemplateResources(rootPath, filePath)...)
	}

	return resources
}

// extractTemplateResources extrai recursos K8s de um arquivo de template Helm.
func extractTemplateResources(rootPath string, filePath string) []InfraResource {
	var resources []InfraResource

	file, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	relPath, err := filepath.Rel(rootPath, filePath)
	if err != nil {
		relPath = filePath
	}

	// Ler o conteúdo completo para analisar documentos YAML separados por ---
	scanner := bufio.NewScanner(file)
	var currentDoc strings.Builder

	flushDoc := func() {
		content := currentDoc.String()
		currentDoc.Reset()

		if strings.TrimSpace(content) == "" {
			return
		}

		kind, name := extractKindAndName(content)
		if kind == "" {
			return
		}

		if name == "" {
			name = "unknown"
		}

		// Remover aspas e anotações de template do nome
		name = cleanTemplateName(name)

		resources = append(resources, InfraResource{
			Kind:     kind,
			APIGroup: "k8s",
			Name:     name,
			Path:     relPath,
		})
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			flushDoc()
			continue
		}
		currentDoc.WriteString(line)
		currentDoc.WriteString("\n")
	}
	// Processar último documento
	flushDoc()

	return resources
}

// extractKindAndName extrai o kind e name de um conteúdo YAML (com ou sem templates Go).
func extractKindAndName(content string) (string, string) {
	kindMatch := kindNameRe.FindStringSubmatch(content)
	if len(kindMatch) < 2 {
		return "", ""
	}
	kind := kindMatch[1]

	nameMatch := nameRe.FindStringSubmatch(content)
	name := ""
	if len(nameMatch) >= 2 {
		name = nameMatch[1]
	}

	return kind, name
}

// cleanTemplateName remove aspas e expressões de template Go de nomes de recursos.
func cleanTemplateName(name string) string {
	// Remover aspas
	name = strings.Trim(name, "\"'")

	// Se o nome é inteiramente uma expressão de template, usar como está
	if strings.HasPrefix(name, "{{") && strings.HasSuffix(name, "}}") {
		// Extrair referência do template (ex: {{ .Release.Name }})
		inner := strings.TrimPrefix(name, "{{")
		inner = strings.TrimSuffix(inner, "}}")
		inner = strings.TrimSpace(inner)
		// Simplificar: usar a última parte da expressão
		parts := strings.Split(inner, ".")
		if len(parts) > 0 {
			return "{{ " + inner + " }}"
		}
	}

	return name
}
