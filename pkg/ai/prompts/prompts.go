package prompts

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/casheiro/yby-cli/pkg/confianca"
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
// 1. .yby/prompts/{nome}.txt (projeto — maior precedência, só com a confiança do usuário)
// 2. ~/.yby/prompts/{nome}.txt (global)
// 3. Default embarcado
//
// O override do projeto reescreve o que a IA recebe, e um repositório clonado pode
// trazê-lo: ele só vale depois de "yby plugin trust .yby/prompts/{nome}.txt" e enquanto
// o arquivo não mudar. Sem confiança, fica de fora com um aviso no log.
func Get(name string) string {
	// 1. Override do projeto
	if content := readProjeto(filepath.Join(".yby", "prompts", name+".txt")); content != "" {
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

// avisados guarda os overrides do projeto sem confiança já avisados, para o aviso sair
// uma vez por arquivo e não a cada pergunta.
var avisados sync.Map

// readProjeto lê o override do projeto, ou vazio se ele não existir ou não tiver a
// confiança do usuário. A confiança confere o conteúdo lido, o mesmo que é usado.
func readProjeto(path string) string {
	data, err := os.ReadFile(path)
	if err != nil || strings.TrimSpace(string(data)) == "" {
		return ""
	}
	if !confianca.Confiavel(path, data) {
		if _, ja := avisados.LoadOrStore(path, true); !ja {
			slog.Warn("prompt do projeto ignorado: sem confiança (ou mudou depois dela)",
				"arquivo", filepath.ToSlash(path),
				"dica", "rode yby plugin trust "+filepath.ToSlash(path))
		}
		return ""
	}
	return strings.TrimSpace(string(data))
}

// readFile lê um arquivo e retorna seu conteúdo, ou vazio se não existir.
func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
