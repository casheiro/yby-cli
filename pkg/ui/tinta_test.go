package ui

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/muesli/termenv"

	yerrors "github.com/casheiro/yby-cli/v5/pkg/errors"
)

// textoSemCor devolve os trechos de texto visível (exceto espaços) que não estão sob uma cor de
// frente SGR. Sequências que não são SGR são ignoradas.
func textoSemCor(s string) []string {
	var achados []string
	temCor := false
	var atual strings.Builder
	descarregar := func() {
		if atual.Len() > 0 {
			achados = append(achados, atual.String())
			atual.Reset()
		}
	}
	runas := []rune(s)
	for i := 0; i < len(runas); i++ {
		r := runas[i]
		if r == 0x1b && i+1 < len(runas) && runas[i+1] == '[' {
			j := i + 2
			for j < len(runas) && (runas[j] < 0x40 || runas[j] > 0x7e) {
				j++
			}
			if j < len(runas) && runas[j] == 'm' {
				descarregar()
				temCor = aplicarSGR(string(runas[i+2:j]), temCor)
			}
			i = j
			continue
		}
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			descarregar()
			continue
		}
		if !temCor {
			atual.WriteRune(r)
		}
	}
	descarregar()
	return achados
}

// aplicarSGR atualiza o estado "há cor de frente" conforme os parâmetros de uma sequência SGR.
func aplicarSGR(params string, temCor bool) bool {
	if params == "" {
		return false
	}
	partes := strings.Split(params, ";")
	for i := 0; i < len(partes); i++ {
		n, err := strconv.Atoi(partes[i])
		if err != nil {
			continue
		}
		switch {
		case n == 0 || n == 39:
			temCor = false
		case n == 38:
			temCor = true
			if i+1 < len(partes) {
				if partes[i+1] == "5" {
					i += 2
				} else if partes[i+1] == "2" {
					i += 4
				}
			}
		case n == 48:
			if i+1 < len(partes) {
				if partes[i+1] == "5" {
					i += 2
				} else if partes[i+1] == "2" {
					i += 4
				}
			}
		case (n >= 30 && n <= 37) || (n >= 90 && n <= 97):
			temCor = true
		}
	}
	return temCor
}

