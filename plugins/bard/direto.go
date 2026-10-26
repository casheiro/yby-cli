package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/config"
	yerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/logger"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/bard/tools"
	"github.com/casheiro/yby-cli/plugins/bard/tui"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Fora da TUI (a pergunta do -p e as linhas do stdin no batch), a resposta vai sozinha
// para o stdout, em texto e sem códigos de controle, para outro programa ler. As etapas
// (ui.Progresso, como ui.Steps) vão para o stderr só quando ele é um terminal, e os erros
// saem por ui.WriteError. Nada ali pede o que digitar.

// obterProvider escolhe o provider de IA (trocado nos testes).
var obterProvider = func(ctx context.Context) ai.Provider { return ai.GetProvider(ctx, "auto") }

// intervaloEtapas é de quanto em quanto a etapa atual é redesenhada, para o tempo correr.
const intervaloEtapas = 250 * time.Millisecond

// separadorLote fica entre as respostas do batch.
const separadorLote = "\n---\n\n"

// direto responde perguntas fora da TUI pelo mesmo fluxo dela (contexto, intenção,
// ferramenta e resposta).
type direto struct {
	fluxo *tui.Fluxo
	saida io.Writer // a resposta, e só ela
	erros io.Writer // as etapas e os erros
	// etapas liga as etapas no stderr (ele é um terminal); continua escreve a resposta à
	// medida que chega (o stdout é um terminal), em vez de inteira no fim.
	etapas, continua bool
	rodadas          int
}

// enriquecerCluster lê o cluster da sessão; os testes trocam por um falso.
var enriquecerCluster = EnrichContext

// novoDireto prepara o provider, o system prompt e a busca no Synapstor e liga o
// direto ao stdout e ao stderr do processo. Quem chama já rodou prepararStderr.
func novoDireto(ctx context.Context, ctxData map[string]interface{}, opcoes opcoesBard) (*direto, error) {
	tools.LoadExternalTools()
	provider := obterProvider(ctx)
	if provider == nil {
		return nil, semProvider()
	}
	bardCfg := loadBardConfig()
	// O cluster e o namespace da sessão (--context, -n ou o do kubeconfig) vão ao
	// classificador, como na TUI.
	clusterCtx := enriquecerCluster(ctx, opcoes.contexto, opcoes.namespace)
	cfg := tui.Config{
		SystemPrompt:     buildSystemPrompt(ctxData, bardCfg, nil, clusterCtx),
		TopK:             bardCfg.TopK,
		LimiarRelevancia: bardCfg.RelevanceThreshold,
	}
	aplicarCluster(&cfg, clusterCtx, opcoes)
	fluxo := tui.NovoFluxo(provider, abrirMemoria(ctx), cfg)
	return &direto{
		fluxo:    fluxo,
		saida:    os.Stdout,
		erros:    os.Stderr,
		etapas:   ui.Largura(os.Stderr) > 0,
		continua: ui.Largura(os.Stdout) > 0,
	}, nil
}

// prepararStderr roda prepararErros no stderr do processo. Fora da TUI, vem antes de
// tudo o que pode sair ali: o log da detecção do provider e os erros.
func prepararStderr() {
	prepararErros(os.Stderr, ui.Largura(os.Stderr) > 0)
}

// prepararErros ajusta ao stderr, onde saem as etapas e os erros, o perfil de cor e o
// fundo que o lipgloss tiraria do stdout: com o stdout num pipe as etapas ficariam sem
// cor, e com o stderr num pipe os erros levariam códigos de cor. Liga também o log no
// nível do config (YBY_LOG_LEVEL > log.level), que as etapas desviam enquanto aparecem.
func prepararErros(erros *os.File, terminal bool) {
	cfg := config.Get()
	logger.InitGlobal(logger.Config{Level: cfg.Log.Level, Format: cfg.Log.Format})
	if !terminal {
		lipgloss.SetColorProfile(termenv.Ascii)
		return
	}
	saida := termenv.NewOutput(erros)
	if ui.Atual().Tema == ui.TemaAuto {
		lipgloss.SetHasDarkBackground(saida.HasDarkBackground())
	}
	// Sem cor, ui.Configurar já deixou o perfil ANSI (negrito sem cores).
	if !ui.Atual().SemCor {
		lipgloss.SetColorProfile(saida.EnvColorProfile())
	}
}

