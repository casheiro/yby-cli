package analysis

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ErrNaoEhMermaid é o erro de ValidarMermaid quando o texto nem começa como um flowchart
// (prosa, resposta vazia), para separar "sem diagrama" de "diagrama que não confere".
var ErrNaoEhMermaid = errors.New("o texto não começa com flowchart ou graph")

var (
	// textoEntreAspas casa os rótulos entre aspas, que podem trazer setas e colchetes.
	textoEntreAspas = regexp.MustCompile(`"[^"]*"`)
	// rotuloDeForma casa o rótulo de um nó com forma ([x], [(x)], ((x)), {{x}}...), que
	// pode trazer & e setas sem aspas.
	rotuloDeForma = regexp.MustCompile(`\[[^\]]*\]+|\([^)]*\)+|\{[^}]*\}+`)
)

// semRotulos troca os rótulos de uma linha pela abertura da forma: sobram os IDs, as
// setas e o caractere que marca o nó como declarado.
func semRotulos(linha string) string {
	linha = textoEntreAspas.ReplaceAllString(linha, `""`)
	return rotuloDeForma.ReplaceAllStringFunc(linha, func(m string) string { return m[:1] })
}

// leituraMermaid é o que ValidarMermaid tira de um flowchart, linha a linha.
type leituraMermaid struct {
	nos        map[string]bool // IDs usados como nó
	declarados map[string]bool // nós com forma (id[...], id(...), ...) e subgraphs
	subgraphs  map[string]bool
	emArestas  map[string]bool // IDs nas pontas de alguma aresta
	ordem      []string        // IDs na ordem em que aparecem, para mensagens estáveis
}

// lerMermaid lê um flowchart no formato que o Atlas e o prompt atlas.refine produzem.
// Devolve erro quando o cabeçalho não é flowchart/graph ou quando subgraph e end não
// fecham. Não é um parser completo do Mermaid.
func lerMermaid(texto string) (*leituraMermaid, error) {
	l := &leituraMermaid{
		nos: map[string]bool{}, declarados: map[string]bool{},
		subgraphs: map[string]bool{}, emArestas: map[string]bool{},
	}
	usar := func(id string) {
		if !l.nos[id] && !l.subgraphs[id] {
			l.ordem = append(l.ordem, id)
		}
	}
	cabecalho, profundidade := false, 0
	for _, linha := range strings.Split(texto, "\n") {
		linha = strings.TrimSpace(linha)
		if linha == "" || strings.HasPrefix(linha, "%%") {
			continue
		}
		campos := strings.Fields(linha)
		if !cabecalho {
			if campos[0] != "flowchart" && campos[0] != "graph" {
				return nil, fmt.Errorf("%w: %q", ErrNaoEhMermaid, linha)
			}
			cabecalho = true
			continue
		}
		switch campos[0] {
		case "subgraph":
			profundidade++
			if id := idMermaid.FindString(strings.TrimSpace(strings.TrimPrefix(linha, "subgraph"))); id != "" {
				usar(id)
				l.subgraphs[id] = true
				l.declarados[id] = true
			}
			continue
		case "end":
			profundidade--
			if profundidade < 0 {
				return nil, fmt.Errorf("end sem subgraph aberto")
			}
			continue
		case "flowchart", "graph", "style", "classDef", "class", "linkStyle", "direction", "click":
			continue
		}
		trechos := setaMermaid.Split(semRotulos(linha), -1)
		for _, t := range trechos {
			for _, parte := range strings.Split(t, "&") {
				parte = strings.TrimSpace(parte)
				id := idMermaid.FindString(parte)
				if id == "" {
					continue
				}
				usar(id)
				if !l.subgraphs[id] {
					l.nos[id] = true
				}
				if resto := parte[len(id):]; resto != "" && strings.ContainsRune("[({>", rune(resto[0])) {
					l.declarados[id] = true
				}
				if len(trechos) > 1 {
					l.emArestas[id] = true
				}
			}
		}
	}
	if !cabecalho {
		return nil, ErrNaoEhMermaid
	}
	if profundidade > 0 {
		return nil, fmt.Errorf("%d subgraph sem end", profundidade)
	}
	return l, nil
}

// ValidarMermaid confere se o flowchart refinado pela IA pode substituir o rascunho:
// começa com flowchart ou graph, fecha cada subgraph com end, declara (com forma ou como
// subgraph) os nós das arestas e só usa como nó os IDs dos nós do rascunho. Devolve o
// primeiro problema encontrado.
func ValidarMermaid(texto, rascunho string) error {
	l, err := lerMermaid(texto)
	if err != nil {
		return err
	}
	if len(l.nos) == 0 {
		return fmt.Errorf("o diagrama não tem nenhum nó")
	}
	base, err := lerMermaid(rascunho)
	if err != nil {
		return fmt.Errorf("rascunho ilegível: %v", err)
	}
	var fora []string
	for _, id := range l.ordem {
		if l.nos[id] && !l.subgraphs[id] && !base.nos[id] {
			fora = append(fora, id)
		}
	}
	if len(fora) > 0 {
		return fmt.Errorf("nós fora do rascunho: %s", resumirIDs(fora))
	}
	var soltos []string
	for _, id := range l.ordem {
		if l.emArestas[id] && !l.declarados[id] {
			soltos = append(soltos, id)
		}
	}
	if len(soltos) > 0 {
		return fmt.Errorf("arestas com nós não declarados: %s", resumirIDs(soltos))
	}
	return nil
}

// resumirIDs junta até cinco IDs em ordem, com a contagem do resto.
func resumirIDs(ids []string) string {
	ids = append([]string(nil), ids...)
	sort.Strings(ids)
	if len(ids) <= 5 {
		return strings.Join(ids, ", ")
	}
	return fmt.Sprintf("%s e mais %d", strings.Join(ids[:5], ", "), len(ids)-5)
}
