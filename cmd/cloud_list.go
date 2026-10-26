/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	gocontext "context"
	"fmt"
	"io"
	"strings"

	"github.com/casheiro/yby-cli/pkg/cloud"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/ui"
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

// falhaProviderCloud é um provider que não respondeu à listagem.
type falhaProviderCloud struct {
	provider string
	err      error
}

func runCloudList(cmd *cobra.Command, args []string) error {
	ctx := gocontext.Background()
	runner := &shared.RealRunner{}
	w := cmd.OutOrStdout()

	providerFlag, _ := cmd.Flags().GetString("provider")
	regionFlag, _ := cmd.Flags().GetString("region")

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
		if len(providers) == 0 {
			tituloComando(w, "cloud", "list")
			ui.Check(w, ui.CheckNeutro, "nenhum CLI de cloud detectado · instale aws, az ou gcloud", "", "")
			return nil
		}
	}

	opts := cloud.ListOptions{Region: regionFlag}
	var clusters []cloud.ClusterInfo
	var falhas []falhaProviderCloud
	for _, p := range providers {
		lista, err := p.ListClusters(ctx, opts)
		if err != nil {
			falhas = append(falhas, falhaProviderCloud{p.Name(), err})
			continue
		}
		clusters = append(clusters, lista...)
	}

	renderCloudList(w, cmd.ErrOrStderr(), clusters, falhas)
	return nil
}

// renderCloudList escreve a lista de clusters em colunas, com o total no título, e os
// providers que falharam como avisos no fim, em werr: quem filtra a lista por pipe
// recebe só a tabela.
func renderCloudList(w, werr io.Writer, clusters []cloud.ClusterInfo, falhas []falhaProviderCloud) {
	tituloComando(w, "cloud", "list · "+contagem(len(clusters), "cluster", "clusters"))

	if len(clusters) == 0 {
		ui.Check(w, ui.CheckNeutro, "nenhum cluster encontrado", "", "")
	} else {
		ouTraco := func(s string) string {
			if s == "" {
				return "-"
			}
			return s
		}
		linhas := make([]ui.Linha, len(clusters))
		for i, c := range clusters {
			linhas[i] = ui.Linha{Celulas: []string{c.Name, c.Provider, ouTraco(c.Region), ouTraco(c.Version), ouTraco(c.Status)}}
		}
		ui.Listagem(w, []ui.Coluna{
			{Titulo: "nome"},
			{Titulo: "provider", Prioridade: 2},
			{Titulo: "região", Prioridade: 3},
			{Titulo: "versão k8s", Prioridade: 4},
			{Titulo: "status", Prioridade: 1},
		}, linhas)
	}

	if len(falhas) > 0 {
		fmt.Fprintln(werr)
		linhas := make([]linhaCheck, len(falhas))
		for i, f := range falhas {
			linhas[i] = linhaCheck{ui.CheckAviso, f.provider, "falhou", mensagemDoErro(f.err)}
		}
		checksAlinhados(werr, linhas...)
	}
}
