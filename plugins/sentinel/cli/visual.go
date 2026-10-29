//go:build k8s

package main

import (
	"io"
	"sync"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/config"
	"github.com/casheiro/yby-cli/v5/pkg/logger"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
)

// configurarModoVisual aplica no processo do plugin o modo visual do core: YBY_THEME,
// YBY_ASCII e NO_COLOR > ui.theme e ui.ascii do ~/.yby/config.yaml > padrão. O Sentinel
// roda em outro processo, então o ui.Configurar do cmd/root.go não chega até aqui.
func configurarModoVisual(getenv func(string) string) {
	ui.Configurar(ui.ResolverOpcoes(ui.Opcoes{}, false, getenv, config.Get().UI))
}

// configurarLog liga o slog do plugin ao nível e ao formato do config (YBY_LOG_LEVEL e
// YBY_LOG_FORMAT > log.level e log.format do ~/.yby/config.yaml), como o core faz no
// processo dele; o --log-level do core não chega até aqui. Sem isto o log ficaria no
// handler padrão do slog, que o logger.Desviar de animar não alcança, e uma linha de log
// cairia no meio das etapas redesenhadas.
func configurarLog() {
	cfg := config.Get()
	logger.InitGlobal(logger.Config{Level: cfg.Log.Level, Format: cfg.Log.Format})
}

// Peças substituíveis nos testes: o relógio das etapas e do nome do relatório, a
// detecção de terminal e a largura das saídas.
var (
	relogio    = time.Now
	terminalEm = func(w io.Writer) bool { return ui.Largura(w) > 0 }
	// larguraTerminal é a largura do terminal ligado a w (0 fora dele), a mesma que o
	// ui.Check usa para quebrar a nota; as linhas do scan que quebram por conta própria
	// (checkNaColuna, checkComCaminho) medem com ela.
	larguraTerminal = ui.Largura
)

// larguraPadrao é a largura das saídas do Sentinel, a mesma das saídas de comando do core.
const larguraPadrao = 64

// larguraSaida devolve a largura das linhas em w: a padrão, ou a do terminal quando ele é
// mais estreito.
var larguraSaida = func(w io.Writer) int {
	if l := ui.Largura(w); l > 0 && l < larguraPadrao {
		return l
	}
	return larguraPadrao
}

// intervaloAnimacao é de quanto em quanto tempo a etapa atual é redesenhada no terminal.
const intervaloAnimacao = 200 * time.Millisecond

// novoProgresso cria as etapas de um comando em w, com o relógio e a detecção de terminal
// do pacote.
func novoProgresso(w io.Writer) *ui.Progresso {
	return ui.NovoProgressoCom(w, ui.OpcoesProgresso{Terminal: terminalEm, Relogio: relogio, Largura: larguraSaida(w)})
}

// animar faz o tempo da etapa atual correr na tela e, enquanto isso, manda o log para o
// lugar certo entre as etapas; fora do terminal não faz nada. A função devolvida para os
// dois e pode ser chamada mais de uma vez.
func animar(prog *ui.Progresso, saidaDoLog io.Writer) (parar func()) {
	pararAnimacao := prog.Animar(intervaloAnimacao)
	restaurar := func() {}
	if prog.Terminal() {
		restaurar = logger.Desviar(prog.Escritor(saidaDoLog))
	}
	var uma sync.Once
	return func() {
		uma.Do(func() {
			pararAnimacao()
			restaurar()
		})
	}
}
