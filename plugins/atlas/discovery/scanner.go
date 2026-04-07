package discovery

import (
	"bufio"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/casheiro/yby-cli/plugins/atlas/discovery/analyzers"
)

// ShouldIgnore verifica se um caminho deve ser ignorado com base na lista de ignores.
// Compara cada segmento do caminho individualmente, evitando false positives
// como "my-vendor-lib" sendo ignorado quando "vendor" está na lista.
func ShouldIgnore(path string, ignores []string) bool {
	segments := strings.Split(filepath.ToSlash(path), "/")
	for _, seg := range segments {
		for _, ignore := range ignores {
			if seg == ignore {
				return true
			}
		}
	}
	return false
}

// Scan percorre o diretório e aplica as regras padrão para identificar componentes.
func Scan(root string, ignores []string) (*Blueprint, error) {
	return ScanWithRules(root, ignores, DefaultRules)
}

// ScanWithRules percorre o diretório e aplica as regras fornecidas para identificar componentes.
func ScanWithRules(root string, ignores []string, rules []Rule) (*Blueprint, error) {
	bp := &Blueprint{
		Components: []Component{},
		Roots:      []string{root},
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Verificar diretórios ignorados
		if ShouldIgnore(path, ignores) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Pular diretórios
		if d.IsDir() {
			return nil
		}

		// Verificar correspondência com as regras
		compType := MatchWithRules(d.Name(), rules)
		if compType != "" {
			dir := filepath.Dir(path)
			name := filepath.Base(dir)

			// Evitar duplicatas (verificação por caminho e tipo)
			exists := false
			for _, c := range bp.Components {
				if c.Path == dir && c.Type == compType {
					exists = true
					break
				}
			}

			if !exists {
				comp := Component{
					Name: name,
					Type: compType,
					Path: dir,
				}

				// Extrair nome do módulo de go.mod
				if d.Name() == "go.mod" {
					moduleName := extractGoModuleName(path)
					if moduleName != "" {
						comp.Metadata = map[string]string{
							"module": moduleName,
						}
					}
				}

				// Detectar linguagem e framework a partir do conteúdo do arquivo
				fw := DetectFramework(path)
				comp.Language = fw.Language
				comp.Framework = fw.Framework

				bp.Components = append(bp.Components, comp)
			}
		}

		return nil
	})

	if err != nil {
		return bp, err
	}

	// Segunda passagem: detectar relacionamentos entre componentes
	bp.Relations = detectRelations(root, bp.Components)

	return bp, nil
}

// extractGoModuleName lê um arquivo go.mod e extrai o nome do módulo.
func extractGoModuleName(goModPath string) string {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return ""
	}
	lines := strings.SplitN(string(data), "\n", 2)
	if len(lines) > 0 && strings.HasPrefix(lines[0], "module ") {
		return strings.TrimSpace(strings.TrimPrefix(lines[0], "module "))
	}
	return ""
}

// detectRelations analisa os componentes descobertos e identifica relacionamentos entre eles.
func detectRelations(root string, components []Component) []Relation {
	var relations []Relation

	// Mapa de caminhos de componentes para busca rápida
	compPaths := make(map[string]Component)
	for _, comp := range components {
		relPath, err := filepath.Rel(root, comp.Path)
		if err != nil {
			relPath = comp.Path
		}
		compPaths[relPath] = comp
	}

	for _, comp := range components {
		switch comp.Type {
		case "app":
			// Verificar go.mod por diretivas replace locais
			relations = append(relations, detectGoModRelations(comp, root, compPaths)...)
			// Verificar imports Go entre módulos do monorepo
			relations = append(relations, detectGoImportRelations(root, comp, compPaths)...)
			// Verificar package.json por dependências locais
			relations = append(relations, detectPackageJsonRelations(root, comp, compPaths)...)
		case "infra":
			// Verificar Dockerfile por referências COPY a outros componentes
			relations = append(relations, detectDockerfileRelations(comp, root, compPaths)...)
			// Verificar COPY --from referências entre stages e componentes
			relations = append(relations, detectDockerFromRelations(root, comp, compPaths)...)
		case "helm":
			// Verificar Chart.yaml por dependências locais
			relations = append(relations, detectHelmRelations(comp, root, compPaths)...)
			// Verificar Chart.yaml por dependências remotas
			relations = append(relations, detectHelmRemoteRelations(root, comp)...)
		}
	}

	return relations
}

