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
	"github.com/casheiro/yby-cli/pkg/ui"
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
		w := cmd.OutOrStdout()

		// 0. Validar perfil
		profile, _ := cmd.Flags().GetString("profile")
		if profile != "dev" && profile != "server" {
			return errors.New(errors.ErrCodeValidation, "Perfil inválido. Use 'dev' ou 'server'")
		}
		ui.Title(w, "setup", "perfil "+profile)

		// 1. Criar serviço
		runner := &shared.RealRunner{}
		fsys := &shared.RealFilesystem{}
		svc := newSetupService(runner, fsys)

		// 2. Verificar ferramentas
		result, err := svc.CheckTools(profile)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeValidation, "falha ao verificar ferramentas")
		}

		ferramentas := secaoCheck{nome: "ferramentas"}
		for _, ts := range result.Tools {
			if ts.Installed {
				ferramentas.linhas = append(ferramentas.linhas, linhaCheck{ui.CheckOk, ts.Name, "instalado", ts.Path})
			} else {
				ferramentas.linhas = append(ferramentas.linhas, linhaCheck{ui.CheckErro, ts.Name, "ausente", ts.InstallHelp})
			}
		}
		renderChecks(w, []secaoCheck{ferramentas}, "faltam ferramentas")

		if len(result.Missing) == 0 {
			if profile == "dev" {
				if direnvErr := svc.ConfigureDirenv("."); direnvErr != nil {
					slog.Warn("falha ao configurar direnv", "erro", direnvErr)
				}
			}
			return nil
		}

		// 3. Prompt interativo para instalação das ferramentas faltantes, só quando há um
		// gerenciador de pacotes suportado (brew, apt, snap; no Windows, nenhum)
		install := false
		if result.Gerenciador != "" {
			install, _ = prompter.Confirm("Instalar as ferramentas que faltam com o "+result.Gerenciador+"?", true)
		}

		if install {
			ui.Section(w, "instalação")
			ctx := context.Background()
			installResults := svc.InstallMissing(ctx, result.Missing)
			for _, ir := range installResults {
				if ir.Success {
					ui.Check(w, ui.CheckOk, ir.Tool, "instalado", "")
				} else {
					ui.Check(w, ui.CheckErro, ir.Tool, "falhou", "")
					saidaRecuada(w, ir.Output, true)
				}
			}
		} else {
			fmt.Fprintln(w)
			if result.Gerenciador == "" {
				ui.Check(w, ui.CheckNeutro, "nenhum gerenciador de pacotes suportado (brew, apt, snap)", "", "")
			}
			ui.Check(w, ui.CheckNeutro, "instale pelos links acima e rode yby setup de novo", "", "")
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
