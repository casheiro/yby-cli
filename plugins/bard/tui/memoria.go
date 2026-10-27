package tui

import (
	"fmt"
	"strings"

	"github.com/casheiro/yby-cli/pkg/redacao"
)

// Troca é uma pergunta da conversa atual e a resposta que ela teve. As trocas anteriores
// vão com a pergunta nova, para a IA entender "e os logs dele?".
type Troca struct {
	Pergunta, Resposta string
}

// MaxTrocas é o máximo de trocas da conversa atual que vão à IA; o TetoConversa decide
// quantas delas cabem.
const MaxTrocas = 10

// rotuloConversa abre o bloco da conversa atual no prompt da resposta e no do classificador.
const rotuloConversa = "Conversa até aqui (sessão atual):"

// TrocaDe é a troca de uma rodada encerrada. A resposta interrompida vai com a marca, como
// no histórico salvo, e a que falhou fica vazia.
func TrocaDe(pergunta string, fim FimMsg) Troca {
	switch {
	case fim.Interrompida:
		return Troca{Pergunta: pergunta, Resposta: textoInterrompido(fim.Resposta)}
	case fim.Err != nil:
		return Troca{Pergunta: pergunta}
	}
	return Troca{Pergunta: pergunta, Resposta: fim.Resposta}
}

// memoriaDaConversa escreve as últimas trocas que cabem no TetoConversa, da mais velha para
// a mais nova, cada fala redigida (pkg/redacao, como o histórico gravado) e cortada no
// TetoMensagem; vazio sem trocas. A saída das ferramentas não entra: a resposta já diz o
// que importava dela.
func memoriaDaConversa(trocas []Troca) string {
	if len(trocas) > MaxTrocas {
		trocas = trocas[len(trocas)-MaxTrocas:]
	}
	blocos := make([]string, len(trocas))
	tamanhos := make([]int, len(trocas))
	for i, t := range trocas {
		resposta := t.Resposta
		if strings.TrimSpace(resposta) == "" {
			resposta = "(sem resposta)"
		}
		blocos[i] = fmt.Sprintf("Usuário: %s\nAssistente: %s\n",
			Cortar(redacao.Redigir(t.Pergunta), TetoMensagem), Cortar(redacao.Redigir(resposta), TetoMensagem))
		tamanhos[i] = len(blocos[i]) + 1 // com a linha em branco entre as trocas
	}
	return strings.Join(blocos[len(blocos)-Recentes(tamanhos, TetoConversa):], "\n")
}

// comConversa põe a conversa atual antes do texto; sem conversa, o texto fica igual.
func comConversa(conversa, texto string) string {
	if conversa == "" {
		return texto
	}
	return rotuloConversa + "\n" + conversa + "\n" + texto
}
