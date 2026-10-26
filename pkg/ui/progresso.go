package ui

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// OpcoesProgresso ajusta um Progresso; os campos vazios usam o comportamento padrão.
type OpcoesProgresso struct {
	// Terminal diz se w é um terminal; nil usa Largura(w) > 0.
	Terminal func(w io.Writer) bool
	// Relogio fornece a hora das durações; nil usa time.Now.
	Relogio func() time.Time
	// Largura total das linhas em células; <= 0 usa a das saídas de comando (64, ou a
	// do terminal quando ele é mais estreito).
	Largura int
}

// Progresso acompanha as etapas de um comando longo (yby up, bootstrap) e as escreve no
// formato de Steps: ✓ feita com a duração, ◈ atual (com cota quando há fração) e ◇
// pendente.
//
// Num terminal, as etapas que já terminaram ficam fixas na tela e o trecho de baixo (a
// atual e as pendentes) é redesenhado no lugar a cada mudança. Fora dele (pipe, CI), cada
// etapa sai uma vez, numa linha, quando termina, sem códigos de controle. É seguro para
// uso concorrente (ex.: um tique que avança a cota enquanto a etapa espera).
type Progresso struct {
	mu      sync.Mutex
	w       io.Writer
	tty     bool
	relogio func() time.Time
	largura int

	etapas  []Etapa
	inicios []time.Time
	atual   int  // índice da etapa em andamento; -1 sem nenhuma
	fixas   int  // etapas do começo já escritas de vez
	vivas   int  // linhas do trecho redesenhável na tela (só no terminal)
	pausas  int  // Pausar em vigor: sem trecho redesenhável enquanto > 0
	falhou  bool // a última etapa encerrada falhou: as pendentes saem da tela
}

// NovoProgresso cria um Progresso que escreve em w; rotulos são as etapas planejadas,
// mostradas como pendentes no terminal até começarem.
func NovoProgresso(w io.Writer, rotulos ...string) *Progresso {
	return NovoProgressoCom(w, OpcoesProgresso{}, rotulos...)
}

// NovoProgressoCom é NovoProgresso com detecção de terminal, relógio e largura escolhidos
// pelo chamador (testes, ou largura fixa).
func NovoProgressoCom(w io.Writer, o OpcoesProgresso, rotulos ...string) *Progresso {
	terminal := o.Terminal
	if terminal == nil {
		terminal = func(w io.Writer) bool { return Largura(w) > 0 }
	}
	relogio := o.Relogio
	if relogio == nil {
		relogio = time.Now
	}
	largura := o.Largura
	if largura <= 0 {
		largura = larguraLinha(w)
	}
	p := &Progresso{w: w, tty: terminal(w), relogio: relogio, largura: largura, atual: -1}
	for _, r := range rotulos {
		p.etapas = append(p.etapas, Etapa{Rotulo: r, Estado: EtapaPendente})
		p.inicios = append(p.inicios, time.Time{})
	}
	return p
}

// Terminal informa se o Progresso redesenha no lugar (w é um terminal).
func (p *Progresso) Terminal() bool { return p.tty }

// Iniciar começa a etapa rotulo: a primeira pendente com esse rótulo ou, se não houver,
// uma nova no fim da lista. Uma etapa ainda em andamento é concluída antes, sem nota.
func (p *Progresso) Iniciar(rotulo string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	agora := p.relogio()
	if p.atual >= 0 {
		p.terminar(EtapaFeita, "", agora)
	}
	i := -1
	for j := p.fixas; j < len(p.etapas); j++ {
		if p.etapas[j].Estado == EtapaPendente && p.etapas[j].Rotulo == rotulo {
			i = j
			break
		}
	}
	if i < 0 {
		p.etapas = append(p.etapas, Etapa{Rotulo: rotulo})
		p.inicios = append(p.inicios, time.Time{})
		i = len(p.etapas) - 1
	}
	// a etapa vai para logo depois das que já terminaram, para a ordem na tela seguir a
	// ordem em que rodaram
	if alvo := p.primeiraNaoFinal(); i != alvo {
		e, ini := p.etapas[i], p.inicios[i]
		copy(p.etapas[alvo+1:i+1], p.etapas[alvo:i])
		copy(p.inicios[alvo+1:i+1], p.inicios[alvo:i])
		p.etapas[alvo], p.inicios[alvo] = e, ini
		i = alvo
	}
	p.etapas[i].Estado = EtapaAtual
	p.etapas[i].Nota, p.etapas[i].Fracao, p.etapas[i].Duracao = "", 0, 0
	p.inicios[i] = agora
	p.atual = i
	p.falhou = false
	p.desenhar(agora)
}

