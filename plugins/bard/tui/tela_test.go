package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/pkg/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/muesli/termenv"
)

// Telas da prancha com dados de exemplo, em tamanho fixo. Para regravar:
// go test -run TestTela ./plugins/bard/tui/ -update
const (
	larguraTela = 110
	alturaTela  = 29
)

// modo é uma variante visual dos goldens.
type modo struct {
	nome   string
	tema   ui.Tema
	ascii  bool
	perfil termenv.Profile
}

// usarModo configura tema, glifos e perfil de cor e restaura tudo no fim do teste.
func usarModo(t *testing.T, md modo) {
	t.Helper()
	escuroAnterior := lipgloss.HasDarkBackground()
	perfilAnterior := lipgloss.ColorProfile()
	opcoesAnteriores := ui.Atual()
	t.Cleanup(func() {
		ui.Configurar(opcoesAnteriores)
		lipgloss.SetHasDarkBackground(escuroAnterior)
		lipgloss.SetColorProfile(perfilAnterior)
	})
	ui.Configurar(ui.Opcoes{Tema: md.tema, ASCII: md.ascii})
	lipgloss.SetColorProfile(md.perfil)
}

// relogio devolve as horas dadas, uma a cada chamada; a última se repete.
func relogio(horas ...string) func() time.Time {
	i := 0
	return func() time.Time {
		h, _ := time.ParseInLocation("2006-01-02 15:04", "2026-10-24 "+horas[min(i, len(horas)-1)], time.Local)
		i++
		return h
	}
}

// telaDeTeste cria o modelo do Bard no tamanho fixo, sem provider de verdade.
func telaDeTeste(t *testing.T, cfg Config, horas ...string) Model {
	t.Helper()
	isolarHome(t)
	m := New(&provedorFalso{}, nil, cfg)
	m.agora = relogio(horas...)
	// Sem TTY o spinner fica no quadro fixo da carta, e as telas não dependem do terminal.
	m.novoSpinner = func(r string) ui.Spinner { return ui.NovoSpinnerCom(r, func() bool { return false }) }
	m.novoEstado = func(r string) ui.Estado { return ui.NovoEstadoCarregandoCom(r, func() bool { return false }) }
	atualizado, _ := m.Update(tea.WindowSizeMsg{Width: larguraTela, Height: alturaTela})
	return atualizado.(Model)
}

// conversar registra a pergunta e passa a rodada pelos mesmos eventos do fluxo:
// as ferramentas como etapas e a resposta no FimMsg.
func conversar(t *testing.T, m Model, pergunta, resposta string, duracao time.Duration, ferramentas ...EtapaMsg) Model {
	t.Helper()
	m.messages = append(m.messages, chatMessage{role: "user", content: pergunta, hora: m.agora()})
	m.state = stateStreaming
	m.rodada++
	for _, e := range ferramentas {
		e.Rodada = m.rodada
		atualizado, _ := m.Update(e)
		m = atualizado.(Model)
	}
	atualizado, _ := m.Update(FimMsg{Rodada: m.rodada, Resposta: resposta, Duracao: duracao})
	return atualizado.(Model)
}

// conferirTela confere as dimensões da tela e compara com o golden.
func conferirTela(t *testing.T, m Model) string {
	t.Helper()
	tela := m.View()
	linhas := strings.Split(tela, "\n")
	if len(linhas) != alturaTela {
		t.Errorf("altura: %d linhas, esperado %d", len(linhas), alturaTela)
	}
	for i, l := range linhas {
		if w := lipgloss.Width(l); w != larguraTela {
			t.Errorf("linha %d com %d células: %q", i+1, w, l)
		}
	}
	golden.RequireEqual(t, []byte(tela))
	return tela
}

const respostaStorage = "O staging tem 6 volumes. Quatro estão em uso e dois sobraram de apps removidos.\n\n" +
	"| Volume | Namespace | Tamanho | Status |\n" +
	"|---|---|---|---|\n" +
	"| data-postgres-0 | payments | 20Gi | Bound |\n" +
	"| redis-data-0 | payments | 5Gi | Bound |\n" +
	"| minio-0 | storage | 50Gi | Bound |\n" +
	"| grafana | monitoring | 10Gi | Bound |\n" +
	"| pvc-3f1a9c2e | sem dono | 8Gi | Released |\n" +
	"| pvc-91cd04b7 | sem dono | 8Gi | Released |\n\n" +
	"Os dois volumes **Released** ainda ocupam 16Gi no nó. Se não precisar dos dados:\n\n" +
	"```bash\nkubectl delete pv pvc-3f1a9c2e pvc-91cd04b7\n```\n"

