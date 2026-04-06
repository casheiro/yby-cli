/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/services/setup"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/spf13/cobra"
)

// newSetupService permite override em testes para injetar mocks
var newSetupService = func(r shared.Runner, fs shared.Filesystem) setup.Service {
	checker := &setup.SystemToolChecker{Runner: r}
	pkg := &setup.SystemPackageManager{Runner: r}
	return setup.NewService(checker, pkg, r, fs)
}

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

		// 0. Validar perfil
		profile, _ := cmd.Flags().GetString("profile")
		if profile != "dev" && profile != "server" {
			return errors.New(errors.ErrCodeValidation, "Perfil inválido. Use 'dev' ou 'server'")
		}
		fmt.Printf("🔧 Perfil selecionado: %s\n", profile)

		// 1. Criar serviço
		runner := &shared.RealRunner{}
		fsys := &shared.RealFilesystem{}
		svc := newSetupService(runner, fsys)

		// 2. Verificar ferramentas
		result, err := svc.CheckTools(profile)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeValidation, "falha ao verificar ferramentas")
		}

		for _, ts := range result.Tools {
			fmt.Printf("%s Verificando %s... ", stepStyle.Render("🔍"), ts.Name)
			if ts.Installed {
				fmt.Printf("%s\n", checkStyle.String())
			} else {
				fmt.Printf("%s\n", crossStyle.String())
			}
		}

		if len(result.Missing) == 0 {
			fmt.Println("\n" + checkStyle.Render("✨ Todas as dependências estão instaladas!"))
			if profile == "dev" {
				if direnvErr := svc.ConfigureDirenv("."); direnvErr != nil {
					slog.Warn("falha ao configurar direnv", "erro", direnvErr)
				}
			}
			return nil
		}

		// 3. Exibir ferramentas faltantes
		fmt.Println("\n" + warningStyle.Render("Algumas ferramentas estão faltando:"))
		for _, m := range result.Missing {
			fmt.Println(itemStyle.Render("- " + m))
		}

		// 4. Prompt interativo para instalação
		install, _ := prompter.Confirm("Deseja tentar instalar as dependências automaticamente (via brew/apt)?", true)

		if install {
			fmt.Println(headerStyle.Render("📦 Instalando Dependências..."))
			ctx := context.Background()
			installResults := svc.InstallMissing(ctx, result.Missing)
			for _, ir := range installResults {
				fmt.Printf("Instalando %s... ", ir.Tool)
				if ir.Success {
					fmt.Printf("%s\n", checkStyle.String())
				} else {
					fmt.Printf("%s\n", crossStyle.String())
					fmt.Println(grayStyle.Render(ir.Output))
				}
			}
		} else {
			fmt.Println("\nPor favor, instale as ferramentas manualmente e rode 'yby setup' novamente.")
		}

		// 5. Configurar direnv se perfil dev e direnv disponível
		if profile == "dev" {
			// Verifica se direnv está disponível após possível instalação
			checker := &setup.SystemToolChecker{Runner: runner}
			if _, checkErr := checker.IsInstalled("direnv"); checkErr == nil {
				if direnvErr := svc.ConfigureDirenv("."); direnvErr != nil {
					slog.Warn("falha ao configurar direnv", "erro", direnvErr)
				}
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.Flags().String("profile", "dev", "Perfil de configuração: 'dev' (completo) ou 'server' (operações básicas)")
}
