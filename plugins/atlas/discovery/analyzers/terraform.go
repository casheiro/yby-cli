package analyzers

import (
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// reResource captura blocos "resource "type" "name""
	reResource = regexp.MustCompile(`(?m)^\s*resource\s+"([^"]+)"\s+"([^"]+)"`)
	// reModule captura blocos "module "name""
	reModule = regexp.MustCompile(`(?m)^\s*module\s+"([^"]+)"`)
	// reData captura blocos "data "type" "name""
	reData = regexp.MustCompile(`(?m)^\s*data\s+"([^"]+)"\s+"([^"]+)"`)
	// reModuleSource captura "source = "path"" dentro de blocos module
	reModuleSource = regexp.MustCompile(`(?m)^\s*source\s*=\s*"([^"]+)"`)
)

// TerraformAnalyzer descobre recursos e relações em arquivos Terraform (.tf).
// Usa regex (sem dependência HCL) para extração leve de blocos.
type TerraformAnalyzer struct{}

// NewTerraformAnalyzer cria uma nova instância do TerraformAnalyzer.
func NewTerraformAnalyzer() *TerraformAnalyzer {
	return &TerraformAnalyzer{}
}

// Name retorna o identificador do analyzer.
func (a *TerraformAnalyzer) Name() string {
	return "terraform"
}

// Analyze processa arquivos .tf e retorna recursos e relações.
func (a *TerraformAnalyzer) Analyze(rootPath string, files []string) (*AnalyzerResult, error) {
	result := &AnalyzerResult{Type: "terraform"}

	tfFiles := filterTerraformFiles(files)
	if len(tfFiles) == 0 {
		return result, nil
	}

	// Agrupa arquivos por diretório (cada diretório = um "módulo" conceitual)
	dirFiles := make(map[string][]string)
	for _, f := range tfFiles {
		dir := filepath.Dir(f)
		dirFiles[dir] = append(dirFiles[dir], f)
	}

	for dir, dFiles := range dirFiles {
		a.analyzeDirectory(rootPath, dir, dFiles, result)
	}

	return result, nil
}

// filterTerraformFiles filtra apenas arquivos .tf.
func filterTerraformFiles(files []string) []string {
	var matched []string
	for _, f := range files {
		if strings.ToLower(filepath.Ext(f)) == ".tf" {
			matched = append(matched, f)
		}
	}
	return matched
}

// analyzeDirectory analisa todos os arquivos .tf de um diretório.
func (a *TerraformAnalyzer) analyzeDirectory(rootPath, dir string, files []string, result *AnalyzerResult) {
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			slog.Warn("erro ao ler arquivo terraform", "path", f, "error", err)
			continue
		}

		relPath, _ := filepath.Rel(rootPath, f)
		content := string(data)

		a.extractResources(content, relPath, result)
		a.extractModules(content, relPath, result)
		a.extractData(content, relPath, result)
	}
}

// extractResources extrai blocos "resource" do conteúdo HCL.
func (a *TerraformAnalyzer) extractResources(content, relPath string, result *AnalyzerResult) {
	matches := reResource.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		resType := m[1]
		resName := m[2]
		result.Resources = append(result.Resources, InfraResource{
			Kind:     "TerraformResource",
			APIGroup: "terraform",
			Name:     resType + "." + resName,
			Path:     relPath,
			Metadata: map[string]string{
				"type": resType,
				"name": resName,
			},
		})
	}
}

// extractModules extrai blocos "module" e suas referências source.
func (a *TerraformAnalyzer) extractModules(content, relPath string, result *AnalyzerResult) {
	matches := reModule.FindAllStringSubmatchIndex(content, -1)
	for _, loc := range matches {
		name := content[loc[2]:loc[3]]

		modRes := InfraResource{
			Kind:     "TerraformModule",
			APIGroup: "terraform",
			Name:     name,
			Path:     relPath,
		}
		result.Resources = append(result.Resources, modRes)

		// Busca "source" dentro do bloco do módulo
		// Procura a partir da posição do match até o próximo bloco de nível superior
		blockStart := loc[1]
		blockContent := extractBlock(content, blockStart)
		if blockContent == "" {
			continue
		}

		sourceMatches := reModuleSource.FindStringSubmatch(blockContent)
		if len(sourceMatches) < 2 {
			continue
		}

		source := sourceMatches[1]
		if strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../") {
			targetName := filepath.Base(source)
			result.Relations = append(result.Relations, InfraRelation{
				From: modRes.ID(),
				To:   InfraResource{Kind: "TerraformModule", Name: targetName}.ID(),
				Type: "includes",
			})
		}
	}
}

// extractData extrai blocos "data" do conteúdo HCL.
func (a *TerraformAnalyzer) extractData(content, relPath string, result *AnalyzerResult) {
	matches := reData.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		dataType := m[1]
		dataName := m[2]
		result.Resources = append(result.Resources, InfraResource{
			Kind:     "TerraformData",
			APIGroup: "terraform",
			Name:     dataType + "." + dataName,
			Path:     relPath,
			Metadata: map[string]string{
				"type": dataType,
				"name": dataName,
			},
		})
	}
}

// extractBlock extrai o conteúdo entre o primeiro '{' após startPos e o '}' correspondente.
func extractBlock(content string, startPos int) string {
	rest := content[startPos:]
	braceStart := strings.Index(rest, "{")
	if braceStart == -1 {
		return ""
	}

	depth := 0
	for i := braceStart; i < len(rest); i++ {
		switch rest[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return rest[braceStart+1 : i]
			}
		}
	}

	return ""
}
