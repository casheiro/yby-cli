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

// All retorna todas as ferramentas registradas.
func All() []*Tool {
	mu.RLock()
	defer mu.RUnlock()
	result := make([]*Tool, 0, len(registry))
	for _, t := range registry {
		result = append(result, t)
	}
	return result
}

// FormatToolsPrompt gera a descrição das ferramentas disponíveis para injeção no system prompt.
func FormatToolsPrompt() string {
	mu.RLock()
	defer mu.RUnlock()

	if len(registry) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Ferramentas Disponíveis\n\n")
	sb.WriteString("Para executar uma ferramenta, responda com um bloco JSON:\n")
	sb.WriteString("```json\n{\"tool\": \"nome_da_ferramenta\", \"params\": {\"chave\": \"valor\"}}\n```\n\n")
	sb.WriteString("Só use ferramentas quando necessário. Responda diretamente quando possível.\n\n")

	for _, tool := range registry {
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

// Reset limpa o registry. Usado apenas em testes.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	registry = make(map[string]*Tool)
}
