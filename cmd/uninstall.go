package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/spf13/cobra"
)

// Variáveis mockáveis para testes
var osExecutable = os.Executable
var osRemove = os.Remove

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
			return errors.Wrap(err, errors.ErrCodeExec, "falha ao localizar o binário")
		}

		ui.Title(w, "uninstall", "versão "+Version)
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

		if err := osRemove(exePath); err != nil {
			e := errors.Wrap(err, errors.ErrCodeExec, "falha ao remover o binário")
			// Dica caso seja erro de permissão
			if strings.Contains(err.Error(), "permission denied") {
				e = e.WithHint("rode com sudo: sudo yby uninstall")
			} else {
				e = e.WithHint("feche outros terminais que estejam usando o yby e tente de novo")
			}
			return e
		}

		fmt.Fprintln(w)
		ui.Check(w, ui.CheckOk, "Yby CLI desinstalado", "", "")
		ui.Check(w, ui.CheckNeutro, "~/.yby fica no lugar, com a configuração e os dados", "", "")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
	uninstallCmd.Flags().BoolP("yes", "y", false, "Remove sem perguntar (para scripts e terminais não interativos)")
}
