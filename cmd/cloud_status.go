/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	gocontext "context"
	"fmt"
	"time"

	"github.com/casheiro/yby-cli/pkg/cloud"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/spf13/cobra"
)

var cloudStatusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Exibe o status de credenciais dos provedores cloud",
	Example: `  yby cloud status`,
	RunE:    runCloudStatus,
}

func init() {
	cloudCmd.AddCommand(cloudStatusCmd)
}

func runCloudStatus(cmd *cobra.Command, args []string) error {
	runner := &shared.RealRunner{}
	providers := cloud.Detect(gocontext.Background(), runner)

	if len(providers) == 0 {
		fmt.Println(grayStyle.Render("Nenhum CLI de cloud provider detectado."))
		fmt.Println(grayStyle.Render("Instale aws-cli, az-cli ou gcloud para começar."))
		return nil
	}

	fmt.Println(titleStyle.Render("Status de Credenciais Cloud"))

	for _, p := range providers {
		fmt.Printf("\n%s\n", headerStyle.Render(fmt.Sprintf("  %s  ", p.Name())))

		// Verificar versão do CLI
		version, verErr := p.CLIVersion(gocontext.Background())
		if verErr == nil {
			fmt.Printf("  CLI: %s\n", grayStyle.Render(version))
		}

		// Validar credenciais com timeout de 10s
		ctx, cancel := gocontext.WithTimeout(gocontext.Background(), 10*time.Second)
		status, err := p.ValidateCredentials(ctx)
		cancel()

		if err != nil || !status.Authenticated {
			fmt.Printf("  %sNão autenticado\n", crossStyle.Render(""))
			if err != nil {
				fmt.Printf("  %s\n", grayStyle.Render(err.Error()))
			}
			continue
		}

		fmt.Printf("  %sAutenticado\n", checkStyle.Render(""))
		fmt.Printf("  Identidade: %s\n", status.Identity)

		if status.Method != "" {
			fmt.Printf("  Método: %s\n", status.Method)
		}

		if status.ExpiresAt != nil {
			remaining := time.Until(*status.ExpiresAt)
			expiresStr := status.ExpiresAt.Format("2006-01-02 15:04:05")

			if remaining > 0 {
				fmt.Printf("  Expira em: %s (%s restante)\n", expiresStr, formatDuration(remaining))
			} else {
				fmt.Printf("  %sToken expirado em: %s\n", warningStyle.Render(""), expiresStr)
			}
		}
	}

	return nil
}

// formatDuration formata uma duração em formato legível.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if minutes == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh%dm", hours, minutes)
}
