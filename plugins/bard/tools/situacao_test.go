package tools

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSituacoes_DisponibilidadePorFamilia(t *testing.T) {
	Reset()
	defer Reset()
	for _, nome := range []string{"kubectl_get", "kubectl_logs", "sentinel_scan", "atlas_blueprint"} {
		Register(&Tool{Name: nome, Description: "Faz " + nome})
	}
	Register(&Tool{Name: "backup", Description: "Backup", Externa: true})
	registrarInvalida("~/.yby/tools/quebrada.yaml", "falta o campo command")

	procuras := 0
	procurar := func(string) (string, error) { procuras++; return "", errors.New("ausente") }
	plugin := func(nome string) (string, error) {
		if nome == "atlas" {
			return "/bin/atlas", nil
		}
		return "", errors.New("ausente")
	}
	lista := situacoes(procurar, plugin)

	type resumo struct {
		nome       string
		disponivel bool
		nota       string
	}
	var got []resumo
	for _, s := range lista {
		got = append(got, resumo{s.Nome, s.Disponivel, s.Nota})
	}
	want := []resumo{
		{"atlas_blueprint", true, ""},
		{"backup", true, ""},
		{"kubectl_get", false, "kubectl não está no PATH"},
		{"kubectl_logs", false, "kubectl não está no PATH"},
		{"sentinel_scan", false, "plugin sentinel não instalado"},
		{"~/.yby/tools/quebrada.yaml", false, "falta o campo command"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("situações:\n got %v\nwant %v", got, want)
	}
	if procuras != 1 {
		t.Errorf("o kubectl deveria ser procurado uma vez, foi %d", procuras)
	}
	if ultima := lista[len(lista)-1]; !ultima.Invalida || !ultima.Externa {
		t.Errorf("a externa inválida vem marcada: %+v", ultima)
	}
	if lista[0].Familia != "atlas" || !lista[1].Externa {
		t.Errorf("família e origem: %+v %+v", lista[0], lista[1])
	}
}

func TestLoadExternalTools_RegistraInvalidas(t *testing.T) {
	Reset()
	defer Reset()
	Register(&Tool{Name: "kubectl_get"})

	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	tools := filepath.Join(dir, ".yby", "tools")
	if err := os.MkdirAll(tools, 0o755); err != nil {
		t.Fatal(err)
	}
	arquivos := map[string]string{
		"a-sem-comando.yaml": "name: backup\n",
		"b-sem-nome.yaml":    "command: echo\n",
		"c-vazia.yaml":       "description: nada\n",
		"d-quebrada.yaml":    "name: [\n",
		"e-nativa.yaml":      "name: kubectl_get\ncommand: echo\n",
		"f-boa.yaml":         "name: ola\ncommand: echo ola\n",
	}
	for nome, conteudo := range arquivos {
		if err := os.WriteFile(filepath.Join(tools, nome), []byte(conteudo), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	LoadExternalTools()

	want := []Invalida{
		{"~/.yby/tools/a-sem-comando.yaml", "falta o campo command"},
		{"~/.yby/tools/b-sem-nome.yaml", "falta o campo name"},
		{"~/.yby/tools/c-vazia.yaml", "faltam os campos name e command"},
		{"~/.yby/tools/d-quebrada.yaml", "o YAML não é válido"},
		{"~/.yby/tools/e-nativa.yaml", "o nome kubectl_get já é de uma ferramenta nativa"},
	}
	if got := Invalidas(); !reflect.DeepEqual(got, want) {
		t.Errorf("inválidas:\n got %v\nwant %v", got, want)
	}
	if ola := Get("ola"); ola == nil || !ola.Externa {
		t.Errorf("a externa boa deveria carregar marcada como externa: %+v", ola)
	}

	// Carregar de novo não repete as inválidas.
	LoadExternalTools()
	if got := Invalidas(); len(got) != len(want) {
		t.Errorf("a segunda carga repetiu as inválidas: %v", got)
	}
}
