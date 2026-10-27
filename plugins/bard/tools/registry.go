package tools

import (
	"fmt"
	"strings"
	"sync"
)

var (
	mu       sync.RWMutex
	registry = make(map[string]*Tool)
)

// Register registra uma ferramenta no registry global.
func Register(tool *Tool) {
	mu.Lock()
	defer mu.Unlock()
	registry[tool.Name] = tool
}

// Get retorna uma ferramenta pelo nome, ou nil se não encontrada.
func Get(name string) *Tool {
	mu.RLock()
	defer mu.RUnlock()
	return registry[name]
}

// All retorna todas as ferramentas registradas, por nome.
func All() []*Tool {
	mu.RLock()
	defer mu.RUnlock()
	return ordenadas()
}

// FormatToolsPrompt gera a descrição das ferramentas disponíveis para injeção no system
// prompt. Só descreve: quem escolhe e roda a ferramenta é o classificador do Bard, antes
// da resposta, e nada do que a IA escrever na resposta é executado.
func FormatToolsPrompt() string {
	mu.RLock()
	defer mu.RUnlock()

	if len(registry) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, tool := range ordenadas() {
		sb.WriteString(fmt.Sprintf("### %s\n", tool.Name))
		sb.WriteString(fmt.Sprintf("%s\n", tool.Description))
		if len(tool.Parameters) > 0 {
			sb.WriteString("Parâmetros:\n")
			for _, p := range tool.Parameters {
				req := "opcional"
				if p.Required {
					req = "obrigatório"
				}
				sb.WriteString(fmt.Sprintf("- `%s` (%s): %s\n", p.Name, req, p.Description))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// FindByIntent retorna a primeira tool que corresponde ao intent.
func FindByIntent(intent string) *Tool {
	mu.RLock()
	defer mu.RUnlock()
	for _, tool := range ordenadas() {
		for _, i := range tool.Intents {
			if i == intent {
				return tool
			}
		}
	}
	return nil
}

// Reset limpa o registry. Usado apenas em testes.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	registry = make(map[string]*Tool)
	invalidas = nil
}
