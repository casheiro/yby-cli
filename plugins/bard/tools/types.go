package tools

import "context"

// Tool define uma ferramenta que o Bard pode invocar.
type Tool struct {
	Name        string
	Description string
	Parameters  []ToolParam
	Execute     func(ctx context.Context, params map[string]string) (string, error)
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