// Avancar define a fração (0 a 1) da etapa atual, que passa a aparecer como cota.
func (p *Progresso) Avancar(fracao float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	agora := p.relogio()
	if p.atual < 0 {
		return
	}
	p.etapas[p.atual].Fracao = limitarFracao(fracao)
	p.desenhar(agora)
}

// Tique redesenha a etapa atual com o tempo corrido até agora (só no terminal).
func (p *Progresso) Tique() {
	p.mu.Lock()
	defer p.mu.Unlock()
	agora := p.relogio()
	if p.atual >= 0 {
		p.desenhar(agora)
	}
}

// Concluir encerra a etapa atual com sucesso; nota sai em TintaSuave depois do rótulo.
func (p *Progresso) Concluir(nota string) { p.encerrar(EtapaFeita, nota) }

// Falhar encerra a etapa atual com erro. No terminal, as pendentes saem da tela (como
// fora dele, onde nunca aparecem): o erro que vem a seguir explica onde o comando parou.
// Se outra etapa começar depois, elas voltam.
func (p *Progresso) Falhar(nota string) { p.encerrar(EtapaFalha, nota) }

// Pular encerra a etapa atual sem sucesso nem erro (○): um passo opcional que não trouxe
// nada (ex.: métricas indisponíveis). O comando segue, e as pendentes continuam na tela.
func (p *Progresso) Pular(nota string) { p.encerrar(EtapaPulada, nota) }

func (p *Progresso) encerrar(estado EstadoEtapa, nota string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	agora := p.relogio()
	if p.atual < 0 {
		return
	}
	p.terminar(estado, nota, agora)
	p.falhou = estado == EtapaFalha
	p.desenhar(agora)
}

// Detalhe escreve texto (ex.: a saída de um script que falhou) abaixo das etapas que já
// terminaram, recuado em 4 células e em TintaSuave; o trecho redesenhável continua abaixo.
func (p *Progresso) Detalhe(texto string) {
	texto = strings.TrimRight(texto, "\n")
	if texto == "" {
		return
	}
	s := S()
	p.foraDoQuadro(func() {
		for _, l := range strings.Split(texto, "\n") {
			fmt.Fprintln(p.w, "    "+s.TintaSuave.Render(strings.TrimRight(l, "\r")))
		}
	})
}

// Avisar escreve um alerta (▲, como ui.Check com CheckAviso) abaixo das etapas que já
// terminaram, para o que não interrompe o comando mas pede atenção (ex.: o Argo CD ainda
// não convergiu). O texto é cortado na largura das linhas. Fora do terminal, a etapa em
// andamento só sai quando termina: chame Avisar depois de Concluir para manter a ordem.
func (p *Progresso) Avisar(texto string) {
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return
	}
	p.foraDoQuadro(func() {
		Check(p.w, CheckAviso, truncarCelulas(texto, max(p.largura-4, 1)), "", "")
	})
}

// Escritor devolve um io.Writer para texto que não é das etapas (ex.: o log) e que vai
// para destino. No terminal, cada linha completa apaga o trecho redesenhável, sai em
// destino como veio e o trecho volta logo abaixo; assim o redesenho seguinte não apaga a
// linha nem deixa uma cópia velha da etapa atual na tela. O resto de uma linha sem "\n"
// espera a próxima escrita. Fora do terminal, o texto vai direto para destino.
func (p *Progresso) Escritor(destino io.Writer) io.Writer {
	return &escritorExterno{p: p, destino: destino}
}

type escritorExterno struct {
	p       *Progresso
	destino io.Writer
	resto   []byte
}

func (e *escritorExterno) Write(b []byte) (int, error) {
	if !e.p.tty {
		return e.destino.Write(b)
	}
	e.p.mu.Lock()
	defer e.p.mu.Unlock()
	e.resto = append(e.resto, b...)
	fim := bytes.LastIndexByte(e.resto, '\n')
	if fim < 0 {
		return len(b), nil
	}
	linhas := append([]byte(nil), e.resto[:fim+1]...)
	e.resto = append(e.resto[:0], e.resto[fim+1:]...)
	var err error
	e.p.foraDoQuadroTravado(func() { _, err = e.destino.Write(linhas) })
	return len(b), err
}

