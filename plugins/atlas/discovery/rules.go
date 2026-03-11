package discovery

import "path/filepath"

// DefaultRules define as regras padrão de correspondência.
var DefaultRules = []Rule{
	{MatchFile: "Chart.yaml", Type: "helm"},
	{MatchFile: "kustomization.yaml", Type: "kustomize"},
	{MatchGlob: "Dockerfile*", Type: "infra"},
	{MatchFile: "go.mod", Type: "app"},
	{MatchFile: "package.json", Type: "app"},
	{MatchFile: "Dockerfile", Type: "infra"},
	{MatchFile: "Taskfile.yml", Type: "config"},
	{MatchFile: "Makefile", Type: "config"},
}

// Match verifica um nome de arquivo contra as regras padrão e retorna o tipo correspondente.
func Match(filename string) string {
	return MatchWithRules(filename, DefaultRules)
}

// MatchWithRules verifica um nome de arquivo contra um conjunto de regras e retorna o tipo correspondente.
// Suporta correspondência exata (MatchFile) e padrões glob (MatchGlob).
func MatchWithRules(filename string, rules []Rule) string {
	for _, rule := range rules {
		if rule.MatchFile != "" && filename == rule.MatchFile {
			return rule.Type
		}
		if rule.MatchGlob != "" {
			matched, err := filepath.Match(rule.MatchGlob, filename)
			if err == nil && matched {
				return rule.Type
			}
		}
	}
	return ""
}

// MergeRules combina regras customizadas com as regras padrão.
// Regras customizadas têm precedência e são adicionadas no início da lista.
func MergeRules(custom []RuleConfig) []Rule {
	if len(custom) == 0 {
		return DefaultRules
	}

	merged := make([]Rule, 0, len(custom)+len(DefaultRules))
	for _, rc := range custom {
		merged = append(merged, Rule{
			MatchFile: rc.MatchFile,
			MatchGlob: rc.MatchGlob,
			Type:      rc.Type,
		})
	}
	merged = append(merged, DefaultRules...)
	return merged
}
