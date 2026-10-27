package confianca

import (
	"os"
	"path/filepath"
	"testing"
)

// usarRegistro põe o registro numa pasta temporária, longe do ~/.yby de quem roda.
func usarRegistro(t *testing.T) string {
	t.Helper()
	caminho := filepath.Join(t.TempDir(), "trusted-files.json")
	anterior := caminhoRegistro
	caminhoRegistro = func() (string, error) { return caminho, nil }
	t.Cleanup(func() { caminhoRegistro = anterior })
	return caminho
}

func escrever(t *testing.T, caminho, conteudo string) {
	t.Helper()
	if err := os.WriteFile(caminho, []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestConfiavel_SemRegistroNaoConfia(t *testing.T) {
	usarRegistro(t)
	arquivo := filepath.Join(t.TempDir(), "x.yaml")
	escrever(t, arquivo, "name: x")
	if Confiavel(arquivo, []byte("name: x")) {
		t.Error("arquivo sem registro não pode ser confiável")
	}
}

func TestConfiar_ConteudoIgualConfiaEMudadoNao(t *testing.T) {
	usarRegistro(t)
	arquivo := filepath.Join(t.TempDir(), "x.yaml")
	escrever(t, arquivo, "name: x")
	if err := Confiar(arquivo); err != nil {
		t.Fatal(err)
	}
	if !Confiavel(arquivo, []byte("name: x")) {
		t.Error("o conteúdo confirmado deveria ser confiável")
	}
	if Confiavel(arquivo, []byte("name: x\ncommand: rm")) {
		t.Error("conteúdo mudado não pode ser confiável")
	}
	outro := filepath.Join(t.TempDir(), "x.yaml")
	if Confiavel(outro, []byte("name: x")) {
		t.Error("a confiança é do caminho: o mesmo conteúdo noutro lugar não vale")
	}
}

func TestConfiar_CaminhoRelativoEAbsolutoSaoOMesmo(t *testing.T) {
	usarRegistro(t)
	dir := t.TempDir()
	t.Chdir(dir)
	escrever(t, filepath.Join(dir, "p.txt"), "prompt")
	if err := Confiar("p.txt"); err != nil {
		t.Fatal(err)
	}
	if !Confiavel(filepath.Join(dir, "p.txt"), []byte("prompt")) {
		t.Error("confiar pelo caminho relativo deveria valer para o absoluto")
	}
}

func TestRemover(t *testing.T) {
	usarRegistro(t)
	arquivo := filepath.Join(t.TempDir(), "x.yaml")
	escrever(t, arquivo, "name: x")
	if err := Remover(arquivo); err == nil {
		t.Error("remover o que não está no registro deveria dar erro")
	}
	if err := Confiar(arquivo); err != nil {
		t.Fatal(err)
	}
	if err := Remover(arquivo); err != nil {
		t.Fatal(err)
	}
	if Confiavel(arquivo, []byte("name: x")) {
		t.Error("depois de remover, o arquivo não pode ser confiável")
	}
}

func TestConfiavel_RegistroIlegivelNaoConfia(t *testing.T) {
	caminho := usarRegistro(t)
	escrever(t, caminho, "{não é json")
	arquivo := filepath.Join(t.TempDir(), "x.yaml")
	escrever(t, arquivo, "name: x")
	if Confiavel(arquivo, []byte("name: x")) {
		t.Error("registro ilegível não pode dar confiança")
	}
	if err := Confiar(arquivo); err == nil {
		t.Error("confiar com o registro ilegível deveria dar erro, e não apagá-lo")
	}
}
