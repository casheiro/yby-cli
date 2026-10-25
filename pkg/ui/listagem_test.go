package ui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/muesli/termenv"
)

func listagemAmbientes(w *bytes.Buffer) {
	Listagem(w, []Coluna{
		{Titulo: ""},
		{Titulo: "ambiente"},
		{Titulo: "tipo", Prioridade: 2},
		{Titulo: "descrição", Prioridade: 1},
	}, []Linha{
		{Celulas: []string{S().Ok.Render(G().Ativo), "local", "local", "cluster k3d na máquina"}},
		{Celulas: []string{"", "prod", "remote", "produção"}},
		{Celulas: []string{"", "qa", "remote"}, Concluida: true},
	})
}

// listagemComStatus tem uma coluna de Status, célula que já chega com cor.
func listagemComStatus(w *bytes.Buffer) {
	Listagem(w, []Coluna{{Titulo: "ação"}, {Titulo: "status"}}, []Linha{
		{Celulas: []string{"refresh", Status(StatusRodando, "ok")}},
		{Celulas: []string{"connect", Status(StatusFalha, "token expirado")}},
	})
}

func msgListagem() string {
	var b bytes.Buffer
	Title(&b, "env", "ativo local")
	b.WriteString("\n")
	listagemAmbientes(&b)
	Section(&b, "auditoria")
	listagemComStatus(&b)
	Section(&b, "ambiente")
	Campos(&b, Campo{Rotulo: "nome", Valor: "local"}, Campo{Rotulo: "values", Valor: "config/values-local.yaml"}, Campo{Rotulo: "url"})
	Section(&b, "próximos passos")
	Passos(&b, "edite o arquivo gerado", "aplique com kubectl apply -f bootstrap/secret-store-vault.yaml")
	return b.String()
}

func TestListagemGolden(t *testing.T) {
	for _, c := range msgCenarios {
		t.Run(c.nome, func(t *testing.T) {
			usarTema(t, c.escuro, c.perfil, c.ascii)
			if c.semCor {
				Configurar(Opcoes{Tema: TemaEscuro, ASCII: true, SemCor: true})
			}
			conferirGolden(t, "listagem_"+c.nome, msgListagem())
		})
	}
}

func TestListagemColunasESemEspacoADireita(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	var b bytes.Buffer
	listagemAmbientes(&b)
	esperado := "     AMBIENTE  TIPO    DESCRIÇÃO\n" +
		"  ◆  local     local   cluster k3d na máquina\n" +
		"     prod      remote  produção\n" +
		"     qa        remote\n"
	if b.String() != esperado {
		t.Errorf("saída inesperada:\n%q\nesperado:\n%q", b.String(), esperado)
	}
}

func TestListagemNaoRetintaCelulaComCor(t *testing.T) {
	usarTema(t, true, termenv.TrueColor, false)
	var b bytes.Buffer
	listagemComStatus(&b)
	status := Status(StatusFalha, "token expirado")
	if !strings.Contains(b.String(), status) {
		t.Errorf("a célula com cor deveria sair como veio:\n%q\nesperado conter:\n%q", b.String(), status)
	}
	if strings.Contains(b.String(), S().Tinta.Render(status)) {
		t.Errorf("a célula com cor não deveria receber a tinta de fora:\n%q", b.String())
	}
	if !strings.Contains(b.String(), S().Tinta.Render("connect")) {
		t.Errorf("a célula sem cor deveria receber a tinta:\n%q", b.String())
	}
}

func TestListagemMaxDireitaEMinimo(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	var b bytes.Buffer
	Listagem(&b, []Coluna{
		{Titulo: "nome", Min: 6},
		{Titulo: "total", Direita: true},
		{Titulo: "identidade", Max: 12},
	}, []Linha{
		{Celulas: []string{"aws", "3", "arn:aws:iam::123456789012:user/ana"}},
		{Celulas: []string{"gcp", "12", "ana"}},
	})
	esperado := "  NOME    TOTAL  IDENTIDADE\n" +
		"  aws         3  arn:aws:iam…\n" +
		"  gcp        12  ana\n"
	if b.String() != esperado {
		t.Errorf("saída inesperada:\n%q\nesperado:\n%q", b.String(), esperado)
	}
}

func TestListagemSemColunasELinhaVazia(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	var b bytes.Buffer
	Listagem(&b, nil, []Linha{{Celulas: []string{"x"}}})
	if b.Len() != 0 {
		t.Errorf("sem colunas não deveria escrever nada: %q", b.String())
	}
	Listagem(&b, []Coluna{{Titulo: "a"}, {Titulo: "b"}}, []Linha{{}})
	if got := b.String(); got != "  A  B\n\n" {
		t.Errorf("saída inesperada: %q", got)
	}
}

func TestListagemNoTerminalEstreito(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	fixarLargura(t, 30)
	var b bytes.Buffer
	listagemAmbientes(&b)
	for _, l := range strings.Split(strings.TrimRight(b.String(), "\n"), "\n") {
		if w := len([]rune(l)); w > 30 {
			t.Errorf("linha com %d células passa de 30: %q", w, l)
		}
		if !strings.HasPrefix(l, "  ") {
			t.Errorf("linha sem o recuo de 2 células: %q", l)
		}
	}
	if !strings.Contains(b.String(), "cluster…") {
		t.Errorf("esperava a descrição truncada:\n%s", b.String())
	}
}

func TestCamposEPassosNoTerminal(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	fixarLargura(t, 34)
	var b bytes.Buffer
	Campos(&b, Campo{Rotulo: "identidade", Valor: "arn:aws:iam::123 usuário ana da conta de produção"}, Campo{Rotulo: "vazio"})
	Passos(&b, "aplique com kubectl apply -f bootstrap/secret-store-vault.yaml")
	esperado := "  identidade  arn:aws:iam::123\n" +
		"              usuário ana da conta\n" +
		"              de produção\n" +
		"  1. aplique com kubectl apply -f\n" +
		"     bootstrap/secret-store-vault.yaml\n"
	if b.String() != esperado {
		t.Errorf("saída inesperada:\n%q\nesperado:\n%q", b.String(), esperado)
	}
}

func TestPassosAlinhaComDezOuMais(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	var b bytes.Buffer
	passos := make([]string, 10)
	for i := range passos {
		passos[i] = "passo"
	}
	Passos(&b, passos...)
	linhas := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	if linhas[0] != "  1.  passo" || linhas[9] != "  10. passo" {
		t.Errorf("numeração desalinhada: %q / %q", linhas[0], linhas[9])
	}
}
