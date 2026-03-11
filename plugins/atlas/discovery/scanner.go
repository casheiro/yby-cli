package discovery

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

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
		for _, ignore := range ignores {
			if strings.Contains(path, ignore) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
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
		case "infra":
			// Verificar Dockerfile por referências COPY a outros componentes
			relations = append(relations, detectDockerfileRelations(comp, root, compPaths)...)
		case "helm":
			// Verificar Chart.yaml por dependências
			relations = append(relations, detectHelmRelations(comp, root, compPaths)...)
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