// detectGoModRelations detecta relacionamentos a partir de diretivas replace em go.mod.
func detectGoModRelations(comp Component, root string, compPaths map[string]Component) []Relation {
	var relations []Relation
	goModPath := filepath.Join(comp.Path, "go.mod")

	file, err := os.Open(goModPath)
	if err != nil {
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Procurar por diretivas replace com caminhos locais (../ ou ./)
		if strings.HasPrefix(line, "replace") || strings.Contains(line, "=>") {
			parts := strings.Split(line, "=>")
			if len(parts) == 2 {
				target := strings.TrimSpace(parts[1])
				// Caminho local começa com ./ ou ../
				if strings.HasPrefix(target, "./") || strings.HasPrefix(target, "../") {
					absTarget := filepath.Join(comp.Path, target)
					relTarget, err := filepath.Rel(root, absTarget)
					if err == nil {
						if _, ok := compPaths[relTarget]; ok {
							compRel, _ := filepath.Rel(root, comp.Path)
							relations = append(relations, Relation{
								From: compRel,
								To:   relTarget,
								Type: "imports",
							})
						}
					}
				}
			}
		}
	}

	return relations
}

// detectDockerfileRelations detecta relacionamentos a partir de instruções COPY em Dockerfiles.
func detectDockerfileRelations(comp Component, root string, compPaths map[string]Component) []Relation {
	var relations []Relation

	// Procurar Dockerfiles no diretório do componente
	entries, err := os.ReadDir(comp.Path)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matched, _ := filepath.Match("Dockerfile*", entry.Name())
		if !matched && entry.Name() != "Dockerfile" {
			continue
		}

		dockerfilePath := filepath.Join(comp.Path, entry.Name())
		file, err := os.Open(dockerfilePath)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "COPY") || strings.HasPrefix(line, "ADD") {
				// Procurar referências a diretórios de outros componentes
				for relPath := range compPaths {
					if strings.Contains(line, relPath) {
						compRel, _ := filepath.Rel(root, comp.Path)
						if compRel != relPath {
							relations = append(relations, Relation{
								From: compRel,
								To:   relPath,
								Type: "builds",
							})
						}
					}
				}
			}
		}
		file.Close()
	}

	return relations
}

// detectHelmRelations detecta relacionamentos a partir de dependências em Chart.yaml.
func detectHelmRelations(comp Component, root string, compPaths map[string]Component) []Relation {
	var relations []Relation
	chartPath := filepath.Join(comp.Path, "Chart.yaml")

	file, err := os.Open(chartPath)
	if err != nil {
		return nil
	}
	defer file.Close()

	inDependencies := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "dependencies:" {
			inDependencies = true
			continue
		}

		// Sair da seção de dependências quando encontrar outra chave de nível superior
		if inDependencies && len(line) > 0 && line[0] != ' ' && line[0] != '\t' && !strings.HasPrefix(trimmed, "-") {
			inDependencies = false
		}

		if inDependencies && strings.Contains(trimmed, "repository:") {
			repo := strings.TrimSpace(strings.TrimPrefix(trimmed, "- repository:"))
			repo = strings.TrimPrefix(repo, "repository:")
			repo = strings.TrimSpace(repo)
			repo = strings.Trim(repo, "\"'")

			// Verificar se é referência local (file://)
			if strings.HasPrefix(repo, "file://") {
				localPath := strings.TrimPrefix(repo, "file://")
				absTarget := filepath.Join(comp.Path, localPath)
				relTarget, err := filepath.Rel(root, absTarget)
				if err == nil {
					if _, ok := compPaths[relTarget]; ok {
						compRel, _ := filepath.Rel(root, comp.Path)
						relations = append(relations, Relation{
							From: compRel,
							To:   relTarget,
							Type: "deploys",
						})
					}
				}
			}
		}
	}

	return relations
}