// abrirMemoria abre o índice do Synapstor do diretório atual para a busca. Sem o índice,
// fica sem memória e nada é criado no diretório.
func abrirMemoria(ctx context.Context) *ai.VectorStore {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}
	caminho := filepath.Join(cwd, ".synapstor", ".index")
	if st, err := os.Stat(caminho); err != nil || !st.IsDir() {
		return nil
	}
	vs, err := ai.NewVectorStore(ctx, caminho, ai.GetEmbeddingProvider(ctx))
	if err != nil {
		slog.Debug("memoria semantica indisponivel", "erro", err)
		return nil
	}
	return vs
}

// perguntar responde uma pergunta (o -p).
func (d *direto) perguntar(ctx context.Context, pergunta string) error {
	d.rodadas++
	return d.erroDaRodada(d.responder(ctx, d.rodadas, pergunta))
}

// lote responde uma pergunta por linha de entrada, com as respostas separadas por "---".
// A falha de uma pergunta sai no stderr e o lote segue; no fim, o código de saída é 1.
func (d *direto) lote(ctx context.Context, entrada io.Reader) error {
	scanner := bufio.NewScanner(entrada)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	perguntas, falhas := 0, 0
	for scanner.Scan() {
		if ctx.Err() != nil {
			return saidaSilenciosa{codigo: codigoCtrlC}
		}
		pergunta := strings.TrimSpace(scanner.Text())
		if pergunta == "" {
			continue
		}
		if perguntas > 0 {
			fmt.Fprint(d.saida, separadorLote)
		}
		perguntas++
		d.rodadas++
		err := d.erroDaRodada(d.responder(ctx, d.rodadas, pergunta))
		var silenciosa saidaSilenciosa
		switch {
		case err == nil:
		case errors.As(err, &silenciosa):
			return err
		default:
			falhas++
			sairComErro(d.erros, comPergunta(err, pergunta))
		}
	}
	if err := scanner.Err(); err != nil {
		return yerrors.Wrap(err, yerrors.ErrCodeIO, "falha ao ler as perguntas do stdin")
	}
	if ctx.Err() != nil {
		return saidaSilenciosa{codigo: codigoCtrlC}
	}
	if perguntas == 0 {
		return yerrors.New(yerrors.ErrCodeValidation, "nenhuma pergunta no stdin").
			WithHint(`mande uma pergunta por linha ou use yby bard -p "pergunta"`)
	}
	if falhas > 0 {
		return saidaSilenciosa{codigo: 1}
	}
	return nil
}

// comPergunta põe a pergunta no contexto do erro, para o lote dizer qual falhou.
func comPergunta(err error, pergunta string) error {
	var ye *yerrors.YbyError
	if errors.As(err, &ye) {
		return ye.WithContext("pergunta", pergunta)
	}
	return err
}

// erroDaRodada traduz o fim de uma rodada: interrompida sai calada com 130 (ctrl+c), e a
// falha do provider vira um YbyError com o nome dele nos detalhes. A dica do log vale
// para o plugin: o --log-level do core não chega a ele, e depois do -p vira pergunta.
func (d *direto) erroDaRodada(fim tui.FimMsg) error {
	switch {
	case fim.Interrompida:
		return saidaSilenciosa{codigo: codigoCtrlC}
	case fim.Err != nil:
		ye := yerrors.Wrap(fim.Err, yerrors.ErrCodeExec, "a IA não respondeu").
			WithHint("confira o provider de IA com yby doctor; para ver o log, rode com YBY_LOG_LEVEL=debug")
		if nome := rotuloProvider(d.fluxo.Provider); nome != "" {
			ye = ye.WithContext("provider", nome)
		}
		return ye
	}
	return nil
}