// Pausar tira o trecho redesenhável da tela e o mantém fora dela até retomar ser
// chamada, para um programa que fala direto com o terminal (ex.: o pedido de senha do
// sudo) não ter a sua linha apagada pelo redesenho. Fora do terminal não muda nada.
func (p *Progresso) Pausar() (retomar func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.apagarVivas()
	p.escreverFixas()
	p.pausas++
	var uma sync.Once
	return func() {
		uma.Do(func() {
			p.mu.Lock()
			defer p.mu.Unlock()
			p.pausas--
			p.desenhar(p.relogio())
		})
	}
}

// foraDoQuadro escreve, com escrever, entre as etapas já terminadas e o trecho
// redesenhável, que volta logo abaixo.
func (p *Progresso) foraDoQuadro(escrever func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.foraDoQuadroTravado(escrever)
}

// foraDoQuadroTravado é foraDoQuadro com o mutex já seguro por quem chama.
func (p *Progresso) foraDoQuadroTravado(escrever func()) {
	p.apagarVivas()
	p.escreverFixas()
	escrever()
	if p.tty && p.pausas == 0 {
		p.escreverVivas(p.relogio())
	}
}

// Animar redesenha a etapa atual a cada intervalo, para o tempo correr na tela, até a
// função devolvida ser chamada. Fora do terminal não faz nada.
func (p *Progresso) Animar(intervalo time.Duration) (parar func()) {
	if !p.tty || intervalo <= 0 {
		return func() {}
	}
	fim := make(chan struct{})
	pronto := make(chan struct{})
	go func() {
		defer close(pronto)
		t := time.NewTicker(intervalo)
		defer t.Stop()
		for {
			select {
			case <-fim:
				return
			case <-t.C:
				p.Tique()
			}
		}
	}()
	var uma sync.Once
	return func() {
		uma.Do(func() {
			close(fim)
			<-pronto
		})
	}
}

// Etapas devolve uma cópia das etapas, com a duração da atual até agora.
func (p *Progresso) Etapas() []Etapa {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.copia(p.relogio())
}

// Quadro devolve as etapas como Steps as escreveria agora, uma por linha.
func (p *Progresso) Quadro() string {
	var b strings.Builder
	Steps(&b, p.Etapas(), p.largura)
	return b.String()
}

// terminar fecha a etapa atual; quem chama segura o mutex.
func (p *Progresso) terminar(estado EstadoEtapa, nota string, agora time.Time) {
	e := &p.etapas[p.atual]
	e.Estado, e.Nota, e.Fracao = estado, nota, 0
	e.Duracao = agora.Sub(p.inicios[p.atual])
	p.atual = -1
}

func (p *Progresso) primeiraNaoFinal() int {
	for i, e := range p.etapas {
		if e.Estado != EtapaFeita && e.Estado != EtapaFalha && e.Estado != EtapaPulada {
			return i
		}
	}
	return len(p.etapas)
}

func (p *Progresso) copia(agora time.Time) []Etapa {
	c := make([]Etapa, len(p.etapas))
	copy(c, p.etapas)
	if p.atual >= 0 {
		c[p.atual].Duracao = agora.Sub(p.inicios[p.atual])
	}
	return c
}

// desenhar atualiza a saída depois de uma mudança; quem chama segura o mutex.
func (p *Progresso) desenhar(agora time.Time) {
	p.apagarVivas()
	p.escreverFixas()
	if p.tty && p.pausas == 0 {
		p.escreverVivas(agora)
	}
}

// apagarVivas sobe até o começo do trecho redesenhável e o limpa.
func (p *Progresso) apagarVivas() {
	if p.tty && p.vivas > 0 {
		fmt.Fprintf(p.w, "\x1b[%dA\r\x1b[J", p.vivas)
	}
	p.vivas = 0
}

// escreverFixas escreve de vez as etapas do começo que já terminaram.
func (p *Progresso) escreverFixas() {
	fim := p.primeiraNaoFinal()
	if fim > p.fixas {
		Steps(p.w, p.etapas[p.fixas:fim], p.largura)
		p.fixas = fim
	}
}

// escreverVivas escreve a etapa atual e as pendentes, que serão redesenhadas.
func (p *Progresso) escreverVivas(agora time.Time) {
	vivas := p.copia(agora)[p.fixas:]
	if len(vivas) == 0 || (p.falhou && p.atual < 0) {
		return
	}
	Steps(p.w, vivas, p.largura)
	p.vivas = len(vivas)
}