// detectGoImportRelations detecta relações de import Go entre módulos do monorepo.
func detectGoImportRelations(root string, comp Component, compPaths map[string]Component) []Relation {
	var relations []Relation

	// Ler go.mod do componente para obter module path
	goModPath := filepath.Join(comp.Path, "go.mod")
	rootModulePath := extractGoModuleName(goModPath)
	if rootModulePath == "" {
		return nil
	}

	// Obter module path do monorepo (go.mod na raiz)
	rootGoMod := extractGoModuleName(filepath.Join(root, "go.mod"))
	if rootGoMod == "" {
		// Usar o próprio module path como base do monorepo
		rootGoMod = rootModulePath
	}

	// Escanear arquivos .go no diretório do componente
	entries, err := os.ReadDir(comp.Path)
	if err != nil {
		return nil
	}

	importRe := regexp.MustCompile(`^\s*"([^"]+)"`)
	compRel, _ := filepath.Rel(root, comp.Path)

	// Coletar todos os imports do componente
	seenImports := make(map[string]bool)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		file, err := os.Open(filepath.Join(comp.Path, entry.Name()))
		if err != nil {
			continue
		}

		inImportBlock := false
		sc := bufio.NewScanner(file)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())

			if line == "import (" {
				inImportBlock = true
				continue
			}
			if inImportBlock && line == ")" {
				inImportBlock = false
				continue
			}

			if inImportBlock {
				matches := importRe.FindStringSubmatch(line)
				if len(matches) >= 2 {
					importPath := matches[1]
					if strings.HasPrefix(importPath, rootGoMod) && !seenImports[importPath] {
						seenImports[importPath] = true
						// Mapear import para componente conhecido
						for relPath, target := range compPaths {
							if target.Path == comp.Path {
								continue
							}
							targetMod := ""
							if target.Metadata != nil {
								targetMod = target.Metadata["module"]
							}
							if targetMod != "" && strings.HasPrefix(importPath, targetMod) {
								relations = append(relations, Relation{
									From: compRel,
									To:   relPath,
									Type: "imports",
								})
							}
						}
					}
				}
			}
		}
		file.Close()
	}

	return relations
}

// detectDockerFromRelations detecta relações a partir de COPY --from em Dockerfiles.
func detectDockerFromRelations(root string, comp Component, compPaths map[string]Component) []Relation {
	var relations []Relation

	entries, err := os.ReadDir(comp.Path)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matched, _ := filepath.Match("Dockerfile*", entry.Name())
		if !matched && entry.Name() != "Dockerfile" {
			continue
		}

		dockerfilePath := filepath.Join(comp.Path, entry.Name())
		file, err := os.Open(dockerfilePath)
		if err != nil {
			continue
		}

		// Mapear stages definidas com FROM ... AS <stage>
		stages := make(map[string]bool)
		fromRe := regexp.MustCompile(`(?i)^FROM\s+\S+\s+AS\s+(\S+)`)
		copyFromRe := regexp.MustCompile(`(?i)^COPY\s+--from=(\S+)`)

		// Primeira passagem: coletar stages e linhas
		sc := bufio.NewScanner(file)
		var lines []string
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			lines = append(lines, line)
			matches := fromRe.FindStringSubmatch(line)
			if len(matches) >= 2 {
				stages[strings.ToLower(matches[1])] = true
			}
		}
		file.Close()

		// Segunda passagem: encontrar COPY --from referenciando componentes
		compRel, _ := filepath.Rel(root, comp.Path)
		for _, line := range lines {
			matches := copyFromRe.FindStringSubmatch(line)
			if len(matches) < 2 {
				continue
			}
			fromRef := matches[1]
			// Se referencia uma stage interna, pular
			if stages[strings.ToLower(fromRef)] {
				continue
			}
			// Verificar se referencia um componente conhecido
			for relPath := range compPaths {
				if fromRef == relPath || fromRef == filepath.Base(relPath) {
					if compRel != relPath {
						relations = append(relations, Relation{
							From: compRel,
							To:   relPath,
							Type: "builds",
						})
					}
				}
			}
		}
	}

	return relations
}

