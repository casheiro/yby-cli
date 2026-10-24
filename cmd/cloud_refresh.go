/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	gocontext "context"
	"strings"

	"github.com/casheiro/yby-cli/pkg/cloud"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/ui"
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
	w := cmd.OutOrStdout()

	providerFlag, _ := cmd.Flags().GetString("provider")

	var providers []cloud.CloudProvider

	if providerFlag != "" {
		p := obterProviderCloud(runner, strings.ToLower(providerFlag))
		if p == nil {
			return erroProviderDesconhecido(providerFlag)
		}
		if !p.IsAvailable(ctx) {
			return erroCLICloudAusente(providerFlag)
		}
		providers = []cloud.CloudProvider{p}
	} else {
		providers = detectarProvidersCloud(ctx, runner)
	}

	tituloComando(w, "cloud", "refresh")
	if len(providers) == 0 {
		ui.Check(w, ui.CheckNeutro, "nenhum CLI de cloud detectado · instale aws, az ou gcloud", "", "")
		return nil
	}

	nomes := make([]string, len(providers))
	for i, p := range providers {
		nomes[i] = p.Name()
	}
	col := ui.ColunasPara(nomes...)
	for _, p := range providers {
		if err := p.RefreshToken(ctx, cloud.ClusterInfo{}); err != nil {
			ui.CheckCom(w, col, ui.CheckErro, p.Name(), "falhou", err.Error())
			continue
		}
		ui.CheckCom(w, col, ui.CheckOk, p.Name(), "renovado", "")
	}

	return nil
}
