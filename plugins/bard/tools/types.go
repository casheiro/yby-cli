package tools

import "context"

// Tool define uma ferramenta que o Bard pode invocar.
type Tool struct {
	Name        string
	Description string
	Intents     []string // palavras-chave/padrões que ativam esta ferramenta
	Parameters  []ToolParam
	Execute     func(ctx context.Context, params map[string]string) (string, error)
}

// IntentResult é o resultado da classificação de intenção pela IA.
type IntentResult struct {
	Intent string            `json:"intent"`
	Params map[string]string `json:"params"`
	Direct bool              `json:"direct"`
}

// ToolParam descreve um parâmetro de uma ferramenta.
type ToolParam struct {
	Name        string
	Description string
	Required    bool
}

// ToolCall representa uma invocação de ferramenta extraída da resposta da IA.
type ToolCall struct {
	Name   string            `json:"tool"`
	Params map[string]string `json:"params"`
}

// ToolResult contém o resultado da execução de uma ferramenta.
type ToolResult struct {
	ToolName string `json:"tool_name"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
}
