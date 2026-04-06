package analyzers

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// kustomization representa a estrutura de um arquivo kustomization.yaml.
type kustomization struct {
	Resources  []string `yaml:"resources"`
	Bases      []string `yaml:"bases"`
	Components []string `yaml:"components"`
	Namespace  string   `yaml:"namespace"`
}

// KustomizeAnalyzer descobre recursos e relações em arquivos Kustomize.
type KustomizeAnalyzer struct{}

// NewKustomizeAnalyzer cria uma nova instância do KustomizeAnalyzer.
func NewKustomizeAnalyzer() *KustomizeAnalyzer {
	return &KustomizeAnalyzer{}
}

// Name retorna o identificador do analyzer.
func (a *KustomizeAnalyzer) Name() string {
	return "kustomize"
}

// Analyze processa arquivos kustomization.yaml e retorna recursos e relações.
func (a *KustomizeAnalyzer) Analyze(rootPath string, files []string) (*AnalyzerResult, error) {
	result := &AnalyzerResult{Type: "kustomize"}

	kustomizeFiles := filterKustomizeFiles(files)
	if len(kustomizeFiles) == 0 {
		return result, nil
	}

	for _, f := range kustomizeFiles {
		if err := a.analyzeFile(rootPath, f, result); err != nil {
			slog.Warn("erro ao analisar arquivo kustomize", "path", f, "error", err)
			continue
		}
	}

	return result, nil
}

// filterKustomizeFiles filtra apenas arquivos kustomization válidos.
func filterKustomizeFiles(files []string) []string {
	validNames := map[string]bool{
		"kustomization.yaml": true,
		"kustomization.yml":  true,
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

// analyzeFile analisa um único arquivo kustomization.yaml.
func (a *KustomizeAnalyzer) analyzeFile(rootPath, filePath string, result *AnalyzerResult) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var k kustomization
	if err := yaml.Unmarshal(data, &k); err != nil {
		return err
	}

	relPath, _ := filepath.Rel(rootPath, filePath)
	dirName := filepath.Dir(relPath)
	if dirName == "." {
		dirName = filepath.Base(rootPath)
	}

	// Cria recurso para o próprio kustomization
	kustRes := InfraResource{
		Kind:     "Kustomization",
		APIGroup: "kustomize",
		Name:     dirName,
		Path:     relPath,
	}
	if k.Namespace != "" {
		kustRes.Namespace = k.Namespace
	}
	result.Resources = append(result.Resources, kustRes)

	// Processa resources, bases e components como referências "includes"
	allRefs := make([]string, 0, len(k.Resources)+len(k.Bases)+len(k.Components))
	allRefs = append(allRefs, k.Resources...)
	allRefs = append(allRefs, k.Bases...)
	allRefs = append(allRefs, k.Components...)

	for _, ref := range allRefs {
		a.processReference(kustRes, ref, result)
	}

	return nil
}

// processReference processa uma referência de resource/base/component.
func (a *KustomizeAnalyzer) processReference(parent InfraResource, ref string, result *AnalyzerResult) {
	if isRemoteRef(ref) {
		// Referência remota — cria recurso KustomizeRemote
		remoteRes := InfraResource{
			Kind:     "KustomizeRemote",
			APIGroup: "kustomize",
			Name:     ref,
			Path:     parent.Path,
		}
		result.Resources = append(result.Resources, remoteRes)
		result.Relations = append(result.Relations, InfraRelation{
			From: parent.ID(),
			To:   remoteRes.ID(),
			Type: "includes",
		})
		return
	}

	// Referência local — usa o nome do diretório como alvo
	targetName := ref
	// Remove trailing slash se houver
	targetName = strings.TrimSuffix(targetName, "/")
	// Usa o último segmento do path como nome
	if parts := strings.Split(targetName, "/"); len(parts) > 0 {
		targetName = parts[len(parts)-1]
	}

	// Se aponta para um arquivo específico (ex: deployment.yaml), usa como está
	if hasYAMLExtension(ref) {
		targetName = ref
	}

	targetRes := InfraResource{
		Kind:     "Kustomization",
		APIGroup: "kustomize",
		Name:     targetName,
	}

	result.Relations = append(result.Relations, InfraRelation{
		From: parent.ID(),
		To:   targetRes.ID(),
		Type: "includes",
	})
}

// isRemoteRef verifica se a referência é um URL remoto.
func isRemoteRef(ref string) bool {
	return strings.HasPrefix(ref, "http://") ||
		strings.HasPrefix(ref, "https://") ||
		strings.HasPrefix(ref, "git@") ||
		strings.HasPrefix(ref, "git://") ||
		strings.HasPrefix(ref, "ssh://")
}

// hasYAMLExtension verifica se o path termina com extensão YAML.
func hasYAMLExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}
