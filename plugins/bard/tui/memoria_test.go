package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/pkg/redacao"
	"github.com/casheiro/yby-cli/plugins/bard/tools"
)

func (p *provedorFalso) ultimaClassificacao() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.classificacoes) == 0 {
		return ""
	}
	return p.classificacoes[len(p.classificacoes)-1]
}

// A segunda pergunta ("e os logs dele?") chega à IA com a primeira troca da sessão, antes
// dela, na resposta e no classificador, que precisa do pod da troca anterior.
func TestFluxo_MemoriaDaConversa(t *testing.T) {
	isolarHome(t)
	p := &provedorFalso{trechos: []string{"o worker-1 reinicia por OOMKilled"}}
	f := fluxoDeTeste(p)

	fim := fimDe(t, coletar(t, f.Iniciar(context.Background(), 1, "por que o worker-1 reinicia?"), nil))
	if strings.Contains(p.ultimaEntrada(), "Conversa") {
		t.Errorf("a primeira pergunta não tem conversa anterior: %q", p.ultimaEntrada())
	}
	conversa := []Troca{TrocaDe("por que o worker-1 reinicia?", fim)}
	coletar(t, f.Iniciar(context.Background(), 2, "e os logs dele?", conversa...), nil)

	for nome, prompt := range map[string]string{"resposta": p.ultimaEntrada(), "classificador": p.ultimaClassificacao()} {
		for _, trecho := range []string{"por que o worker-1 reinicia?", "o worker-1 reinicia por OOMKilled", "e os logs dele?"} {
			if !strings.Contains(prompt, trecho) {
				t.Errorf("o prompt da %s deveria ter %q:\n%s", nome, trecho, prompt)
			}
		}
		if strings.Index(prompt, "OOMKilled") > strings.Index(prompt, "e os logs dele?") {
			t.Errorf("no prompt da %s, a troca anterior vem antes da pergunta:\n%s", nome, prompt)
		}
	}
}

// Com ferramenta e notas do Synapstor, a conversa entra entre as notas e a pergunta com o
// resultado da ferramenta.
func TestFluxo_MemoriaComFerramentaENotas(t *testing.T) {
	isolarHome(t)
	p := &provedorFalso{intencao: intencaoInvestigar(), trechos: []string{"ok"}}
	chamadas := 0
	f := fluxoDeTeste(p)
	f.Buscador = notasGrandes(1, 600)
	ferramenta := ferramentaDeTeste("saída da ferramenta", nil, &chamadas)
	f.BuscarFerramenta = func(string) *tools.Tool { return ferramenta }

	coletar(t, f.Iniciar(context.Background(), 2, "investiga ele", Troca{Pergunta: "quem é o worker-1?", Resposta: "um pod do api"}), nil)

	entrada := p.ultimaEntrada()
	notas, conversa, ferr := strings.Index(entrada, "Nota 1"), strings.Index(entrada, "um pod do api"), strings.Index(entrada, "saída da ferramenta")
	if notas < 0 || conversa < 0 || ferr < 0 || notas > conversa || conversa > ferr {
		t.Errorf("esperava notas, conversa e ferramenta nessa ordem:\n%s", entrada)
	}
}

// As trocas mais recentes que cabem no TetoConversa vão à IA, cada fala no TetoMensagem; as
// mais velhas ficam de fora, e nunca mais que MaxTrocas.
func TestFluxo_MemoriaNoTeto(t *testing.T) {
	isolarHome(t)
	p := &provedorFalso{trechos: []string{"ok"}}
	f := fluxoDeTeste(p)
	var conversa []Troca
	for i := 1; i <= 20; i++ {
		conversa = append(conversa, Troca{Pergunta: fmt.Sprintf("pergunta %02d", i), Resposta: fmt.Sprintf("resposta %02d\n", i) + linhasNumeradas(50, 100)})
	}

	coletar(t, f.Iniciar(context.Background(), 21, "e agora?", conversa...), nil)

	entrada := p.ultimaEntrada()
	if len(entrada) > TetoConversa+folgaPrompt {
		t.Errorf("a conversa deveria ficar no teto: %d bytes", len(entrada))
	}
	if !strings.Contains(entrada, "pergunta 20") || !strings.Contains(entrada, "resposta 20") || !strings.Contains(entrada, "linhas omitidas") {
		t.Errorf("a troca mais recente entra, cortada:\n%s", entrada)
	}
	if strings.Contains(entrada, "pergunta 01") {
		t.Error("as trocas mais velhas ficam de fora")
	}

	// Trocas curtas: só as últimas MaxTrocas.
	conversa = conversa[:0]
	for i := 1; i <= MaxTrocas+5; i++ {
		conversa = append(conversa, Troca{Pergunta: fmt.Sprintf("p%02d", i), Resposta: "r"})
	}
	coletar(t, f.Iniciar(context.Background(), 22, "e agora?", conversa...), nil)
	entrada = p.ultimaEntrada()
	if strings.Contains(entrada, "p05") || !strings.Contains(entrada, "p06") || !strings.Contains(entrada, fmt.Sprintf("p%02d", MaxTrocas+5)) {
		t.Errorf("deveriam ir as últimas %d trocas:\n%s", MaxTrocas, entrada)
	}
}