// detectHelmRemoteRelations detecta dependências Helm com repositórios remotos (não file://).
func detectHelmRemoteRelations(root string, comp Component) []Relation {
	var relations []Relation
	chartPath := filepath.Join(comp.Path, "Chart.yaml")

	file, err := os.Open(chartPath)
	if err != nil {
		return nil
	}
	defer file.Close()

	inDependencies := false
	sc := bufio.NewScanner(file)
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "dependencies:" {
			inDependencies = true
			continue
		}

		if inDependencies && len(line) > 0 && line[0] != ' ' && line[0] != '\t' && !strings.HasPrefix(trimmed, "-") {
			inDependencies = false
		}

		if inDependencies && strings.Contains(trimmed, "repository:") {
			repo := strings.TrimSpace(strings.TrimPrefix(trimmed, "- repository:"))
			repo = strings.TrimPrefix(repo, "repository:")
			repo = strings.TrimSpace(repo)
			repo = strings.Trim(repo, "\"'")

			// Apenas repositórios remotos (não file://)
			if repo != "" && !strings.HasPrefix(repo, "file://") {
				compRel, _ := filepath.Rel(root, comp.Path)
				relations = append(relations, Relation{
					From: compRel,
					To:   repo,
					Type: "depends",
				})
			}
		}
	}

	return relations
}

// infraFileNames contém os nomes de arquivos que indicam infraestrutura.
var infraFileNames = map[string]string{
	"Chart.yaml":          "helm",
	"docker-compose.yml":  "compose",
	"docker-compose.yaml": "compose",
	"compose.yml":         "compose",
	"compose.yaml":        "compose",
	"kustomization.yaml":  "kustomize",
	"kustomization.yml":   "kustomize",
}

// k8sExcludeNames são nomes de arquivo que não devem ser parseados como manifests K8s standalone.
var k8sExcludeNames = map[string]bool{
	"Chart.yaml":          true,
	"values.yaml":         true,
	"kustomization.yaml":  true,
	"kustomization.yml":   true,
	"docker-compose.yml":  true,
	"docker-compose.yaml": true,
	"compose.yml":         true,
	"compose.yaml":        true,
}

