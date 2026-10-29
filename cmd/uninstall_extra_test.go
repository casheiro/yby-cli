package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/casheiro/yby-cli/v5/pkg/errors"
)

// saveUninstallMocks salva e restaura as variáveis mockáveis do uninstall (incluindo o
// prompter e a flag --yes).
func saveUninstallMocks(t *testing.T) {
	t.Helper()
	origExec := osExecutable
	origRemove := osRemove
	origRename, origRemoverDepois, origSistema := osRename, removerDepois, sistemaUninstall
	origPid := pidAtual
	origPrompter := prompter
	t.Cleanup(func() {
		osExecutable = origExec
		osRemove = origRemove
		osRename, removerDepois, sistemaUninstall = origRename, origRemoverDepois, origSistema
		pidAtual = origPid
		prompter = origPrompter
		_ = uninstallCmd.Flags().Set("yes", "false")
	})
	// o caminho de fora do Windows em toda máquina; os testes do Windows trocam
	sistemaUninstall = "linux"
	osRename = func(de, para string) error {
		t.Fatalf("não deveria renomear %s fora do Windows", de)
		return nil
	}
	removerDepois = func(caminho string) error {
		t.Fatalf("não deveria agendar a remoção de %s fora do Windows", caminho)
		return nil
	}
}

// responderUninstall faz o Confirm do uninstall devolver resposta e err.
func responderUninstall(resposta bool, err error) {
	prompter = &mockPrompter{confirmFunc: func(string, bool) (bool, error) { return resposta, err }}
}

func TestUninstallCmd_ExecutableError(t *testing.T) {
	saveUninstallMocks(t)

	osExecutable = func() (string, error) {
		return "", fmt.Errorf("falha ao localizar binário")
	}
	osRemove = func(name string) error {
		t.Fatal("não deveria chamar Remove quando Executable falha")
		return nil
	}
	responderUninstall(true, nil)

	assert.Error(t, uninstallCmd.RunE(uninstallCmd, []string{}))
}

func TestUninstallCmd_UserCancels(t *testing.T) {
	saveUninstallMocks(t)

	osExecutable = func() (string, error) { return "/usr/local/bin/yby", nil }
	osRemove = func(name string) error {
		t.Fatal("não deveria chamar Remove quando o usuário cancela")
		return nil
	}
	responderUninstall(false, nil)

	assert.NoError(t, uninstallCmd.RunE(uninstallCmd, []string{}))
}

func TestUninstallCmd_UserConfirmsYes(t *testing.T) {
	saveUninstallMocks(t)

	// Cria um arquivo temporário para simular o binário
	tmpFile, err := os.CreateTemp("", "yby-test-*")
	assert.NoError(t, err)
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath) // limpeza caso osRemove falhe

	removeCalled := false
	osExecutable = func() (string, error) { return tmpPath, nil }
	osRemove = func(name string) error {
		assert.Equal(t, tmpPath, name, "deveria remover o caminho correto")
		removeCalled = true
		return nil
	}
	var perguntaPadrao *bool
	prompter = &mockPrompter{confirmFunc: func(_ string, padrao bool) (bool, error) {
		perguntaPadrao = &padrao
		return true, nil
	}}

	assert.NoError(t, uninstallCmd.RunE(uninstallCmd, []string{}))

	assert.True(t, removeCalled, "osRemove deveria ter sido chamado")
	require.NotNil(t, perguntaPadrao)
	assert.False(t, *perguntaPadrao, "a resposta padrão da pergunta é não")
}

func TestUninstallCmd_SemTerminal(t *testing.T) {
	saveUninstallMocks(t)

	osExecutable = func() (string, error) { return "/usr/local/bin/yby", nil }
	osRemove = func(name string) error {
		t.Fatal("sem terminal e sem --yes, nada deveria ser removido")
		return nil
	}
	responderUninstall(false, erroSemTerminal())

	err := uninstallCmd.RunE(uninstallCmd, []string{})
	var yerr *errors.YbyError
	require.ErrorAs(t, err, &yerr)
	assert.Contains(t, yerr.Hint, "--yes")
}

