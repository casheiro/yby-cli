package main

import (
	"io"

	"github.com/casheiro/yby-cli/v5/pkg/ui"
)

// flagAjuda é a linha do -h nas seções de flags, como no core.
var flagAjuda = ui.Termo{Nome: "-h, --help", Descricao: "mostra esta ajuda"}

// ajudaSynapstor é a ajuda de yby synapstor, sem subcomando.
func ajudaSynapstor() ui.Ajuda {
	return ui.Ajuda{
		Comando: "yby synapstor",
		Descricao: "Gestão de conhecimento do projeto: transforma texto e código em UKIs (unidades de " +
			"conhecimento em markdown, em .synapstor/.uki), indexa os UKIs com embeddings para a busca " +
			"semântica e o Bard, e confere a qualidade e a idade de cada um.",
		Uso: []string{"yby synapstor <comando> [flags]"},
		Comandos: []ui.Termo{
			{Nome: "capture \"texto\"", Descricao: "estrutura o texto num UKI usando IA"},
			{Nome: "study \"tópico\"", Descricao: "lê o código sobre o tópico e documenta num UKI usando IA"},
			{Nome: "search \"consulta\"", Descricao: "busca semântica nos UKIs indexados"},
			{Nome: "index", Descricao: "indexa os UKIs com embeddings (só o que mudou)"},
			{Nome: "quality", Descricao: "dá uma nota de 0 a 100 a cada UKI"},
			{Nome: "decay", Descricao: "aponta os UKIs sem atividade no git há mais de 90 dias ou com o código citado alterado"},
			{Nome: "export", Descricao: "exporta os UKIs para Docusaurus, Obsidian ou markdown"},
		},
		Exemplos: `
			yby synapstor capture "o deploy do payments passa pelo Argo CD com sync automático"
			yby synapstor study "port-forward"
			yby synapstor index
			yby synapstor search "como fazer rollback" --top-k 3
			yby synapstor export --format obsidian`,
		Flags:  []ui.Termo{flagAjuda},
		Rodape: "yby synapstor <comando> --help mostra os detalhes de cada um",
	}
}

// ajudas são as ajudas dos subcomandos; as chaves são também a lista dos subcomandos.
var ajudas = map[string]func() ui.Ajuda{
	"capture": func() ui.Ajuda {
		return ui.Ajuda{
			Comando: "yby synapstor capture",
			Descricao: "Estrutura um texto livre (uma decisão, um procedimento, um aprendizado) num UKI " +
				"usando IA e o grava em .synapstor/.uki.",
			Uso: []string{"yby synapstor capture \"texto\""},
			Exemplos: `
				yby synapstor capture "o worker do payments precisa do DB_HOST apontando para o Service do Postgres"`,
			Flags:  []ui.Termo{flagAjuda},
			Rodape: "Depois, yby synapstor index põe o UKI novo na busca e no Bard.",
		}
	},
	"study": func() ui.Ajuda {
		return ui.Ajuda{
			Comando: "yby synapstor study",
			Descricao: "Procura no código do projeto os arquivos que falam do tópico, pede à IA a " +
				"documentação deles e a grava como UKI em .synapstor/.uki.",
			Uso: []string{"yby synapstor study \"tópico ou arquivo\""},
			Exemplos: `
				yby synapstor study "port-forward"
				yby synapstor study pkg/mirror`,
			Flags:  []ui.Termo{flagAjuda},
			Rodape: "Depois, yby synapstor index põe o UKI novo na busca e no Bard.",
		}
	},
	"search": func() ui.Ajuda {
		return ui.Ajuda{
			Comando: "yby synapstor search",
			Descricao: "Busca semântica nos UKIs indexados: mostra os mais parecidos com a consulta, " +
				"com a pontuação (de 0 a 1) e o caminho de cada um. Ficam de fora os abaixo de 0,35.",
			Uso: []string{"yby synapstor search \"consulta\" [flags]"},
			Exemplos: `
				yby synapstor search "como fazer rollback"
				yby synapstor search "argo cd" --top-k 10`,
			Flags: []ui.Termo{
				{Nome: "    --top-k número", Descricao: "quantos resultados buscar (padrão 5)"},
				flagAjuda,
			},
			Rodape: "A busca usa o índice de yby synapstor index.",
		}
	},
	"index": func() ui.Ajuda {
		return ui.Ajuda{
			Comando: "yby synapstor index",
			Descricao: "Indexa com embeddings os UKIs de .synapstor/.uki e as páginas de docs/wiki. Só " +
				"entra o que mudou desde a última vez, e os arquivos apagados saem do índice.",
			Uso: []string{"yby synapstor index [flags]"},
			Exemplos: `
				yby synapstor index
				yby synapstor index --full`,
			Flags: []ui.Termo{
				{Nome: "    --full", Descricao: "indexa tudo de novo, mesmo o que não mudou"},
				flagAjuda,
			},
			Rodape: "Os embeddings vêm do Ollama, se ele estiver rodando, ou do Gemini e da OpenAI configurados " +
				"em ai.embedding; sem eles, de um modelo local que entende português, baixado no primeiro uso " +
				"(~120 MB, em ~/.yby/models). Os CLIs claude e gemini não geram embeddings.",
		}
	},
	"quality": func() ui.Ajuda {
		return ui.Ajuda{
			Comando: "yby synapstor quality",
			Descricao: "Dá uma nota de 0 a 100 a cada UKI de .synapstor/.uki: seção de contexto, exemplos " +
				"em blocos de código, seções, tamanho do texto, links e metadados (ID, tipo e status). " +
				"De 70 para cima é bom; abaixo de 40, fraco.",
			Uso:   []string{"yby synapstor quality"},
			Flags: []ui.Termo{flagAjuda},
		}
	},
	"decay": func() ui.Ajuda {
		return ui.Ajuda{
			Comando: "yby synapstor decay",
			Descricao: "Aponta os UKIs de .synapstor/.uki sem atividade no git há mais de 90 dias, ou cujos " +
				"arquivos citados tiveram commits depois da nota; podem estar desatualizados.",
			Uso:   []string{"yby synapstor decay"},
			Flags: []ui.Termo{flagAjuda},
		}
	},
	"export": func() ui.Ajuda {
		return ui.Ajuda{
			Comando:   "yby synapstor export",
			Descricao: "Exporta os UKIs de .synapstor/.uki para um site Docusaurus, um cofre do Obsidian ou markdown puro.",
			Uso:       []string{"yby synapstor export [flags]"},
			Exemplos: `
				yby synapstor export
				yby synapstor export --format docusaurus --output site/docs`,
			Flags: []ui.Termo{
				{Nome: "    --format formato", Descricao: "docusaurus, obsidian ou markdown (padrão markdown)"},
				{Nome: "    --output pasta", Descricao: "pasta de destino (padrão .synapstor/export/<formato>)"},
				flagAjuda,
			},
		}
	},
}

// escreverAjuda escreve em w a ajuda do subcomando (vazio é a do Synapstor).
func escreverAjuda(w io.Writer, subcomando string) {
	if f, ok := ajudas[subcomando]; ok {
		ui.EscreverAjuda(w, f())
		return
	}
	ui.EscreverAjuda(w, ajudaSynapstor())
}

// pedeAjuda informa se args pedem a ajuda.
func pedeAjuda(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return true
		}
	}
	return false
}
