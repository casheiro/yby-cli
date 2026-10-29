package cmd

import (
	stderrors "errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/spf13/cobra"
)

// Variáveis mockáveis para testes
var osExecutable = os.Executable
var osRemove = os.Remove
var osRename = os.Rename

// removerDepois apaga um arquivo depois que o yby sair (no Windows, o executável em uso
// não pode ser apagado); sistemaUninstall escolhe o caminho do Windows. Trocadas nos
// testes.
var removerDepois = agendarRemocao
var sistemaUninstall = runtime.GOOS

// pidAtual entra no nome do executável renomeado no Windows; trocada nos testes.
var pidAtual = os.Getpid

// uninstallCmd represents the uninstall command
var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Desinstala o Yby CLI do sistema",
	Long: `Remove o binário do Yby CLI do sistema.
Esta ação é irreversível e removerá apenas o executável atual.
Arquivos de configuração e dados em ~/.yby não serão removidos automaticamente.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := cmd.OutOrStdout()
		exePath, err := osExecutable()
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "falha ao localizar o binário").
				WithHint("apague o yby à mão; where yby (ou which yby) mostra onde ele está")
		}

		ui.Title(w, "uninstall", "versão "+versaoInstalada().Versao)
		ui.Check(w, ui.CheckNeutro, "remove o executável", exePath, "")
		if sim, _ := cmd.Flags().GetBool("yes"); !sim {
			confirmado, err := prompter.Confirm("Remover o Yby CLI deste computador?", false)
			if err != nil {
				return errors.Wrap(err, errors.ErrCodeValidation, "desinstalação não confirmada").
					WithHint("rode yby uninstall num terminal ou confirme com yby uninstall --yes")
			}
			if !confirmado {
				ui.Check(w, ui.CheckNeutro, "desinstalação cancelada", "", "")
				return nil
			}
		}

		if sistemaUninstall == "windows" {
			return desinstalarNoWindows(cmd, exePath)
		}

		if err := osRemove(exePath); err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "falha ao remover o binário").
				WithHint(dicaRemocao(err, "rode com sudo: sudo yby uninstall"))
		}

		fmt.Fprintln(w)
		ui.Check(w, ui.CheckOk, "Yby CLI desinstalado", "", "")
		ui.Check(w, ui.CheckNeutro, "~/.yby fica no lugar, com a configuração e os dados", "", "")
		return nil
	},
}

// dicaRemocao escolhe a dica de uma falha ao remover ou renomear o executável. O erro de
// permissão vem traduzido no Windows ("Acesso negado"): a conferência é pelo tipo.
func dicaRemocao(err error, dicaPermissao string) string {
	if stderrors.Is(err, fs.ErrPermission) {
		return dicaPermissao
	}
	return "feche outros terminais que estejam usando o yby e tente de novo"
}

// desinstalarNoWindows tira o yby do lugar no Windows, que não deixa apagar o executável
// em uso mas deixa renomeá-lo: o yby.exe vira .yby.exe.removido-<pid> na mesma pasta (sem
// a extensão .exe, o PATH não o acha mais) e um cmd.exe sem janela o apaga quando este yby
// sair. Se outro yby seguir aberto com o mesmo executável (um plugin rodando em outro
// terminal), o cmd.exe desiste depois de um minuto e o arquivo fica: a tela diz qual é.
func desinstalarNoWindows(cmd *cobra.Command, exePath string) error {
	w := cmd.OutOrStdout()
	removido := filepath.Join(filepath.Dir(exePath),
		fmt.Sprintf(".%s.removido-%d", filepath.Base(exePath), pidAtual()))
	if err := osRename(exePath, removido); err != nil {
		return errors.Wrap(err, errors.ErrCodeExec, "falha ao remover o binário").
			WithHint(dicaRemocao(err, "abra o terminal como administrador e rode yby uninstall de novo"))
	}

	fmt.Fprintln(w)
	ui.Check(w, ui.CheckOk, "Yby CLI desinstalado", "", "")
	if err := removerDepois(removido); err != nil {
		// o yby já saiu do PATH; sobra só o arquivo renomeado
		ui.Check(w, ui.CheckAviso, "apague à mão o arquivo que ficou", removido, "")
	} else {
		ui.Check(w, ui.CheckNeutro, "o arquivo some do disco quando este yby fechar", "", "")
		ui.Check(w, ui.CheckNeutro, "com outro yby aberto, ele fica; apague à mão", removido, "")
	}
	ui.Check(w, ui.CheckNeutro, "~/.yby fica no lugar, com a configuração e os dados", "", "")
	return nil
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
	uninstallCmd.Flags().BoolP("yes", "y", false, "Remove sem perguntar (para scripts e terminais não interativos)")
}
