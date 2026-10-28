package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/casheiro/yby-cli/v5/pkg/ui"
)

func TestGerar_NaoPassaDaLargura(t *testing.T) {
	anterior := ui.Atual()
	t.Cleanup(func() { ui.Configurar(anterior) })

	for _, ascii := range []bool{false, true} {
		for _, largura := range []int{60, 120} {
			ui.Configurar(ui.Opcoes{Tema: ui.TemaEscuro, ASCII: ascii})
			var b bytes.Buffer
			Gerar(&b, largura)
			saida := b.String()
			for _, secao := range []string{"PALETA", "GLIFOS", "MARCA", "TABELA COM STATUS", "QUADRO", "ETAPAS", "COTA", "ROTA", "TOAST", "CONFIRMAR", "ESTADOS", "DOCTOR", "UP", "ERRO", "AJUDA"} {
				if !strings.Contains(strings.ToUpper(saida), secao) {
					t.Errorf("largura %d: seção %q ausente", largura, secao)
				}
			}
			for i, l := range strings.Split(saida, "\n") {
				if w := ansi.StringWidth(l); w > largura {
					t.Errorf("largura %d (ascii=%v): linha %d tem %d células: %q", largura, ascii, i+1, w, l)
				}
			}
		}
	}
}

func TestExecutar_Flags(t *testing.T) {
	anterior := ui.Atual()
	t.Cleanup(func() { ui.Configurar(anterior) })
	semEnv := func(string) string { return "" }

	var b bytes.Buffer
	if err := executar([]string{"--tema", "claro", "--largura", "80", "--ascii", "--sem-cor"}, &b, semEnv); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "v cluster yby-local") {
		t.Errorf("esperava glifos ASCII na saída")
	}
	if o := ui.Atual(); o.Tema != ui.TemaClaro || !o.ASCII || !o.SemCor {
		t.Errorf("opções = %+v", o)
	}
	if err := executar([]string{"--tema", "roxo"}, &b, semEnv); err == nil {
		t.Error("esperava erro para tema inválido")
	}
	if err := executar([]string{"--largura", "10"}, &b, semEnv); err == nil {
		t.Error("esperava erro para largura pequena")
	}
	if err := executar([]string{"--nada"}, &b, semEnv); err == nil {
		t.Error("esperava erro para flag desconhecida")
	}
}
