package tui

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// folgaMarca é o espaço da marca "… N linhas omitidas", que fica além do teto.
const folgaMarca = 64

func linhasNumeradas(n, largura int) string {
	var sb strings.Builder
	for i := 1; i <= n; i++ {
		linha := fmt.Sprintf("linha %05d ", i)
		sb.WriteString(linha + strings.Repeat("x", largura-len(linha)) + "\n")
	}
	return sb.String()
}

func TestCortar_TextoQueCabeFicaIgual(t *testing.T) {
	texto := "Status: Running\nReady: 1/1\n"
	if got := Cortar(texto, 1024); got != texto {
		t.Errorf("texto dentro do teto deveria ficar igual: %q", got)
	}
}

// Uma saída de 200 KB fica no teto, com o começo (o cabeçalho de uma lista), o fim (o
// fim de um log) e a marca com as linhas omitidas.
func TestCortar_LinhasDoMeioOmitidas(t *testing.T) {
	texto := linhasNumeradas(2000, 100) // 200 KB
	got := Cortar(texto, TetoFerramenta)
	if len(got) > TetoFerramenta+folgaMarca {
		t.Fatalf("o corte passou do teto: %d bytes", len(got))
	}
	if !strings.HasPrefix(got, "linha 00001 ") {
		t.Errorf("o começo deveria ficar: %q", got[:40])
	}
	if !strings.Contains(got, "linha 02000 ") {
		t.Error("o fim deveria ficar")
	}
	inteiras := strings.Count(got, "linha ")
	marca := fmt.Sprintf("… %d linhas omitidas", 2000-inteiras)
	if !strings.Contains(got, marca) {
		t.Errorf("faltou a marca %q:\n%s", marca, got)
	}
}

// Um JSON numa linha só (200 KB) é cortado dentro da linha, sem partir um caractere.
func TestCortar_LinhaUnicaGigante(t *testing.T) {
	texto := strings.Repeat("ação ", 40_000)
	got := Cortar(texto, TetoFerramenta)
	if len(got) > TetoFerramenta+folgaMarca {
		t.Fatalf("o corte passou do teto: %d bytes", len(got))
	}
	if !utf8.ValidString(got) {
		t.Error("o corte partiu um caractere")
	}
	if !strings.Contains(got, "…") {
		t.Errorf("o corte deveria ser marcado: %q", got[len(got)-20:])
	}
}

func TestCortar_UmaLinhaOmitidaNoSingular(t *testing.T) {
	texto := "aaaa\nbbbb\ncccc\n"
	got := Cortar(texto, 10)
	if !strings.Contains(got, "… 1 linha omitida") {
		t.Errorf("esperava a marca no singular: %q", got)
	}
}

func TestRecentes(t *testing.T) {
	casos := []struct {
		nome     string
		tamanhos []int
		teto     int
		quantos  int
	}{
		{"todos cabem", []int{10, 10, 10}, 100, 3},
		{"só os últimos", []int{50, 30, 30, 30}, 100, 3},
		{"o mais novo não cabe", []int{10, 200}, 100, 0},
		{"para no primeiro que não cabe", []int{10, 90, 10}, 50, 1},
		{"vazio", nil, 100, 0},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := Recentes(c.tamanhos, c.teto); got != c.quantos {
				t.Errorf("Recentes(%v, %d) = %d, esperado %d", c.tamanhos, c.teto, got, c.quantos)
			}
		})
	}
}