func TestUninstallCmd_FlagYes_NaoPergunta(t *testing.T) {
	saveUninstallMocks(t)

	removeCalled := false
	osExecutable = func() (string, error) { return "/usr/local/bin/yby", nil }
	osRemove = func(name string) error {
		removeCalled = true
		return nil
	}
	prompter = &mockPrompter{confirmFunc: func(string, bool) (bool, error) {
		t.Fatal("com --yes não deveria perguntar")
		return false, nil
	}}
	require.NoError(t, uninstallCmd.Flags().Set("yes", "true"))

	assert.NoError(t, uninstallCmd.RunE(uninstallCmd, []string{}))
	assert.True(t, removeCalled)
}

func TestUninstallCmd_RemovePermissionDenied(t *testing.T) {
	saveUninstallMocks(t)

	osExecutable = func() (string, error) { return "/usr/local/bin/yby", nil }
	osRemove = func(name string) error {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrPermission}
	}
	responderUninstall(true, nil)

	// Deve retornar erro com a dica de sudo
	err := uninstallCmd.RunE(uninstallCmd, []string{})
	var yerr *errors.YbyError
	require.ErrorAs(t, err, &yerr)
	assert.Contains(t, yerr.Hint, "sudo")
}

// O erro de permissão do Windows vem traduzido ("Acesso negado"): a dica sai pelo tipo do
// erro, não pelo texto.
func TestUninstallCmd_RemovePermissaoPeloTipo(t *testing.T) {
	saveUninstallMocks(t)

	osExecutable = func() (string, error) { return "/usr/local/bin/yby", nil }
	osRemove = func(name string) error {
		return &fs.PathError{Op: "remove", Path: name, Err: fmt.Errorf("Acesso negado.: %w", fs.ErrPermission)}
	}
	responderUninstall(true, nil)

	err := uninstallCmd.RunE(uninstallCmd, []string{})
	var yerr *errors.YbyError
	require.ErrorAs(t, err, &yerr)
	assert.Contains(t, yerr.Hint, "sudo")
}

func TestUninstallCmd_RemoveOtherError(t *testing.T) {
	saveUninstallMocks(t)

	osExecutable = func() (string, error) { return "/usr/local/bin/yby", nil }
	osRemove = func(name string) error {
		return fmt.Errorf("erro desconhecido ao remover arquivo: permission denied")
	}
	responderUninstall(true, nil)

	// sem um erro de permissão de verdade, a dica não é a do sudo (mesmo com o texto)
	err := uninstallCmd.RunE(uninstallCmd, []string{})
	var yerr *errors.YbyError
	require.ErrorAs(t, err, &yerr)
	assert.NotContains(t, yerr.Hint, "sudo")
	assert.Contains(t, yerr.Hint, "feche outros terminais")
}

// ========================================================
// Windows: renomeia o executável em uso e agenda a remoção
// ========================================================

func TestUninstallCmd_Windows_RenomeiaEAgenda(t *testing.T) {
	saveUninstallMocks(t)
	sistemaUninstall = "windows"

	exe := filepath.Join("C:", "Users", "x", "go", "bin", "yby.exe")
	osExecutable = func() (string, error) { return exe, nil }
	osRemove = func(name string) error {
		t.Fatal("no Windows o executável em uso não é apagado direto")
		return nil
	}
	var de, para, agendado string
	osRename = func(a, b string) error { de, para = a, b; return nil }
	removerDepois = func(caminho string) error { agendado = caminho; return nil }
	responderUninstall(true, nil)

	b := saidaDoComando(t, uninstallCmd)
	require.NoError(t, uninstallCmd.RunE(uninstallCmd, []string{}))

	assert.Equal(t, exe, de)
	assert.Equal(t, filepath.Dir(exe), filepath.Dir(para), "o nome temporário fica na mesma pasta")
	assert.NotEqual(t, ".exe", filepath.Ext(para), "o nome temporário não pode ser achado no PATH")
	assert.True(t, strings.HasPrefix(filepath.Base(para), ".yby.exe.removido-"), para)
	assert.Equal(t, para, agendado, "a remoção agendada é a do arquivo renomeado")
	assert.Contains(t, b.String(), "Yby CLI desinstalado")
	assert.Contains(t, b.String(), "quando este yby fechar")
	// com outro yby aberto, o cmd.exe desiste e o arquivo fica: a tela diz qual é
	assert.Contains(t, b.String(), "com outro yby aberto")
	assert.Contains(t, b.String(), para)
}