// responder roda uma rodada do fluxo: as etapas vão para o stderr (num terminal) e a
// resposta para o stdout, à medida que chega ou inteira no fim.
func (d *direto) responder(ctx context.Context, rodada int, pergunta string) tui.FimMsg {
	resposta := &porLinha{destino: d.saida}
	var prog *ui.Progresso
	if d.etapas {
		prog = ui.NovoProgressoCom(d.erros, ui.OpcoesProgresso{Terminal: func(io.Writer) bool { return true }})
		resposta.destino = prog.Escritor(d.saida)
		restaurar := logger.Desviar(prog.Escritor(d.erros))
		parar := prog.Animar(intervaloEtapas)
		defer func() {
			parar()
			restaurar()
		}()
	}
	var (
		fim       tui.FimMsg
		atual     string // rótulo da etapa em andamento no Progresso
		tentativa int    // última tentativa da resposta anunciada
	)
	for ev := range d.fluxo.Iniciar(ctx, rodada, pergunta) {
		switch m := ev.(type) {
		case tui.EtapaMsg:
			if m.Etapa == tui.EtapaResposta {
				switch {
				case m.Estado != ui.EtapaAtual, m.Nota == tui.NotaNovaTentativa:
					// A tentativa acabou; depois de uma falha, a próxima pode nem começar.
					resposta.fechar()
				case m.Tentativa > tentativa:
					tentativa = m.Tentativa
					if m.Tentativa > 1 && prog != nil {
						if resposta.escreveu {
							// A resposta recomeçou depois de já ter aparecido em parte.
							prog.Avisar("a resposta recomeçou depois de uma falha do provider")
						} else {
							prog.Detalhe(m.Nota)
						}
					}
					resposta.escreveu = false
				}
			}
			if prog == nil {
				slog.Debug("bard.etapa", "etapa", m.Rotulo, "estado", m.Estado, "nota", m.Nota)
				continue
			}
			atual = mostrarEtapa(prog, atual, m)
		case tui.TrechoMsg:
			if d.continua {
				_, _ = resposta.Write([]byte(m.Texto))
			}
		case tui.FimMsg:
			fim = m
		}
	}
	if !d.continua && fim.Err == nil && !fim.Interrompida && fim.Resposta != "" {
		texto := fim.Resposta
		if !strings.HasSuffix(texto, "\n") {
			texto += "\n"
		}
		_, _ = io.WriteString(d.saida, texto)
	}
	return fim
}

// mostrarEtapa leva uma etapa do fluxo ao Progresso e devolve o rótulo da etapa em
// andamento. Uma nova tentativa da resposta continua na mesma linha.
func mostrarEtapa(prog *ui.Progresso, atual string, m tui.EtapaMsg) string {
	switch m.Estado {
	case ui.EtapaAtual:
		if m.Rotulo != atual {
			prog.Iniciar(m.Rotulo)
		}
		return m.Rotulo
	case ui.EtapaFeita:
		prog.Concluir(m.Nota)
	case ui.EtapaFalha:
		prog.Falhar(m.Nota)
	}
	return ""
}

// porLinha leva a resposta ao destino só em linhas inteiras: com as etapas na tela, o
// trecho redesenhado sai do caminho a cada linha. fechar escreve o resto.
type porLinha struct {
	destino  io.Writer
	resto    []byte
	escreveu bool // algo da tentativa atual já foi escrito (ou espera no resto)
}

func (l *porLinha) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	l.escreveu = true
	l.resto = append(l.resto, p...)
	if i := bytes.LastIndexByte(l.resto, '\n'); i >= 0 {
		if _, err := l.destino.Write(l.resto[:i+1]); err != nil {
			return 0, err
		}
		l.resto = append(l.resto[:0], l.resto[i+1:]...)
	}
	return len(p), nil
}

// fechar termina a última linha, se ficou aberta.
func (l *porLinha) fechar() {
	if len(l.resto) == 0 {
		return
	}
	_, _ = l.destino.Write(append(l.resto, '\n'))
	l.resto = l.resto[:0]
}
