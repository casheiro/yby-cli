package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// Variáveis mockáveis para testes
var osExecutable = os.Executable
var osRemove = os.Remove
var stdinReader io.Reader = os.Stdin

// uninstallCmd represents the uninstall command
var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Desinstala o Yby CLI do sistema",
	Long: `Remove o binário do Yby CLI do sistema.
Esta ação é irreversível e removerá apenas o executável atual.
Arquivos de configuração e dados em ~/.yby PAG não serã removidos automaticamente.`,
	Run: func(cmd *cobra.Command, args []string) {
		exePath, err := osExecutable()
		if err != nil {
			fmt.Printf("%s Erro ao localizar o binário: %v\n", crossStyle.Render("❌"), err)
			return
		}

		fmt.Println(titleStyle.Render("🗑️  Yby Uninstall"))
		fmt.Printf("O executável localizado em: %s será removido.\n", exePath)
		fmt.Println(warningStyle.Render("⚠️  Tem certeza que deseja continuar? (y/N)"))

		reader := bufio.NewReader(stdinReader)
		fmt.Print("-> ")
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response != "y" && response != "s" && response != "sim" && response != "yes" {
			fmt.Println("Operação cancelada.")
			return
		}

		fmt.Printf("Removendo %s... ", exePath)
		if err := osRemove(exePath); err != nil {
			fmt.Printf("\n%s Erro ao remover o arquivo: %v\n", crossStyle.Render("❌"), err)
			// Dica caso seja erro de permissão
			if strings.Contains(err.Error(), "permission denied") {
				fmt.Println(grayStyle.Render("💡 Tente rodar com sudo: sudo yby uninstall"))
			}
			return
		}

		fmt.Println("\n" + checkStyle.Render("✅ Yby CLI desinstalado com sucesso!"))
		fmt.Println("Até logo! 👋")
	},
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}
