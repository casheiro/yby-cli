/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"github.com/casheiro/yby-cli/pkg/cloud"
	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/spf13/cobra"
)

// detectarProvidersCloud e obterProviderCloud permitem trocar os providers por dublês
// nos testes dos comandos cloud e env.
var (
	detectarProvidersCloud = cloud.Detect
	obterProviderCloud     = cloud.GetProvider
)

var cloudCmd = &cobra.Command{
	Use:   "cloud",
	Short: "Gerencia integrações com provedores cloud (AWS, Azure, GCP)",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(cloudCmd)
}

// erroProviderDesconhecido é o erro de --provider com um nome fora de aws, azure e gcp.
func erroProviderDesconhecido(nome string) error {
	return errors.New(errors.ErrCodeValidation, "provider '"+nome+"' não encontrado").
		WithHint("use --provider aws, azure ou gcp")
}

// erroCLICloudAusente é o erro de um provider cujo CLI não está instalado.
func erroCLICloudAusente(nome string) error {
	return errors.New(errors.ErrCodeCloudCLIMissing, "CLI do provider '"+nome+"' não está instalado").
		WithHint("instale o CLI do provider (aws, az ou gcloud) e confira com yby doctor")
}
