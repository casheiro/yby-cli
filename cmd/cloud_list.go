/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	gocontext "context"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/casheiro/yby-cli/pkg/cloud"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/spf13/cobra"
)

var cloudListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista clusters K8s disponíveis nos provedores cloud",
	Example: `  yby cloud list
  yby cloud list --provider aws
  yby cloud list --provider gcp --region us-central1`,
	RunE: runCloudList,
}

func init() {
	cloudCmd.AddCommand(cloudListCmd)

	cloudListCmd.Flags().String("provider", "", "Filtrar por provider (aws, azure, gcp)")
	cloudListCmd.Flags().String("region", "", "Filtrar por região")
}

func runCloudList(cmd *cobra.Command, args []string) error {
	ctx := gocontext.Background()
	runner := &shared.RealRunner{}

	providerFlag, _ := cmd.Flags().GetString("provider")
	regionFlag, _ := cmd.Flags().GetString("region")

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
			fmt.Println(grayStyle.Render("Instale aws-cli, az-cli ou gcloud para começar."))
			return nil
		}
	}

	opts := cloud.ListOptions{Region: regionFlag}
	totalClusters := 0

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NOME\tPROVIDER\tREGIÃO\tVERSÃO K8S\tSTATUS")
	fmt.Fprintln(w, "----\t--------\t------\t----------\t------")

	for _, p := range providers {
		clusters, err := p.ListClusters(ctx, opts)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "%sFalha ao listar clusters de %s: %v\n",
				warningStyle.Render(""), p.Name(), err)
			continue
		}

		for _, c := range clusters {
			version := c.Version
			if version == "" {
				version = "-"
			}
			status := c.Status
			if status == "" {
				status = "-"
			}
			region := c.Region
			if region == "" {
				region = "-"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				c.Name, c.Provider, region, version, status)
			totalClusters++
		}
	}

	w.Flush()

	if totalClusters == 0 {
		fmt.Println(grayStyle.Render("\nNenhum cluster encontrado."))
	} else {
		fmt.Printf("\nTotal: %d cluster(s)\n", totalClusters)
	}

	return nil
}
