package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Orçamento do que vai à IA, em bytes de cada bloco do prompt (o TokenAwareProvider estima
// ~4 bytes por token). Com a menor janela que o Bard aceita (8192 tokens, a do llama3 e
// dos modelos que o pkg/ai não conhece), o system prompt (~2,5 KB) com o cluster e o
// histórico, as notas, a conversa atual, a saída da ferramenta e a pergunta cabem juntos
// com todos os blocos cheios em ~85% da janela: passam dos 80%, em que o TokenAwareProvider
// avisa no log, e ficam abaixo dos 90%, em que ele recusa.
const (
	// TetoFerramenta é o máximo da saída de uma ferramenta, ou do erro dela.
	TetoFerramenta = 8 * 1024
	// TetoNotas é o máximo das notas do Synapstor somadas.
	TetoNotas = 8 * 1024
	// TetoHistorico é o máximo do histórico das conversas anteriores no system prompt. A
	// conversa atual, que vale mais para a pergunta, tem o TetoConversa.
	TetoHistorico = 2 * 1024
	// TetoConversa é o máximo das trocas da conversa atual que vão com a pergunta; cabe
	// sempre a última troca inteira (pergunta e resposta no TetoMensagem).
	TetoConversa = 3 * 1024
	// TetoMensagem é o máximo de cada mensagem do histórico e da conversa, para uma
	// resposta longa não ocupar o bloco inteiro.
	TetoMensagem = 1024
	// TetoCluster é o máximo do estado do cluster no system prompt.
	TetoCluster = 3 * 1024
)

// minimoNota é o menor espaço que ainda vale um trecho de nota cortado.
const minimoNota = 512

// Cortar devolve o texto em no máximo teto bytes, mais a marca do corte. O corte é por
// linhas: ficam as primeiras e as últimas (o cabeçalho de uma lista e o fim de um log), e
// "… N linhas omitidas" entra no lugar das do meio. Uma linha maior que metade do teto
// (um JSON numa linha só) é cortada antes, sem partir um caractere, e termina em "…".
func Cortar(texto string, teto int) string {
	if teto <= 0 || len(texto) <= teto {
		return texto
	}
	final := ""
	if strings.HasSuffix(texto, "\n") {
		final = "\n"
	}
	linhas := strings.Split(strings.TrimSuffix(texto, "\n"), "\n")
	metade := teto / 2
	for i, l := range linhas {
		if len(l) > metade {
			linhas[i] = CortarLinha(l, metade)
		}
	}

	// O começo fica com metade do teto e o fim com o resto; cada linha conta com a quebra.
	usado, ini, fim := 0, 0, len(linhas)-1
	for ini <= fim && usado+len(linhas[ini])+1 <= metade {
		usado += len(linhas[ini]) + 1
		ini++
	}
	for fim >= ini && usado+len(linhas[fim])+1 <= teto {
		usado += len(linhas[fim]) + 1
		fim--
	}
	omitidas := fim - ini + 1
	if omitidas <= 0 {
		return strings.Join(linhas, "\n") + final
	}
	partes := append([]string{}, linhas[:ini]...)
	partes = append(partes, marcaOmitidas(omitidas))
	partes = append(partes, linhas[fim+1:]...)
	return strings.Join(partes, "\n") + final
}

// marcaOmitidas é a linha que entra no lugar das linhas cortadas.
func marcaOmitidas(n int) string {
	if n == 1 {
		return "… 1 linha omitida"
	}
	return fmt.Sprintf("… %d linhas omitidas", n)
}

// CortarLinha corta a linha em no máximo max bytes, com a marca " …", num limite de
// caractere. A linha que cabe fica igual.
func CortarLinha(l string, max int) string {
	if len(l) <= max {
		return l
	}
	const marca = " …"
	n := max - len(marca)
	if n < 0 {
		n = 0
	}
	for n > 0 && !utf8.RuneStart(l[n]) {
		n--
	}
	return l[:n] + marca
}

// Recentes devolve quantos dos últimos itens, pelos tamanhos dados em ordem, cabem em teto
// bytes. A conta vai do mais novo para o mais velho e para no primeiro que não cabe, para
// não pular uma mensagem no meio da conversa.
func Recentes(tamanhos []int, teto int) int {
	usado := 0
	for i := len(tamanhos) - 1; i >= 0; i-- {
		if usado+tamanhos[i] > teto {
			return len(tamanhos) - 1 - i
		}
		usado += tamanhos[i]
	}
	return len(tamanhos)
}
