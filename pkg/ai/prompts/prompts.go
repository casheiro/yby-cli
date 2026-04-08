package prompts

import (
	"os"
	"path/filepath"
	"strings"
)

// defaultPrompts mapeia nomes padronizados aos prompts default.
var defaultPrompts = map[string]string{
	"bard.system":          BardSystem,
	"bard.classify":        BardClassify,
	"sentinel.investigate": SentinelInvestigate,
	"sentinel.scan":        SentinelScan,
	"synapstor.capture":    SynapsotorCapture,
	"synapstor.study":      SynapsotorStudy,
	"synapstor.tagger":     SynapsotorTagger,
	"atlas.refine":         AtlasRefine,
	"governance.system":    GovernanceSystem,
}

// Get retorna o prompt pelo nome, aplicando overrides na ordem:
// 1. .yby/prompts/{nome}.txt (projeto — maior precedência)
// 2. ~/.yby/prompts/{nome}.txt (global)
// 3. Default embarcado
func Get(name string) string {
	// 1. Override do projeto
	if content := readFile(filepath.Join(".yby", "prompts", name+".txt")); content != "" {
		return content
	}

	// 2. Override global
	if home, err := os.UserHomeDir(); err == nil {
		if content := readFile(filepath.Join(home, ".yby", "prompts", name+".txt")); content != "" {
			return content
		}
	}

	// 3. Default embarcado
	if p, ok := defaultPrompts[name]; ok {
		return p
	}

	return ""
}

// GetWithVars retorna o prompt com variáveis {{var}} substituídas pelos valores fornecidos.
func GetWithVars(name string, vars map[string]string) string {
	prompt := Get(name)
	for key, value := range vars {
		prompt = strings.ReplaceAll(prompt, "{{"+key+"}}", value)
	}
	return prompt
}

// List retorna os nomes de todos os prompts disponíveis.
func List() []string {
	names := make([]string, 0, len(defaultPrompts))
	for name := range defaultPrompts {
		names = append(names, name)
	}
	return names
}

// readFile lê um arquivo e retorna seu conteúdo, ou vazio se não existir.
func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
