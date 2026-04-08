/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	gocontext "context"
	"fmt"
	"strings"

	"github.com/casheiro/yby-cli/pkg/cloud"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/spf13/cobra"
)

var cloudRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Renova tokens de autenticação dos provedores cloud",
	Example: `  yby cloud refresh
  yby cloud refresh --provider aws`,
	RunE: runCloudRefresh,
}

func init() {
	cloudCmd.AddCommand(cloudRefreshCmd)

	cloudRefreshCmd.Flags().String("provider", "", "Filtrar por provider (aws, azure, gcp)")
}

func runCloudRefresh(cmd *cobra.Command, args []string) error {
	ctx := gocontext.Background()
	runner := &shared.RealRunner{}

	providerFlag, _ := cmd.Flags().GetString("provider")

	var providers []cloud.CloudProvider

	if providerFlag != "" {
		p := cloud.GetProvider(runner, strings.ToLower(providerFlag))
		if p == nil {
			return fmt.Errorf("provider '%s' não encontrado. Valores aceitos: aws, azure, gcp", providerFlag)
		}
		if !p.IsAvailable(ctx) {
			return fmt.Errorf("CLI do provider '%s' não está instalado", providerFlag)
		}
		providers = []cloud.CloudProvider{p}
	} else {
		providers = cloud.Detect(ctx, runner)
		if len(providers) == 0 {
			fmt.Println(grayStyle.Render("Nenhum CLI de cloud provider detectado."))
			return nil
		}
	}

	for _, p := range providers {
		fmt.Printf("Renovando token para %s...\n", p.Name())

		if err := p.RefreshToken(ctx, cloud.ClusterInfo{}); err != nil {
			fmt.Printf("  %sFalha: %v\n", crossStyle.Render(""), err)
			continue
		}

		fmt.Printf("  %sToken renovado com sucesso\n", checkStyle.Render(""))
	}

	return nil
}
