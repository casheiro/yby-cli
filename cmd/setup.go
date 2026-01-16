/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/AlecAivazis/survey/v2"
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
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(titleStyle.Render("🚀 Yby Setup - Configuração de Ambiente"))
		fmt.Println("---------------------------------------")

		// 0. Detect Profile
		profile, _ := cmd.Flags().GetString("profile")
		if profile != "dev" && profile != "server" {
			fmt.Println(crossStyle.Render("❌ Perfil inválido. Use 'dev' ou 'server'."))
			os.Exit(1)
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
			if _, err := exec.LookPath(t.Cmd); err != nil {
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
			return
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
			if _, err := exec.LookPath("direnv"); err == nil {
				configureDirenv()
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.Flags().String("profile", "dev", "Perfil de configuração: 'dev' (completo) ou 'server' (operações básicas)")
}

func attemptInstall(tools []string) {
	fmt.Println(headerStyle.Render("📦 Instalando Dependências..."))

	pkgManager := ""
	if _, err := exec.LookPath("brew"); err == nil {
		pkgManager = "brew"
	} else if _, err := exec.LookPath("apt-get"); err == nil && runtime.GOOS == "linux" {
		pkgManager = "apt"
	} else if _, err := exec.LookPath("snap"); err == nil && runtime.GOOS == "linux" {
		pkgManager = "snap" // Fallback but apt is preferred
	}

	if pkgManager == "" {
		fmt.Println(crossStyle.Render("❌ Nenhum gerenciador de pacotes suportado encontrado (brew, apt)."))
		return
	}

	for _, tool := range tools {
		fmt.Printf("Instalando %s via %s... ", tool, pkgManager)
		var cmd *exec.Cmd

		switch pkgManager {
		case "brew":
			cmd = exec.Command("brew", "install", tool)
		case "apt":
			// Need sudo
			cmd = exec.Command("sudo", "apt-get", "install", "-y", tool)
		case "snap":
			cmd = exec.Command("sudo", "snap", "install", tool)
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

	_ = exec.Command("direnv", "allow").Run()
	fmt.Println(checkStyle.Render("direnv allow executado."))
}
