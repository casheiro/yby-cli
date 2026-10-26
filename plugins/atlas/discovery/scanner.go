package discovery

import (
	"bufio"
	"encoding/json"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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

	filtro := novoFiltro(root, ignores)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Verificar diretórios ignorados (pelo caminho relativo à raiz) e o .gitignore
		if filtro.pular(path, d) {
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
	bp.Relations = detectRelations(root, bp.Components, filtro)

	// O blueprint sai com caminhos relativos ao root e com '/', iguais em
	// qualquer SO e sem expor o diretório da máquina que fez o scan.
	for i := range bp.Components {
		if rel, err := relSlash(root, bp.Components[i].Path); err == nil {
			bp.Components[i].Path = rel
		}
	}
	bp.Roots = []string{"."}

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
func detectRelations(root string, components []Component, filtro *filtroVarredura) []Relation {
	var relations []Relation

	// Mapa de caminhos de componentes para busca rápida
	compPaths := make(map[string]Component)
	for _, comp := range components {
		relPath, err := relSlash(root, comp.Path)
		if err != nil {
			relPath = comp.Path
		}
		// App e infra na mesma pasta dividem o caminho: o app fica, porque
		// carrega o módulo Go usado para casar os imports.
		if existente, ok := compPaths[relPath]; ok && existente.Type == "app" {
			continue
		}
		compPaths[relPath] = comp
	}

	for _, comp := range components {
		switch comp.Type {
		case "app":
			// Verificar go.mod por diretivas replace locais
			relations = append(relations, detectGoModRelations(comp, root, compPaths)...)
			// Verificar imports Go entre módulos do monorepo
			relations = append(relations, detectGoImportRelations(root, comp, compPaths, filtro)...)
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

	return deduplicarRelacoes(relations)
}

// deduplicarRelacoes remove as relações repetidas (mesmos from, to e type),
// mantendo a ordem da primeira ocorrência.
func deduplicarRelacoes(relations []Relation) []Relation {
	vistas := make(map[Relation]bool, len(relations))
	var saida []Relation
	for _, r := range relations {
		if vistas[r] {
			continue
		}
		vistas[r] = true
		saida = append(saida, r)
	}
	return saida
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
					relTarget, err := relSlash(root, absTarget)
					if err == nil {
						if _, ok := compPaths[relTarget]; ok {
							compRel, _ := relSlash(root, comp.Path)
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
			if strings.HasPrefix(line, "COPY ") || strings.HasPrefix(line, "ADD ") {
				compRel, _ := relSlash(root, comp.Path)
				// Casa as origens do COPY, por segmento de caminho, com os componentes
				for _, origem := range origensDoCopy(line) {
					for _, relPath := range componenteDaOrigem(origem, compRel, compPaths) {
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
				relTarget, err := relSlash(root, absTarget)
				if err == nil {
					if _, ok := compPaths[relTarget]; ok {
						compRel, _ := relSlash(root, comp.Path)
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

// origensDoCopy extrai as origens de uma instrução COPY/ADD (tudo menos o
// destino), já normalizadas com '/'. COPY --from=… não vem para cá: é de outro
// stage ou imagem e tem detector próprio. Origens com variável, URL ou caminho
// absoluto são descartadas.
func origensDoCopy(linha string) []string {
	campos := strings.Fields(linha)
	var args []string
	for _, c := range campos[1:] {
		if strings.HasPrefix(c, "--") {
			if strings.HasPrefix(c, "--from") {
				return nil
			}
			continue
		}
		args = append(args, c)
	}
	if len(args) < 2 {
		return nil
	}
	var origens []string
	for _, a := range args[:len(args)-1] {
		a = filepath.ToSlash(strings.Trim(a, "\"'[],"))
		if a == "" || strings.ContainsAny(a, "$*?") || strings.Contains(a, "://") || strings.HasPrefix(a, "/") {
			continue
		}
		origens = append(origens, path.Clean(a))
	}
	return origens
}

// componenteDaOrigem devolve os caminhos de componente que contêm a origem do
// COPY, casando por segmento ("api" não casa com "apigateway") e nunca a raiz
// ("."). A origem vale relativa à raiz do projeto e à pasta do Dockerfile; para
// cada uma fica só o componente mais específico, e o próprio componente do
// Dockerfile e seus ancestrais são descartados (um "COPY . ." dentro de
// services/api2 não aponta para services).
func componenteDaOrigem(origem, compRel string, compPaths map[string]Component) []string {
	candidatos := []string{origem}
	if compRel != "." {
		candidatos = append(candidatos, path.Join(compRel, origem))
	}
	unicos := map[string]bool{}
	for _, c := range candidatos {
		melhor := ""
		for relPath := range compPaths {
			if relPath == "." {
				continue
			}
			if (c == relPath || strings.HasPrefix(c, relPath+"/")) && len(relPath) > len(melhor) {
				melhor = relPath
			}
		}
		if melhor == "" || compRel == melhor || strings.HasPrefix(compRel, melhor+"/") {
			continue
		}
		unicos[melhor] = true
	}
	achados := make([]string, 0, len(unicos))
	for relPath := range unicos {
		achados = append(achados, relPath)
	}
	sort.Strings(achados)
	return achados
}

// detectGoImportRelations detecta relações de import Go entre módulos do monorepo.
// Lê os .go de todo o módulo do componente (cmd/, internal/ e demais pastas),
// sem entrar em módulos aninhados, vendor, testdata, pastas ocultas nem no que
// o filtro da varredura (ignores e .gitignore) pula.
func detectGoImportRelations(root string, comp Component, compPaths map[string]Component, filtro *filtroVarredura) []Relation {
	if extractGoModuleName(filepath.Join(comp.Path, "go.mod")) == "" {
		return nil
	}
	compRel, _ := relSlash(root, comp.Path)

	// Módulos conhecidos, com o caminho do componente que os declara
	type modulo struct{ path, rel string }
	var modulos []modulo
	for relPath, c := range compPaths {
		if m := c.Metadata["module"]; m != "" {
			modulos = append(modulos, modulo{m, relPath})
		}
	}

	var relations []Relation
	_ = filepath.WalkDir(comp.Path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if filtro != nil && filtro.pular(p, d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if p == comp.Path {
				return nil
			}
			nome := d.Name()
			if nome == "vendor" || nome == "testdata" || strings.HasPrefix(nome, ".") {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(p, "go.mod")); err == nil {
				return filepath.SkipDir // módulo aninhado é outro componente
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		arq, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.ImportsOnly)
		if err != nil || arq == nil {
			return nil
		}
		for _, imp := range arq.Imports {
			importPath, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			// O módulo mais longo com fronteira '/' é o dono do import
			melhor := modulo{}
			for _, m := range modulos {
				if (importPath == m.path || strings.HasPrefix(importPath, m.path+"/")) && len(m.path) > len(melhor.path) {
					melhor = m
				}
			}
			if melhor.path == "" || melhor.rel == compRel {
				continue
			}
			relations = append(relations, Relation{From: compRel, To: melhor.rel, Type: "imports"})
		}
		return nil
	})

	sort.SliceStable(relations, func(i, j int) bool { return relations[i].To < relations[j].To })
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
		compRel, _ := relSlash(root, comp.Path)
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
				compRel, _ := relSlash(root, comp.Path)
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
	filtro := novoFiltro(rootPath, ignores)
	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if filtro.pular(path, d) {
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
		RootPath: ".",
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

	ordenarBlueprint(bp)
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

			// Arquivos encriptados por SOPS são descartados pelo K8sAnalyzer,
			// que já lê o arquivo.

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

	compRel, _ := relSlash(root, comp.Path)

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
		relTarget, err := relSlash(root, absTarget)
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

// ordenarBlueprint fixa a ordem dos recursos e das relações. Os analyzers
// percorrem mapas, cuja ordem muda a cada execução; sem isto o diagrama
// sairia diferente a cada vez.
func ordenarBlueprint(bp *InfraBlueprint) {
	sort.SliceStable(bp.Resources, func(i, j int) bool {
		a, b := bp.Resources[i], bp.Resources[j]
		if a.ID() != b.ID() {
			return a.ID() < b.ID()
		}
		return a.Path < b.Path
	})
	sort.SliceStable(bp.Relations, func(i, j int) bool {
		a, b := bp.Relations[i], bp.Relations[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.To != b.To {
			return a.To < b.To
		}
		return a.Type < b.Type
	})
}

// relSlash devolve o caminho de target relativo a base, sempre com '/', para que
// o blueprint seja igual no Windows e no Linux.
func relSlash(base, target string) (string, error) {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}
