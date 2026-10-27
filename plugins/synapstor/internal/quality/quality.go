// Package quality implementa scoring de qualidade para documentos UKI.
package quality

import (
	"os"
	"regexp"
	"strings"

	"github.com/casheiro/yby-cli/plugins/synapstor/internal/uki"
)

// QualityBreakdown detalha os critérios avaliados.
type QualityBreakdown struct {
	HasContext  bool `json:"has_context"`
	HasExamples bool `json:"has_examples"`
	HasHeaders  bool `json:"has_headers"`
	WordCount   int  `json:"word_count"`
	LinkCount   int  `json:"link_count"`
	HeaderCount int  `json:"header_count"`
	HasMetadata bool `json:"has_metadata"`
	// Tipo é o tipo do UKI quando a especificação tem padrão de seções para ele.
	Tipo string `json:"tipo,omitempty"`
	// SecoesFaltando lista as seções do padrão do tipo que a nota não tem.
	SecoesFaltando []string `json:"secoes_faltando,omitempty"`
	// ExigeExemplos indica que o tipo pede um exemplo (bloco de código); nos outros
	// tipos o bloco de código não conta a favor nem contra.
	ExigeExemplos bool `json:"exige_exemplos"`
}

// QualityScore representa a pontuação de qualidade de um UKI.
type QualityScore struct {
	Path      string           `json:"path"`
	Title     string           `json:"title"`
	Score     int              `json:"score"`
	Breakdown QualityBreakdown `json:"breakdown"`
}

var reHeader = regexp.MustCompile(`(?m)^#{2,}\s+.+$`)
var reCodeBlock = regexp.MustCompile("(?s)```[\\s\\S]*?```")
var reLink = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)

// reContexto acha a seção que abre um UKI na especificação (Contexto nas regras e
// métricas, Problema nas decisões, Quando usar nos padrões, Visão geral nas
// referências), em qualquer caixa, em qualquer nível de seção e com numeração.
var reContexto = regexp.MustCompile(`(?mi)^#{2,}\s+(?:\d+[.)]?\s*)?(contexto|context|problema|problem|quando usar|vis[aã]o geral|overview)\b`)

// secao é uma seção esperada no padrão de um tipo: o nome mostrado ao usuário e o
// regex do título, com os nomes em PT-BR e em inglês dos UKIs antigos.
type secao struct {
	nome string
	re   *regexp.Regexp
}

func novaSecao(nome, alternativas string) secao {
	return secao{nome: nome, re: regexp.MustCompile(`(?mi)^#{2,}\s+(?:\d+[.)]?\s*)?(?:` + alternativas + `)\b`)}
}

// secoesDoTipo são as seções de cada tipo, como em 2 do UKI_SPEC.md.
var secoesDoTipo = map[string][]secao{
	uki.TipoRegra: {
		novaSecao("Contexto", `contexto|context`),
		novaSecao("Enunciado", `enunciado|statement|rule`),
		novaSecao("Exceções", `exce[cç][oõ]es|exceptions`),
	},
	uki.TipoDecisao: {
		novaSecao("Problema", `problema|problem|contexto|context`),
		novaSecao("Opções", `op[cç][oõ]es|options|alternativas|alternatives`),
		novaSecao("Decisão", `decis[aã]o|decision`),
		novaSecao("Consequências", `consequ[eê]ncias|consequences`),
	},
	uki.TipoPadrao: {
		novaSecao("Quando usar", `quando usar|when to use`),
		novaSecao("Exemplo", `exemplos?|examples?|template`),
		novaSecao("Antipadrão", `antipadr[aã]o|anti-?pattern`),
	},
	uki.TipoMetrica: {
		novaSecao("Contexto", `contexto|context`),
		novaSecao("Definição", `defini[cç][aã]o|definition`),
		novaSecao("Meta", `meta|target|goal`),
	},
	uki.TipoReferencia: {
		novaSecao("Visão geral", `vis[aã]o geral|overview|contexto|context`),
		novaSecao("Arquitetura", `arquitetura|architecture`),
		novaSecao("Referências no código", `refer[eê]ncias no c[oó]digo|code references`),
	},
}

// ScoreUKI avalia a qualidade de um documento UKI. Os metadados saem do frontmatter (ou
// das linhas em negrito do formato antigo); o resto da nota olha só o corpo.
func ScoreUKI(content string) QualityScore {
	score := QualityScore{}
	meta, content := uki.Ler(content)
	score.Title = meta.Titulo

	// Verificar contexto
	score.Breakdown.HasContext = reContexto.MatchString(content)
	if score.Breakdown.HasContext {
		score.Score += 15
	}

	// Exemplos (blocos de código): só o padrão pede; nos outros tipos não pesam.
	score.Breakdown.HasExamples = reCodeBlock.MatchString(content)
	secoes, tipado := secoesDoTipo[meta.Tipo]
	score.Breakdown.ExigeExemplos = meta.Tipo == uki.TipoPadrao || !tipado
	if !score.Breakdown.ExigeExemplos || score.Breakdown.HasExamples {
		score.Score += 10
	}

	// Estrutura: as seções do padrão do tipo ou, sem tipo, a quantidade de seções (H2+)
	headers := reHeader.FindAllString(content, -1)
	score.Breakdown.HeaderCount = len(headers)
	score.Breakdown.HasHeaders = len(headers) > 0
	if tipado {
		score.Breakdown.Tipo = meta.Tipo
		achadas := 0
		for _, sec := range secoes {
			if sec.re.MatchString(content) {
				achadas++
			} else {
				score.Breakdown.SecoesFaltando = append(score.Breakdown.SecoesFaltando, sec.nome)
			}
		}
		score.Score += 30 * achadas / len(secoes)
	} else {
		score.Score += min(30, 8*len(headers))
	}

	// Word count
	words := strings.Fields(content)
	score.Breakdown.WordCount = len(words)
	switch {
	case len(words) > 500:
		score.Score += 20
	case len(words) > 300:
		score.Score += 15
	case len(words) > 100:
		score.Score += 10
	}

	// Links/referências
	links := reLink.FindAllString(content, -1)
	score.Breakdown.LinkCount = len(links)
	if len(links) > 0 {
		score.Score += 10
	}

	// Metadados (ID, tipo e status)
	score.Breakdown.HasMetadata = meta.Completos()
	if score.Breakdown.HasMetadata {
		score.Score += 15
	}

	return score
}

// ScoreAll avalia todos os UKIs de um diretório e das subpastas de domínio.
func ScoreAll(ukiDir string) ([]QualityScore, error) {
	arquivos, err := uki.Listar(ukiDir)
	if err != nil {
		return nil, err
	}

	var scores []QualityScore
	for _, a := range arquivos {
		data, err := os.ReadFile(a.Caminho)
		if err != nil {
			continue
		}

		qs := ScoreUKI(string(data))
		qs.Path = a.Caminho
		scores = append(scores, qs)
	}

	return scores, nil
}
