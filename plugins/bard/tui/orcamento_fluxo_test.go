package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/ai/prompts"
	"github.com/casheiro/yby-cli/plugins/bard/tools"
)

// folgaPrompt é o texto fixo do prompt final em volta dos blocos (a pergunta e as frases
// que apresentam as notas e a ferramenta).
const folgaPrompt = 512

func intencaoInvestigar() string {
	return `{"intent":"investigate_pod","params":{"pod":"worker-1"},"direct":false}`
}

// notasGrandes são n notas do Synapstor de tamanho bytes cada, todas acima do limiar.
func notasGrandes(n, tamanho int) buscadorFalso {
	var docs []ai.UnknownDocument
	for i := 1; i <= n; i++ {
		docs = append(docs, ai.UnknownDocument{
			Content:  linhasNumeradas(tamanho/100, 100),
			Score:    0.9,
			Metadata: map[string]string{"title": fmt.Sprintf("Nota %d", i)},
		})
	}
	return buscadorFalso{docs: docs}
}

// Uma ferramenta que devolve 200 KB chega ao provider e ao "↵ ver" no teto, com o
// começo, o fim e "… N linhas omitidas".
func TestFluxo_SaidaGrandeDaFerramentaCortada(t *testing.T) {
	isolarHome(t)
	p := &provedorFalso{intencao: intencaoInvestigar(), trechos: []string{"ok"}}
	chamadas := 0
	f := fluxoDeTeste(p)
	ferramenta := ferramentaDeTeste(linhasNumeradas(2000, 100), nil, &chamadas)
	f.BuscarFerramenta = func(string) *tools.Tool { return ferramenta }

	eventos := coletar(t, f.Iniciar(context.Background(), 1, "por que o worker reinicia?"), nil)

	entrada := p.ultimaEntrada()
	if len(entrada) > TetoFerramenta+folgaMarca+folgaPrompt {
		t.Errorf("o prompt deveria levar a saída no teto: %d bytes", len(entrada))
	}
	for _, trecho := range []string{"linha 00001 ", "linha 02000 ", "linhas omitidas"} {
		if !strings.Contains(entrada, trecho) {
			t.Errorf("o prompt deveria ter %q", trecho)
		}
	}
	if saida := ultimaEtapa(t, eventos, EtapaFerramenta).Saida; !strings.Contains(saida, "linhas omitidas") {
		t.Error("o \"↵ ver\" mostra o que a IA recebeu, com o corte")
	}
}

// As notas do Synapstor somadas ficam no teto: a que não cabe inteira entra cortada, e as
// seguintes ficam de fora; a nota da etapa conta só as que entraram.
func TestFluxo_NotasDoSynapstorNoTeto(t *testing.T) {
	isolarHome(t)
	p := &provedorFalso{trechos: []string{"ok"}}
	f := fluxoDeTeste(p)
	f.Buscador = notasGrandes(5, 6000)

	eventos := coletar(t, f.Iniciar(context.Background(), 1, "como faço o deploy?"), nil)

	entrada := p.ultimaEntrada()
	if len(entrada) > TetoNotas+folgaMarca+folgaPrompt {
		t.Errorf("as notas deveriam ficar no teto: %d bytes", len(entrada))
	}
	if !strings.Contains(entrada, "Nota 1") || !strings.Contains(entrada, "Nota 2") {
		t.Errorf("as notas mais relevantes deveriam entrar (a segunda cortada): %q", entrada[:200])
	}
	if strings.Contains(entrada, "Nota 3") {
		t.Error("a nota que não cabe deveria ficar de fora")
	}
	if n := ultimaEtapa(t, eventos, EtapaContexto).Nota; n != "2 notas do Synapstor" {
		t.Errorf("a etapa deveria contar as notas que entraram: %q", n)
	}
}

// Ponta a ponta com a menor janela que o Bard aceita (llama3, 8192 tokens): o system
// prompt de verdade com o cluster e o histórico no teto, notas cheias e 40 KB de
// ferramenta cabem no TokenAwareProvider. Sem o orçamento, era ERR_TOKEN_LIMIT.
func TestFluxo_CabeNaJanelaDoLlama3(t *testing.T) {
	isolarHome(t)
	p := &provedorFalso{intencao: intencaoInvestigar(), trechos: []string{"ok"}}
	chamadas := 0
	f := fluxoDeTeste(p)
	f.Provider = ai.NewTokenAwareProvider(p, "llama3")
	f.SystemPrompt = prompts.GetWithVars("bard.system", map[string]string{
		"tools_prompt":    tools.FormatToolsPrompt(),
		"cluster_context": strings.Repeat("c", TetoCluster),
	}) + strings.Repeat("h", TetoHistorico)
	f.Buscador = notasGrandes(5, 3000)
	ferramenta := ferramentaDeTeste(linhasNumeradas(400, 100), nil, &chamadas) // 40 KB
	f.BuscarFerramenta = func(string) *tools.Tool { return ferramenta }

	eventos := coletar(t, f.Iniciar(context.Background(), 1, "por que o worker reinicia?"), nil)

	if fim := fimDe(t, eventos); fim.Err != nil {
		t.Fatalf("o prompt deveria caber na janela do llama3: %v", fim.Err)
	}
	if p.totalChamadas() != 1 {
		t.Errorf("a resposta deveria ser pedida uma vez, foi %d", p.totalChamadas())
	}
}