// TestTelaConversa reproduz a prancha "conversa": a ferramenta antes da resposta,
// tabela e comando na largura do terminal e o provider no cabeçalho. Como na prancha,
// a sessão tem só esta troca ("2 mensagens"), que cabe na janela: sem nada acima, não
// há o aviso das mensagens anteriores (ver TestTelaTrabalhando) nem pgup na legenda.
func TestTelaConversa(t *testing.T) {
	for _, md := range []modo{
		{"escuro", ui.TemaEscuro, false, termenv.TrueColor},
		{"ascii", ui.TemaEscuro, true, termenv.Ascii},
	} {
		t.Run(md.nome, func(t *testing.T) {
			usarModo(t, md)
			m := telaDeTeste(t, Config{
				Cluster: "staging", Namespace: "default", AIModel: "claude-cli", SessionID: "20261024-114500",
			}, "11:51", "11:52")
			saida := strings.TrimSuffix(strings.Repeat("NAMESPACE NAME STATUS VOLUME\n", 14), "\n")
			m = conversar(t, m, "qual a situação do meu storage?", respostaStorage, 21*time.Second,
				EtapaMsg{Etapa: EtapaFerramenta, Rotulo: "kubectl get", Nota: "pvc,pv -A", Estado: ui.EtapaAtual},
				EtapaMsg{Etapa: EtapaFerramenta, Rotulo: "kubectl get", Nota: "pvc,pv -A", Estado: ui.EtapaFeita,
					Duracao: 4 * time.Second, Saida: saida + "\n"})

			tela := conferirTela(t, m)
			for _, trecho := range []string{"BARD · 11:52 · 1 ferramenta · 21 s", "kubectl get pvc,pv -A · 14 linhas",
				"COMANDO · ctrl+y copia", "sessão de hoje · 2 mensagens", "Pergunte sobre o cluster, ou / para comandos"} {
				if !strings.Contains(ansi.Strip(tela), trecho) {
					t.Errorf("a tela deveria ter %q", trecho)
				}
			}
			if limpa := ansi.Strip(tela); strings.Contains(limpa, "anteriores") || strings.Contains(limpa, "pgup") ||
				!strings.HasPrefix(strings.Split(limpa, "\n")[2], "VOCÊ · 11:51") {
				t.Errorf("a conversa cabe na janela e começa na pergunta:\n%s", limpa)
			}
		})
	}
}

// TestTelaKraft reproduz a prancha "tema kraft" no tema claro: a mesma moldura com
// os tokens do kraft e o quadro do /contexto, digitado e executado pelo campo.
func TestTelaKraft(t *testing.T) {
	for _, md := range []modo{
		{"claro", ui.TemaClaro, false, termenv.TrueColor},
		{"ascii", ui.TemaClaro, true, termenv.Ascii},
	} {
		t.Run(md.nome, func(t *testing.T) {
			usarModo(t, md)
			m := telaDeTeste(t, Config{
				Cluster: "staging", Namespace: "default", AIModel: "claude-cli", SessionID: "20261024-120000",
				Contexto: contextoDeExemplo, Ferramentas: ferramentasDeExemplo, Historico: HistoricoPrompt{Mensagens: 20, Sessoes: 3},
			}, "12:15", "12:16")
			m, _ = teclarModelo(t, m, append(teclasDe("/contexto"), teclaEnter)...)
			m = conversar(t, m, "quais ferramentas externas estão com problema?",
				"**~/.yby/tools/backup.yaml** não carregou: falta o campo `command`.\n"+
					"As outras 2 ferramentas externas estão disponíveis.", 2400*time.Millisecond)

			tela := conferirTela(t, m)
			if strings.Contains(tela, "pgup") {
				t.Error("a conversa cabe na tela: pgup não deveria aparecer")
			}
			limpa := ansi.Strip(tela)
			if !strings.Contains(limpa, "BARD · 12:16 · 2,4 s") || !strings.Contains(limpa, "sessão de hoje · 4 mensagens") {
				t.Errorf("cabeçalho da resposta ou resumo da sessão:\n%s", tela)
			}
			for _, trecho := range []string{"VOCÊ · 12:15", "CONTEXTO DESTA SESSÃO", "staging · eks · us-east-1",
				"12 UKIs indexadas · atualizado há 2 dias", "1 externa inválida", "últimas 20 mensagens de 3 sessões"} {
				if !strings.Contains(limpa, trecho) {
					t.Errorf("a tela deveria ter %q:\n%s", trecho, limpa)
				}
			}
		})
	}
}
