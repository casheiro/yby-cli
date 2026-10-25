package ui

// Glifos agrupa os símbolos usados pelas TUIs e saídas de comando.
type Glifos struct {
	Ativo       string // ativo, foco, etapa atual, marca
	Pendente    string // pendente, não visitado
	Andamento   string // em andamento
	Feito       string // concluído, saudável
	Falha       string // erro, falhou
	Alerta      string // aviso
	Rodando     string // rodando
	Parado      string // parado, vazio
	Cheio       string // trecho concluído
	Tracejado   string // trecho restante
	Caminho     string // separador de caminho
	Linha       string // fio fino (régua)
	Prompt      string // prompt de campo de texto
	Reticencias string // truncamento (sempre 1 célula)
	Vazio       string // campo sem valor numa tabela ou num detalhe
	Spinner     []string
}

// Unicode é o conjunto padrão de glifos.
var Unicode = Glifos{
	Ativo: "◆", Pendente: "◇", Andamento: "◈", Feito: "✓", Falha: "✕", Alerta: "▲",
	Rodando: "●", Parado: "○", Cheio: "━", Tracejado: "┄", Caminho: "▸", Linha: "─", Prompt: "›", Reticencias: "…", Vazio: "—",
	Spinner: []string{"◇", "◈", "◆", "◈"},
}

// ASCII é o conjunto de fallback para terminais sem UTF-8; cada glifo ocupa a mesma largura do Unicode.
var ASCII = Glifos{
	Ativo: "*", Pendente: "o", Andamento: "+", Feito: "v", Falha: "x", Alerta: "!",
	Rodando: "*", Parado: "o", Cheio: "=", Tracejado: "-", Caminho: ">", Linha: "-", Prompt: ">", Reticencias: "~", Vazio: "-",
	Spinner: []string{"o", "+", "*", "+"},
}

// G devolve o conjunto de glifos do modo atual (Unicode ou ASCII).
func G() Glifos {
	if Atual().ASCII {
		return ASCII
	}
	return Unicode
}
