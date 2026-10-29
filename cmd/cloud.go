/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"github.com/casheiro/yby-cli/v5/pkg/cloud"
	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/spf13/cobra"
)

// detectarProvidersCloud e obterProviderCloud permitem trocar os providers por dublês
// nos testes dos comandos cloud e env; auditoriaCloud, o audit log (~/.yby/audit.log).
var (
	detectarProvidersCloud = cloud.Detect
	obterProviderCloud     = cloud.GetProvider
	auditoriaCloud         = cloud.NewAuditLogger
)

var cloudCmd = &cobra.Command{
	Use:   "cloud",
	Short: "Gerencia clusters em AWS, Azure e GCP",
	Annotations: map[string]string{anotacaoBeta: "Beta: testado com os CLIs aws, az e gcloud e conferido contra a documentação oficial, " +
		"mas ainda não contra clusters reais. Login, tokens e kubeconfig podem falhar de jeitos que os testes não pegaram; " +
		"relate em github.com/casheiro/yby-cli/issues."},
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
