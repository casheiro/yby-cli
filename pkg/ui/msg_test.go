package ui

import (
	"bytes"
	stderrors "errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	yerrors "github.com/casheiro/yby-cli/pkg/errors"
)

func msgDoctor() string {
	var b bytes.Buffer
	Title(&b, "doctor", "23/10 12:10")
	Section(&b, "ferramentas")
	Check(&b, CheckOk, "docker", "27.3", "")
	Check(&b, CheckAviso, "helm", "3.12", "recomendado 3.14 ou mais · yby setup atualiza")
	Check(&b, CheckErro, "kubectl", "ausente", "winget install Kubernetes.kubectl")
	Check(&b, CheckNeutro, "az", "ausente", "opcional")
	Summary(&b, 4, 1, 1, "o erro impede yby up")
	return b.String()
}

func msgErro() string {
	e := yerrors.Wrap(stderrors.New("o token do EKS expirou há 12 min"), yerrors.ErrCodeClusterOffline,
		"Não deu para falar com o cluster staging").
		WithHint("yby cloud refresh --provider aws")
	return RenderError(e)
}

func msgUp() string {
	var b bytes.Buffer
	Title(&b, "up", "23/10 12:10")
	Steps(&b, []Etapa{
		{Rotulo: "cluster yby-local", Nota: "criado", Estado: EtapaFeita, Duracao: 38 * time.Second},
		{Rotulo: "argo cd", Estado: EtapaAtual, Fracao: 0.64, Duracao: 25 * time.Second},
		{Rotulo: "root-app", Estado: EtapaPendente},
	}, 64)
	return b.String()
}

var msgCenarios = []struct {
	nome   string
	escuro bool
	perfil termenv.Profile
	ascii  bool
	semCor bool
}{
	{"escuro", true, termenv.TrueColor, false, false},
	{"claro", false, termenv.TrueColor, false, false},
	{"semcor", true, termenv.Ascii, true, true},
}

func TestMsgGolden(t *testing.T) {
	for _, c := range msgCenarios {
		t.Run(c.nome, func(t *testing.T) {
			usarTema(t, c.escuro, c.perfil, c.ascii)
			if c.semCor {
				Configurar(Opcoes{Tema: TemaEscuro, ASCII: true, SemCor: true})
			}
			conferirGolden(t, "msg_doctor_"+c.nome, msgDoctor())
			conferirGolden(t, "msg_erro_"+c.nome, msgErro())
			conferirGolden(t, "msg_up_"+c.nome, msgUp())
		})
	}
}

func TestMsgAlinhamentoEmCelulas(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	var b bytes.Buffer
	Check(&b, CheckOk, "docker", "27.3", "nota")
	Check(&b, CheckErro, "kubectl", "ausente", "nota")
	linhas := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	pos := func(l string) int { return lipgloss.Width(l[:strings.Index(l, "nota")]) }
	if pos(linhas[0]) != pos(linhas[1]) {
		t.Errorf("colunas desalinhadas: %q / %q", linhas[0], linhas[1])
	}
}

func TestMsgTituloLargura(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	var b bytes.Buffer
	Title(&b, "viz", "x")
	if w := lipgloss.Width(strings.TrimRight(b.String(), "\n")); w != msgLargura {
		t.Errorf("largura = %d, esperado %d", w, msgLargura)
	}
	b.Reset()
	Title(&b, "instrumento muito longo que estoura a largura da linha de título", "direita longa também")
	if !strings.Contains(b.String(), "  ") {
		t.Error("esperava ao menos 2 espaços entre as pontas")
	}
}

func TestMsgCheckSemExtras(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	var b bytes.Buffer
	Check(&b, CheckOk, "git", "", "")
	Check(&b, CheckNeutro, "az", "", "opcional")
	if got := b.String(); got != "  ✓ git\n  ○ az"+strings.Repeat(" ", 20)+"opcional\n" {
		t.Errorf("saída inesperada: %q", got)
	}
}

func TestMsgSummaryPlural(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	var b bytes.Buffer
	Summary(&b, 1, 2, 3, "")
	if !strings.Contains(b.String(), "1 ok · 2 avisos · 3 erros") {
		t.Errorf("saída inesperada: %q", b.String())
	}
}

func TestRenderErrorVariacoes(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	if RenderError(nil) != "" {
		t.Error("nil deveria renderizar vazio")
	}
	// sem hint próprio: usa o do registry, e detalhes traz o contexto
	e := yerrors.New(yerrors.ErrCodeClusterOffline, "cluster\noffline").WithContext("ctx", "staging").WithContext("a", 1)
	out := RenderError(e)
	if !strings.Contains(out, "faça") || !strings.Contains(out, "a=1 · ctx=staging") || strings.Contains(out, "causa") {
		t.Errorf("saída inesperada:\n%s", out)
	}
	// código sem hint no registry: o "faça" cai na dica genérica e não repete em detalhes
	out = RenderError(yerrors.New("ERR_INEXISTENTE", "falhou"))
	if strings.Count(out, "--log-level=debug") > 1 {
		t.Errorf("dica genérica repetida:\n%s", out)
	}
}

func TestSteps(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	for _, larg := range []int{0, 30, 64} {
		var b bytes.Buffer
		Steps(&b, []Etapa{
			{Rotulo: "cluster yby-local", Nota: "criado", Estado: EtapaFeita, Duracao: 38 * time.Second},
			{Rotulo: "argo cd", Estado: EtapaAtual, Fracao: 0.64, Duracao: 25 * time.Second},
			{Rotulo: "root-app", Estado: EtapaPendente},
		}, larg)
		linhas := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
		if len(linhas) != 3 {
			t.Fatalf("larg %d: %d linhas", larg, len(linhas))
		}
		esperado := larg
		if esperado <= 0 {
			esperado = msgLargura
		}
		for _, l := range linhas {
			if !strings.HasPrefix(l, "  ") || lipgloss.Width(l) > esperado {
				t.Errorf("larg %d: linha inválida %q", larg, l)
			}
		}
		if !strings.Contains(linhas[1], "64%") {
			t.Errorf("sem cota: %q", linhas[1])
		}
	}
}