// ScanInfra escaneia o projeto buscando topologia de infraestrutura.
// Usa analyzers especializados para cada tipo de arquivo de infra.
func ScanInfra(rootPath string, ignores []string) (*InfraBlueprint, error) {
	// 1. Coletar todos os arquivos relevantes
	var allFiles []string
	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ShouldIgnore(path, ignores) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		allFiles = append(allFiles, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 2. Classificar arquivos por analyzer
	classified := classifyFiles(allFiles)

	// 3. Rodar analyzers disponíveis
	allAnalyzers := []analyzers.Analyzer{
		analyzers.NewHelmAnalyzer(),
		analyzers.NewK8sAnalyzer(),
		analyzers.NewComposeAnalyzer(),
		analyzers.NewKustomizeAnalyzer(),
		analyzers.NewTerraformAnalyzer(),
	}

	bp := &InfraBlueprint{
		RootPath: rootPath,
	}

	for _, a := range allAnalyzers {
		files, ok := classified[a.Name()]
		if !ok || len(files) == 0 {
			continue
		}

		result, err := a.Analyze(rootPath, files)
		if err != nil {
			continue // analyzer falhou, seguir com os demais
		}
		if result == nil || (len(result.Resources) == 0 && len(result.Relations) == 0) {
			continue
		}

		bp.Resources = append(bp.Resources, result.Resources...)
		bp.Relations = append(bp.Relations, result.Relations...)
		bp.Analyzers = append(bp.Analyzers, a.Name())
	}

	return bp, nil
}

// classifyFiles distribui os arquivos coletados por tipo de analyzer.
func classifyFiles(files []string) map[string][]string {
	result := make(map[string][]string)

	// Primeiro passo: coletar Chart.yaml para saber quais diretórios são charts Helm
	helmChartDirs := make(map[string]bool)
	for _, f := range files {
		if filepath.Base(f) == "Chart.yaml" {
			helmChartDirs[filepath.Dir(f)] = true
		}
	}

	for _, f := range files {
		name := filepath.Base(f)
		ext := filepath.Ext(f)

		// Arquivos com nomes específicos
		if analyzerType, ok := infraFileNames[name]; ok {
			result[analyzerType] = append(result[analyzerType], f)
			continue
		}

		// Terraform
		if ext == ".tf" {
			result["terraform"] = append(result["terraform"], f)
			continue
		}

		// YAML que pode ser manifest K8s (excluindo os já classificados)
		if (ext == ".yaml" || ext == ".yml") && !k8sExcludeNames[name] {
			// Excluir templates Go (.tmpl), values, schemas
			if strings.HasSuffix(name, ".tmpl") || strings.HasPrefix(name, "values") ||
				strings.HasSuffix(name, ".schema.json") {
				continue
			}

			// Excluir YAML dentro de templates/ de Helm charts
			// (já são processados pelo Helm analyzer)
			if isInsideHelmTemplates(f, helmChartDirs) {
				continue
			}

			// Excluir arquivos encriptados por SOPS
			if isSopsEncrypted(f) {
				continue
			}

			result["k8s"] = append(result["k8s"], f)
		}
	}

	return result
}

// isInsideHelmTemplates verifica se um arquivo está dentro de um diretório templates/ de um Helm chart.
func isInsideHelmTemplates(filePath string, helmChartDirs map[string]bool) bool {
	dir := filepath.Dir(filePath)
	for {
		base := filepath.Base(dir)
		parent := filepath.Dir(dir)

		if base == "templates" {
			// Verificar se o pai é um diretório de chart Helm
			if helmChartDirs[parent] {
				return true
			}
		}

		if parent == dir {
			break
		}
		dir = parent
	}
	return false
}

// isSopsEncrypted verifica se um arquivo YAML está encriptado por SOPS.
// Detecta a presença de markers SOPS (ENC[AES256_GCM,...] ou campo sops:).
func isSopsEncrypted(filePath string) bool {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}
	content := string(data)
	// SOPS encripta valores com ENC[AES256_GCM,...] e adiciona metadata sops: no final
	return strings.Contains(content, "ENC[AES256_GCM,") || strings.Contains(content, "sops:\n")
}

// detectPackageJsonRelations detecta relações a partir de dependências locais em package.json.
func detectPackageJsonRelations(root string, comp Component, compPaths map[string]Component) []Relation {
	var relations []Relation
	pkgPath := filepath.Join(comp.Path, "package.json")

	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return nil
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	compRel, _ := filepath.Rel(root, comp.Path)

	allDeps := make(map[string]string)
	for k, v := range pkg.Dependencies {
		allDeps[k] = v
	}
	for k, v := range pkg.DevDependencies {
		allDeps[k] = v
	}

	for _, version := range allDeps {
		var localPath string
		if strings.HasPrefix(version, "file:") {
			localPath = strings.TrimPrefix(version, "file:")
		} else if strings.HasPrefix(version, "workspace:") {
			localPath = strings.TrimPrefix(version, "workspace:")
		} else {
			continue
		}

		absTarget := filepath.Join(comp.Path, localPath)
		relTarget, err := filepath.Rel(root, absTarget)
		if err != nil {
			continue
		}

		if _, ok := compPaths[relTarget]; ok {
			if compRel != relTarget {
				relations = append(relations, Relation{
					From: compRel,
					To:   relTarget,
					Type: "imports",
				})
			}
		}
	}

	return relations
}