func TestUninstallCmd_Windows_RenomearSemPermissao(t *testing.T) {
	saveUninstallMocks(t)
	sistemaUninstall = "windows"

	osExecutable = func() (string, error) { return `C:\Program Files\yby\yby.exe`, nil }
	osRename = func(a, b string) error {
		return &os.LinkError{Op: "rename", Old: a, New: b, Err: fmt.Errorf("Acesso negado.: %w", fs.ErrPermission)}
	}
	responderUninstall(true, nil)

	err := uninstallCmd.RunE(uninstallCmd, []string{})
	var yerr *errors.YbyError
	require.ErrorAs(t, err, &yerr)
	assert.Contains(t, yerr.Hint, "administrador")
	assert.NotContains(t, yerr.Hint, "sudo")
}

func TestUninstallCmd_Windows_RenomearOutroErro(t *testing.T) {
	saveUninstallMocks(t)
	sistemaUninstall = "windows"

	osExecutable = func() (string, error) { return `C:\yby\yby.exe`, nil }
	osRename = func(a, b string) error { return fmt.Errorf("arquivo bloqueado") }
	responderUninstall(true, nil)

	err := uninstallCmd.RunE(uninstallCmd, []string{})
	var yerr *errors.YbyError
	require.ErrorAs(t, err, &yerr)
	assert.Contains(t, yerr.Hint, "feche outros terminais")
}

// Se o agendamento falhar, o yby já saiu do PATH: a tela diz qual arquivo apagar à mão.
func TestUninstallCmd_Windows_AgendamentoFalha(t *testing.T) {
	saveUninstallMocks(t)
	sistemaUninstall = "windows"

	osExecutable = func() (string, error) { return `C:\yby\yby.exe`, nil }
	var para string
	osRename = func(a, b string) error { para = b; return nil }
	removerDepois = func(string) error { return fmt.Errorf("cmd.exe não encontrado") }
	responderUninstall(true, nil)

	b := saidaDoComando(t, uninstallCmd)
	require.NoError(t, uninstallCmd.RunE(uninstallCmd, []string{}))
	assert.Contains(t, b.String(), "apague à mão")
	assert.Contains(t, b.String(), filepath.Base(para))
	assert.NotContains(t, b.String(), "quando este yby fechar")
}

// O título do uninstall mostra a versão do build info quando o binário veio do go install.
func TestUninstallCmd_TituloComVersaoDoBuildInfo(t *testing.T) {
	saveUninstallMocks(t)
	usarBuildInfo(t, "v5.8.4", nil)

	responderUninstall(false, nil)
	osExecutable = func() (string, error) { return "/usr/local/bin/yby", nil }
	b := saidaDoComando(t, uninstallCmd)
	require.NoError(t, uninstallCmd.RunE(uninstallCmd, []string{}))
	assert.Contains(t, b.String(), "v5.8.4")
	assert.NotContains(t, b.String(), "versão dev")
}

// A tela do Windows diz a verdade: o yby sai do PATH na hora e o arquivo some do disco
// quando este yby fechar.
func TestUninstallGolden_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		// a tela traz o caminho do arquivo renomeado, montado pelo filepath do sistema
		t.Skip(`o caminho C:\… do golden só se monta assim no Windows`)
	}
	usarSaidaTexto(t, true)
	prepararUninstall(t, nil)
	sistemaUninstall = "windows"
	osExecutable = func() (string, error) { return `C:\Users\anton\go\bin\yby.exe`, nil }
	osRename = func(string, string) error { return nil }
	removerDepois = func(string) error { return nil }
	pidAtual = func() int { return 4242 }

	b := saidaDoComando(t, uninstallCmd)
	require.NoError(t, uninstallCmd.RunE(uninstallCmd, []string{}))
	conferirGolden(t, "uninstall_windows", b.String())
	conferirSemEstiloAntigo(t, b.String())
}