// A conversa passa pelo pkg/redacao antes de ir à IA, como o histórico gravado: um segredo
// que o usuário colou ou que a resposta repetiu não volta nas perguntas seguintes.
func TestFluxo_MemoriaRedigida(t *testing.T) {
	isolarHome(t)
	p := &provedorFalso{trechos: []string{"ok"}}
	f := fluxoDeTeste(p)
	segredo := "ghp_" + strings.Repeat("a1B2", 10)

	coletar(t, f.Iniciar(context.Background(), 2, "e agora?",
		Troca{Pergunta: "meu token é " + segredo, Resposta: "password: hunter2xyz"}), nil)

	for _, prompt := range []string{p.ultimaEntrada(), p.ultimaClassificacao()} {
		if strings.Contains(prompt, segredo) || strings.Contains(prompt, "hunter2xyz") || !strings.Contains(prompt, redacao.Marca) {
			t.Errorf("a conversa deveria ir redigida:\n%s", prompt)
		}
	}
}

// TrocaDe guarda a resposta da rodada: a interrompida leva a marca e a que falhou fica sem
// resposta, que a IA vê como tal.
func TestTrocaDe(t *testing.T) {
	if tr := TrocaDe("p", FimMsg{Resposta: "r"}); tr != (Troca{Pergunta: "p", Resposta: "r"}) {
		t.Errorf("rodada completa: %+v", tr)
	}
	if tr := TrocaDe("p", FimMsg{Resposta: "meia", Interrompida: true}); tr.Resposta != "meia\n\n"+marcaInterrompida {
		t.Errorf("rodada interrompida: %+v", tr)
	}
	if tr := TrocaDe("p", FimMsg{Resposta: "parcial", Err: errors.New("falhou")}); tr.Resposta != "" {
		t.Errorf("rodada que falhou: %+v", tr)
	}
	if m := memoriaDaConversa([]Troca{{Pergunta: "p"}}); !strings.Contains(m, "(sem resposta)") {
		t.Errorf("troca sem resposta: %q", m)
	}
	if m := memoriaDaConversa(nil); m != "" {
		t.Errorf("sem trocas, sem memória: %q", m)
	}
}

// Na TUI, a segunda pergunta leva a primeira troca; o /limpar começa sem memória; a
// sessão retomada vai pelo system prompt e não se repete na conversa.
func TestModel_MemoriaDaConversa(t *testing.T) {
	p := &provedorFalso{trechos: []string{"resposta do worker-1"}}
	m := modeloComFluxo(t, p, nil)
	var pedidas []string
	sessoes, retomar := historicoFalso(&pedidas)
	m.config.Sessoes, m.config.RetomarSessao = sessoes, retomar

	m, cmd := enviar(t, m, "por que o worker-1 reinicia?")
	m = rodar(t, m, cmd, nil)
	m, cmd = enviar(t, m, "e os logs dele?")
	m = rodar(t, m, cmd, nil)
	if e := p.ultimaEntrada(); !strings.Contains(e, "por que o worker-1 reinicia?") || !strings.Contains(e, "resposta do worker-1") {
		t.Errorf("a segunda pergunta deveria levar a primeira troca:\n%s", e)
	}
	if c := p.ultimaClassificacao(); !strings.Contains(c, "por que o worker-1 reinicia?") {
		t.Errorf("o classificador deveria ver a primeira troca:\n%s", c)
	}

	// Um comando / não vira troca.
	m = digitarComando(t, m, "/contexto")
	m, cmd = enviar(t, m, "e agora?")
	m = rodar(t, m, cmd, nil)
	if e := p.ultimaEntrada(); strings.Contains(e, "/contexto") || !strings.Contains(e, "e os logs dele?") {
		t.Errorf("a conversa leva as perguntas, não os comandos:\n%s", e)
	}

	m = digitarComando(t, m, "/limpar")
	m, cmd = enviar(t, m, "oi")
	m = rodar(t, m, cmd, nil)
	if e := p.ultimaEntrada(); e != "oi" {
		t.Errorf("depois do /limpar, a pergunta vai sem a conversa anterior: %q", e)
	}

	m = digitarComando(t, m, "/sessões 20261023-091000")
	m, cmd = enviar(t, m, "e o maior?")
	_ = rodar(t, m, cmd, nil)
	if e := p.ultimaEntrada(); e != "e o maior?" {
		t.Errorf("a sessão retomada vai pelo system prompt, sem se repetir na conversa: %q", e)
	}
}

// digitarComando digita o comando e aperta enter; ele roda sem rodada da IA.
func digitarComando(t *testing.T, m Model, texto string) Model {
	t.Helper()
	m, _ = teclarModelo(t, m, append(teclasDe(texto), teclaEnter)...)
	return m
}