func amostrasDeComponentes() map[string]string {
	var b bytes.Buffer
	amostras := map[string]string{}
	pegar := func(nome string) {
		amostras[nome] = b.String()
		b.Reset()
	}

	Title(&b, "doctor", "23/10 12:10")
	Section(&b, "ferramentas")
	Check(&b, CheckOk, "docker", "27.3", "")
	Check(&b, CheckAviso, "helm", "3.12", "recomendado 3.14")
	Check(&b, CheckErro, "kubectl", "ausente", "winget install")
	Check(&b, CheckNeutro, "az", "ausente", "opcional")
	Summary(&b, 4, 1, 1, "o erro impede yby up")
	pegar("saida-doctor")

	Title(&b, "up", "23/10 12:10")
	Steps(&b, []Etapa{
		{Rotulo: "cluster", Nota: "criado", Estado: EtapaFeita, Duracao: 38 * time.Second},
		{Rotulo: "argo cd", Estado: EtapaAtual, Fracao: 0.64, Duracao: 25 * time.Second},
		{Rotulo: "root-app", Estado: EtapaPendente},
		{Rotulo: "relatório", Estado: EtapaFalha, Duracao: 2 * time.Second},
	}, 64)
	pegar("saida-up")

	erro := yerrors.Wrap(errors.New("token expirou"), yerrors.ErrCodeClusterOffline, "Sem acesso ao cluster").
		WithHint("yby cloud refresh")
	amostras["erro"] = RenderError(erro)

	colunas := []Coluna{
		{Titulo: "Nome", Min: 12, Max: 28}, {Titulo: "Status", Min: 16, Prioridade: 1},
		{Titulo: "Prontos", Min: 7, Prioridade: 2, Direita: true},
	}
	linhas := []Linha{
		{Celulas: []string{"api-7f9c-x2k", Status(StatusRodando, "Running"), "1/1"}},
		{Celulas: []string{"worker-5d8b", Status(StatusFalha, "CrashLoop"), "0/1"}},
		{Celulas: []string{"migracao", Status(StatusConcluido, "Completed"), "0/1"}, Concluida: true},
	}
	amostras["tabela"] = Tabela(colunas, linhas, 1, 80)

	abas := []Aba{{Titulo: "Pods", Contagem: 8}, {Titulo: "Deployments", Contagem: 4}}
	amostras["abas"] = Abas(abas, 0, 80) + "\n" + Abas(abas, 1, 40)
	chips := []Chip{{Chave: "status", Valor: "com problema"}, {Chave: "app=api"}}
	amostras["filtros"] = Filtros(chips, 8, 42, 80)

	corpo := "api-7f9c-x2k\nnamespace  payments"
	amostras["quadro"] = Quadro("Detalhe", corpo, 60, QuadroNormal) + Quadro("Foco", corpo, 60, QuadroFoco) +
		Quadro("Excluir", corpo, 60, QuadroDestrutivo)

	amostras["etapas"] = Etapas([]Etapa{
		{Rotulo: "contexto", Estado: EtapaFeita, Duracao: 300 * time.Millisecond},
		{Rotulo: "kubectl get", Estado: EtapaAtual, Duracao: 2 * time.Second},
		{Rotulo: "resposta", Estado: EtapaPendente},
	}, 64)

	amostras["cota"] = Cota("argo cd", 0.64, 64) + CotaTempo("sync", 0.4, 38*time.Second, 64)
	amostras["rota"] = Rota([]string{"projeto", "cluster", "argo cd"}, 1, 64)

	toast := NovoToast()
	for _, tp := range []TipoToast{ToastOk, ToastAviso, ToastErro} {
		toast, _ = toast.Mostrar(tp, "mensagem do toast")
		amostras["toast"] += toast.View()
	}

	amostras["confirmar"] = NovoConfirmarDestrutivo("excluir pod", "api-7f9c").View() +
		NovoConfirmarReversivel("reiniciar", "Reiniciar o deployment?").View()

	parado := func() bool { return false }
	amostras["estados"] = NovoEstadoCarregandoCom("lendo pods", parado).View() +
		NovoEstadoVazio("nada aqui", "esc limpa").View() + NovoEstadoErro("sem acesso", "r tenta").View()

	t := DefaultKeys()
	amostras["moldura"] = Moldura(100, 20, Partes{
		Instrumento: "viz", Rotulo: "INSTR. 05", Contexto: CaminhoContexto("staging", "payments") + "  " + IndicadorVivo("ao vivo"),
		Abas: Abas(abas, 0, 100), AbasCompactas: Abas(abas, 0, 60), Filtros: Filtros(chips, 8, 42, 100),
		Lista: Tabela(colunas, linhas, 1, 60), Detalhe: "api-7f9c\nnamespace  payments",
		Mensagem: Status(StatusRodando, "8 pods"), Hora: time.Date(2026, 10, 23, 12, 10, 0, 0, time.UTC),
		Teclas: []key.Binding{t.Lista.Abrir, t.Global.Sair},
	})
	amostras["masthead"] = Masthead(80, "viz", "INSTR. 05", CaminhoContexto("staging")) + StatusLine(80, Status(StatusRodando, "ok"), time.Now()) +
		KeyHints(80, t.Global.Sair)
	amostras["muito-pequena"] = Moldura(20, 5, Partes{Instrumento: "viz"})
	return amostras
}

// TestTextoSempreComCorDeFrente garante que todo texto visível sai sob uma cor de frente
// (Tinta, TintaSuave ou semântica), em vez da cor padrão do terminal.
// Exceções documentadas: espaços de preenchimento e sequências não SGR.
func TestTextoSempreComCorDeFrente(t *testing.T) {
	for _, escuro := range []bool{true, false} {
		usarTema(t, escuro, termenv.TrueColor, false)
		for nome, saida := range amostrasDeComponentes() {
			if achados := textoSemCor(saida); len(achados) > 0 {
				t.Errorf("escuro=%v %s: texto sem cor de frente: %q", escuro, nome, achados)
			}
		}
	}
}

func TestTextoSemCorDetector(t *testing.T) {
	if got := textoSemCor("\x1b[1mabc\x1b[0m"); len(got) != 1 {
		t.Errorf("negrito sem cor deveria ser detectado, obtido %q", got)
	}
	if got := textoSemCor("\x1b[1;38;2;1;2;3mabc\x1b[0m  "); len(got) != 0 {
		t.Errorf("texto colorido não deveria ser apontado: %q", got)
	}
}
