package plugin

import (
	"fmt"
	"os"
	"testing"
)

// TestMain põe o pacote numa home temporária: vários testes instalam plugins e gravam o
// registro de confiança sem trocar a home, e sem isso escreviam no ~/.yby/plugins de
// quem roda os testes. Os testes que trocam a home com t.Setenv continuam valendo.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "yby-plugin-testes-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "home temporária dos testes:", err)
		os.Exit(1)
	}
	os.Setenv("HOME", home)
	os.Setenv("USERPROFILE", home)
	codigo := m.Run()
	os.RemoveAll(home)
	os.Exit(codigo)
}
