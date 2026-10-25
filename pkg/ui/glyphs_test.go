package ui

import (
	"reflect"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestGlifos_LarguraASCIIIgualUnicode(t *testing.T) {
	vu, va := reflect.ValueOf(Unicode), reflect.ValueOf(ASCII)
	for i := 0; i < vu.NumField(); i++ {
		nome := vu.Type().Field(i).Name
		if nome == "Spinner" {
			continue
		}
		u, a := vu.Field(i).String(), va.Field(i).String()
		if u == "" || a == "" {
			t.Errorf("%s vazio (Unicode=%q ASCII=%q)", nome, u, a)
			continue
		}
		if wu, wa := ansi.StringWidth(u), ansi.StringWidth(a); wu != wa {
			t.Errorf("%s: largura Unicode %d != ASCII %d (%q vs %q)", nome, wu, wa, u, a)
		}
	}
}

func TestGlifos_Spinner(t *testing.T) {
	if len(Unicode.Spinner) != 4 || len(ASCII.Spinner) != 4 {
		t.Fatalf("spinner deve ter 4 quadros: %d e %d", len(Unicode.Spinner), len(ASCII.Spinner))
	}
	for i := range Unicode.Spinner {
		if ansi.StringWidth(Unicode.Spinner[i]) != ansi.StringWidth(ASCII.Spinner[i]) {
			t.Errorf("quadro %d com larguras diferentes", i)
		}
	}
}

func TestG_SegueModoAtual(t *testing.T) {
	usarTema(t, true, termenv.TrueColor, false)
	if !reflect.DeepEqual(G(), Unicode) {
		t.Error("esperado conjunto Unicode")
	}
	Configurar(Opcoes{Tema: TemaEscuro, ASCII: true})
	if !reflect.DeepEqual(G(), ASCII) {
		t.Error("esperado conjunto ASCII")
	}
}
