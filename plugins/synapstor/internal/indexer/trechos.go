package indexer

import (
	"regexp"
	"strings"
)

const (
	// tamanhoTrecho é o tamanho alvo, em runas, do corpo de um trecho; seção maior é
	// repartida em janelas desse tamanho.
	tamanhoTrecho = 1500
	// sobreposicao é quanto do fim de uma janela se repete no começo da seguinte, para a
	// frase cortada na divisa aparecer inteira em algum trecho.
	sobreposicao = 200
)

// reH2 acha as linhas de título de segundo nível.
var reH2 = regexp.MustCompile(`(?m)^## `)

// Trecho é um pedaço de um documento que vira um vetor no índice.
type Trecho struct {
	// Texto vai ao embedding: título do documento e da seção, e o corpo.
	Texto string
	// Secao é o título H2 do trecho; vazio no que vem antes do primeiro H2.
	Secao string
}

// dividirEmTrechos parte o documento por títulos H2. O que passa de tamanhoTrecho numa
// seção é repartido em janelas com sobreposição, cortadas de preferência em quebra de
// parágrafo. Cada trecho começa com o título do documento (H1) e o da seção, para o
// vetor e o texto devolvido pela busca saberem de onde vêm. Documento vazio não gera
// trecho.
func dividirEmTrechos(titulo, conteudo string) []Trecho {
	conteudo = strings.ReplaceAll(conteudo, "\r\n", "\n")

	var secoes []string
	inicio := 0
	for _, loc := range reH2.FindAllStringIndex(conteudo, -1) {
		if loc[0] > inicio {
			secoes = append(secoes, conteudo[inicio:loc[0]])
		}
		inicio = loc[0]
	}
	secoes = append(secoes, conteudo[inicio:])

	var trechos []Trecho
	for _, s := range secoes {
		secao := ""
		corpo := s
		if strings.HasPrefix(s, "## ") {
			cabeca, resto, _ := strings.Cut(s, "\n")
			secao = strings.TrimSpace(strings.TrimPrefix(cabeca, "## "))
			corpo = resto
		}
		corpo = strings.TrimSpace(corpo)
		// só o título do documento, sem texto: não é conteúdo para buscar
		if corpo == "" || (secao == "" && strings.HasPrefix(corpo, "# ") && !strings.Contains(corpo, "\n")) {
			continue
		}
		prefixo := ""
		if titulo != "" && !strings.HasPrefix(corpo, "# ") {
			prefixo = "# " + titulo + "\n"
		}
		if secao != "" {
			prefixo += "## " + secao + "\n"
		}
		for _, janela := range janelas(corpo) {
			trechos = append(trechos, Trecho{Texto: prefixo + janela, Secao: secao})
		}
	}
	return trechos
}

// janelas reparte o texto em pedaços de até tamanhoTrecho runas, cada um repetindo as
// últimas sobreposicao runas do anterior. O corte cai na última quebra de parágrafo (ou
// de linha) da segunda metade da janela, quando houver.
func janelas(texto string) []string {
	runas := []rune(texto)
	if len(runas) <= tamanhoTrecho {
		return []string{texto}
	}
	var saida []string
	inicio := 0
	for inicio < len(runas) {
		fim := min(inicio+tamanhoTrecho, len(runas))
		if fim < len(runas) {
			fim = melhorCorte(runas, inicio, fim)
		}
		if p := strings.TrimSpace(string(runas[inicio:fim])); p != "" {
			saida = append(saida, p)
		}
		if fim >= len(runas) {
			break
		}
		inicio = max(fim-sobreposicao, inicio+1)
	}
	return saida
}

// melhorCorte procura, de trás para frente na segunda metade da janela, uma quebra de
// parágrafo e depois uma de linha; sem nenhuma, corta no limite.
func melhorCorte(runas []rune, inicio, fim int) int {
	piso := inicio + (fim-inicio)/2
	for i := fim - 1; i > piso; i-- {
		if runas[i] == '\n' && runas[i-1] == '\n' {
			return i + 1
		}
	}
	for i := fim - 1; i > piso; i-- {
		if runas[i] == '\n' {
			return i + 1
		}
	}
	return fim
}
