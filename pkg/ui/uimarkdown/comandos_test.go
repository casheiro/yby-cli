package uimarkdown

import (
	"reflect"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestComandos(t *testing.T) {
	casos := []struct {
		nome string
		md   string
		want []string
	}{
		{"sem blocos", "só texto, com `kubectl` no meio", nil},
		{"um comando", "Rode:\n\n```bash\nkubectl delete pv pvc-3f1a9c2e\n```\n", []string{"kubectl delete pv pvc-3f1a9c2e"}},
		{
			"vários, na ordem, sem as linhas vazias do fim",
			"```sh\nkubectl get pods\n\n\n```\n\ntexto\n\n```Shell\nkubectl logs api\nkubectl describe pod api\n```",
			[]string{"kubectl get pods", "kubectl logs api\nkubectl describe pod api"},
		},
		{"outras linguagens ficam de fora", "```yaml\nkind: Pod\n```\n\n```\nsem linguagem\n```", nil},
		{"bloco vazio fica de fora", "```bash\n\n```", nil},
		{"cerca de til e bloco sem fim", "~~~console\n$ yby doctor\n~~~\n\n```pwsh\nGet-Process", []string{"yby doctor", "Get-Process"}},
		{
			"console: só as linhas de comando, sem o prompt nem a saída",
			"```console\n$ kubectl get pv\nNAME   STATUS\npv-1   Released\n# kubectl delete pv pv-1\n```",
			[]string{"kubectl get pv\nkubectl delete pv pv-1"},
		},
		{"console sem prompt fica como veio", "```console\nyby doctor\n```", []string{"yby doctor"}},
		{"shell com $ em todas as linhas perde o prompt", "```bash\n$ kubectl get pv\n\n  $ kubectl get pvc\n```", []string{"kubectl get pv\nkubectl get pvc"}},
		{
			"shell com $ só em algumas linhas fica como veio; # é comentário",
			"```sh\n# limpa os volumes\n$ kubectl delete pv x\n```",
			[]string{"# limpa os volumes\n$ kubectl delete pv x"},
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := Comandos(c.md); !reflect.DeepEqual(got, c.want) {
				t.Errorf("Comandos = %q, esperado %q", got, c.want)
			}
		})
	}
}

// documento tem de tudo: texto, tabela, código de outra linguagem e dois comandos, com
// um bloco de shell vazio no meio (que não conta como comando).
const documento = "Os volumes:\n\n| Volume | Status |\n|---|---|\n| pv-1 | Released |\n\n" +
	"```yaml\nkind: PersistentVolume\n```\n\n```sh\nkubectl get pv\n```\n\n```bash\n\n```\n\n" +
	"Para apagar:\n\n```bash\nkubectl delete pv pv-1\n```\n"

func TestRenderizarComTecla(t *testing.T) {
	usarModo(t, ui.Opcoes{Tema: ui.TemaEscuro, ASCII: true}, termenv.Ascii)
	r, err := Novo(60)
	if err != nil {
		t.Fatal(err)
	}
	rotulos := func(saida string) []string {
		var achados []string
		for _, l := range strings.Split(ansi.Strip(saida), "\n") {
			if i := strings.Index(l, "COMANDO"); i >= 0 {
				achados = append(achados, strings.Trim(l[i:], "-+ "))
			}
		}
		return achados
	}

	// Só o quadro do índice pedido leva a tecla; o bloco vazio não conta.
	saida, err := r.RenderizarComTecla(documento, 1, "ctrl+y", "copia")
	if err != nil {
		t.Fatal(err)
	}
	if got := rotulos(saida); !reflect.DeepEqual(got, []string{"COMANDO", "COMANDO", "COMANDO · ctrl+y copia"}) {
		t.Errorf("rótulos com o índice 1: %q\n%s", got, saida)
	}
	if !strings.Contains(saida, "pv-1") || !strings.Contains(saida, "YAML") || !strings.Contains(saida, "Para apagar") {
		t.Errorf("tabela, código e texto passam como no Renderizar:\n%s", saida)
	}

	// -1 deixa todos sem tecla.
	saida, _ = r.RenderizarComTecla(documento, -1, "ctrl+y", "copia")
	if got := rotulos(saida); !reflect.DeepEqual(got, []string{"COMANDO", "COMANDO", "COMANDO"}) {
		t.Errorf("rótulos sem índice: %q", got)
	}
}

// Com a mesma tecla do Renderizar no único comando, a saída é a mesma, também com cor.
func TestRenderizarComTecla_IgualAoRenderizar(t *testing.T) {
	usarModo(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.TrueColor)
	r, err := Novo(70)
	if err != nil {
		t.Fatal(err)
	}
	md := strings.Replace(documento, "```bash\n\n```\n\n", "", 1)
	md = strings.Replace(md, "```sh\nkubectl get pv\n```\n\n", "", 1)
	esperado, _ := r.Renderizar(md)
	obtido, err := r.RenderizarComTecla(md, 0, "c", "copia")
	if err != nil {
		t.Fatal(err)
	}
	if obtido != esperado {
		t.Errorf("deveria sair igual ao Renderizar:\n--- esperado\n%s\n--- obtido\n%s", esperado, obtido)
	}
}
