/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/AlecAivazis/survey/v2"
	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/spf13/cobra"
)

// setupCmd represents the setup command
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configura o ambiente de desenvolvimento local",
	Long: `Verifica e auxilia na instalação das dependências necessárias para
rodar o ambiente de desenvolvimento localmente (kubectl, helm, k3d, direnv).

Exemplo:
  yby setup
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(titleStyle.Render("🚀 Yby Setup - Configuração de Ambiente"))
		fmt.Println("---------------------------------------")

		// 0. Detect Profile
		profile, _ := cmd.Flags().GetString("profile")
		if profile != "dev" && profile != "server" {
			return errors.New(errors.ErrCodeValidation, "Perfil inválido. Use 'dev' ou 'server'")
		}
		fmt.Printf("🔧 Perfil selecionado: %s\n", profile)

		// 1. Check Tools
		type Tool struct {
			Name        string
			Cmd         string
			CheckCmd    []string
			InstallHelp string
		}

		allTools := map[string]Tool{
			"kubectl": {"kubectl", "kubectl", []string{"version", "--client"}, "https://kubernetes.io/docs/tasks/tools/"},
			"helm":    {"helm", "helm", []string{"version"}, "https://helm.sh/docs/intro/install/"},
			"k3d":     {"k3d", "k3d", []string{"version"}, "https://k3d.io/v5.4.6/#installation"},
			"direnv":  {"direnv", "direnv", []string{"version"}, "https://direnv.net/docs/installation.html"},
		}

		var selectedTools []Tool

		if profile == "server" {
			// Server Profile: Minimal tools for operations (kubectl, helm)
			selectedTools = []Tool{allTools["kubectl"], allTools["helm"]}
		} else {
			// Dev Profile: Full stack (incl. k3d, direnv)
			selectedTools = []Tool{allTools["kubectl"], allTools["helm"], allTools["k3d"], allTools["direnv"]}
		}

		missing := []string{}

		for _, t := range selectedTools {
			fmt.Printf("%s Verificando %s... ", stepStyle.Render("🔍"), t.Name)
			if _, err := lookPath(t.Cmd); err != nil {
				fmt.Printf("%s\n", crossStyle.String())
				missing = append(missing, t.Name)
			} else {
				fmt.Printf("%s\n", checkStyle.String())
			}
		}

		if len(missing) == 0 {
			fmt.Println("\n" + checkStyle.Render("✨ Todas as dependências estão instaladas!"))
			if profile == "dev" {
				configureDirenv()
			}
			return nil
		}

		fmt.Println("\n" + warningStyle.Render("Algumas ferramentas estão faltando:"))
		for _, m := range missing {
			fmt.Println(itemStyle.Render("- " + m))
		}

		// Interactive Install Prompt
		install := false
		prompt := &survey.Confirm{
			Message: "Deseja tentar instalar as dependências automaticamente (via brew/apt)?",
			Default: true,
		}
		_ = survey.AskOne(prompt, &install)

		if install {
			attemptInstall(missing)
		} else {
			fmt.Println("\nPor favor, instale as ferramentas manualmente e rode 'yby setup' novamente.")
		}

		// Always try to configure direnv if present and in dev mode
		if profile == "dev" {
			if _, err := lookPath("direnv"); err == nil {
				configureDirenv()
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.Flags().String("profile", "dev", "Perfil de configuração: 'dev' (completo) ou 'server' (operações básicas)")
}

func attemptInstall(tools []string) {
	fmt.Println(headerStyle.Render("📦 Instalando Dependências..."))

	pkgManager := ""
	if _, err := lookPath("brew"); err == nil {
		pkgManager = "brew"
	} else if _, err := lookPath("apt-get"); err == nil && runtime.GOOS == "linux" {
		pkgManager = "apt"
	} else if _, err := lookPath("snap"); err == nil && runtime.GOOS == "linux" {
		pkgManager = "snap" // Fallback but apt is preferred
	}

	if pkgManager == "" {
		fmt.Println(crossStyle.Render("❌ Nenhum gerenciador de pacotes suportado encontrado (brew, apt)."))
		return
	}

	for _, tool := range tools {
		fmt.Printf("Instalando %s via %s... ", tool, pkgManager)

		var cmd = execCommand("echo", "noop") // default
		switch pkgManager {
		case "brew":
			cmd = execCommand("brew", "install", tool)
		case "apt":
			// Need sudo
			cmd = execCommand("sudo", "apt-get", "install", "-y", tool)
		case "snap":
			cmd = execCommand("sudo", "snap", "install", tool)
		}

		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("%s\n", crossStyle.String())
			fmt.Println(grayStyle.Render(string(out)))
		} else {
			fmt.Printf("%s\n", checkStyle.String())
		}
	}
}

func configureDirenv() {
	fmt.Println(headerStyle.Render("🔧 Configurando direnv..."))

	// Create .envrc if not exists
	if _, err := os.Stat(".envrc"); os.IsNotExist(err) {
		content := "export KUBECONFIG=$(pwd)/.kube/config\necho \"☸️  Ambiente configurado: KUBECONFIG=./.kube/config\""
		_ = os.WriteFile(".envrc", []byte(content), 0644)
		fmt.Println(checkStyle.Render(".envrc criado."))
	}

	_ = execCommand("direnv", "allow").Run()
	fmt.Println(checkStyle.Render("direnv allow executado."))
}
