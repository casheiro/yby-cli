/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	gocontext "context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/cloud"
	ybycontext "github.com/casheiro/yby-cli/pkg/context"
	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/scaffold"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/spf13/cobra"
)

// newEnvRunner permite substituir o runner do kubectl nos testes de env use.
var newEnvRunner = func() shared.Runner { return &shared.RealRunner{} }

var envCmd = &cobra.Command{
	Use:     "env",
	Aliases: []string{"context"},
	Short:   "Gerencia ambientes e contextos (dev, staging, prod)",
}

// env list
var envListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista os ambientes disponíveis",
	Example: `  yby env list
  # Lista ambiente, tipo e descrição; o ambiente ativo vem marcado`,
	RunE: func(cmd *cobra.Command, args []string) error {
		infraRoot, err := FindInfraRoot()
		if err != nil {
			infraRoot = "."
		}
		mgr := ybycontext.NewManager(infraRoot)
		manifest, err := mgr.LoadManifest()
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeConfig, "falha ao carregar manifesto de ambientes").
				WithHint("entre na pasta do projeto ou crie um com yby init")
		}

		renderEnvList(cmd.OutOrStdout(), manifest)
		return nil
	},
}

// renderEnvList escreve os ambientes em ordem alfabética, com ◆ no ativo.
func renderEnvList(w io.Writer, manifest *ybycontext.EnvironmentsManifest) {
	direita := "nenhum ativo"
	if manifest.Current != "" {
		direita = "ativo " + manifest.Current
	}
	tituloComando(w, "env", direita)
	if len(manifest.Environments) == 0 {
		ui.Check(w, ui.CheckNeutro, "nenhum ambiente · crie um com yby env create", "", "")
		return
	}

	nomes := make([]string, 0, len(manifest.Environments))
	for nome := range manifest.Environments {
		nomes = append(nomes, nome)
	}
	sort.Strings(nomes)
	linhas := make([]ui.Linha, len(nomes))
	for i, nome := range nomes {
		env := manifest.Environments[nome]
		marca := ""
		if nome == manifest.Current {
			marca = ui.S().Ok.Render(ui.G().Ativo)
		}
		linhas[i] = ui.Linha{Celulas: []string{marca, nome, env.Type, env.Description}}
	}
	ui.Listagem(w, []ui.Coluna{
		{Titulo: ""},
		{Titulo: "ambiente"},
		{Titulo: "tipo", Prioridade: 2},
		{Titulo: "descrição", Prioridade: 1},
	}, linhas)
}

// env use <name>
var envUseCmd = &cobra.Command{
	Use:   "use [name]",
	Short: "Define o ambiente ativo",
	Example: `  yby env use prod
  # Atualiza automaticamente o contexto do kubectl e helm`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		infraRoot, err := FindInfraRoot()
		if err != nil {
			infraRoot = "."
		}
		mgr := ybycontext.NewManager(infraRoot)

		if err := mgr.SetCurrent(name); err != nil {
			return errors.Wrap(err, errors.ErrCodeConfig, "falha ao definir ambiente ativo").
				WithHint("veja os ambientes com yby env list")
		}

		w := cmd.OutOrStdout()
		tituloComando(w, "env", "use")
		linhas := []linhaCheck{{ui.CheckOk, "ambiente", "ativo", name}}

		// Integração com kubectl context
		manifest, err := mgr.LoadManifest()
		if err == nil {
			if env, ok := manifest.Environments[name]; ok {
				if env.KubeContext != "" {
					runner := newEnvRunner()
					if err := runner.Run(gocontext.Background(), "kubectl", "config", "use-context", env.KubeContext); err != nil {
						linhas = append(linhas, linhaCheck{ui.CheckAviso, "kubectl", "sem trocar", fmt.Sprintf("contexto %s: %v", env.KubeContext, err)})
					} else {
						linhas = append(linhas, linhaCheck{ui.CheckOk, "kubectl", "contexto", env.KubeContext})
					}
				}
			}
		}
		checksAlinhados(w, linhas...)

		return nil
	},
}

// env show
var envShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Mostra detalhes do ambiente atual",
	RunE: func(cmd *cobra.Command, args []string) error {
		infraRoot, err := FindInfraRoot()
		if err != nil {
			infraRoot = "."
		}
		mgr := ybycontext.NewManager(infraRoot)
		name, env, err := mgr.GetCurrent()
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeConfig, "falha ao obter detalhes do ambiente ativo").
				WithHint("entre na pasta do projeto ou escolha um ambiente com yby env use <nome>")
		}

		// Tenta validar credenciais cloud com timeout curto
		var credenciais *cloud.CredentialStatus
		if env.Cloud != nil {
			provider := obterProviderCloud(&shared.RealRunner{}, env.Cloud.Provider)
			if provider != nil {
				ctx, cancel := gocontext.WithTimeout(gocontext.Background(), 5*time.Second)
				defer cancel()
				if status, err := provider.ValidateCredentials(ctx); err == nil {
					credenciais = status
				}
			}
		}

		renderEnvShow(cmd.OutOrStdout(), name, env, credenciais, agora())
		return nil
	},
}

// renderEnvShow escreve os dados do ambiente ativo e, quando ele é de cloud, os do
// provider e as credenciais (nil quando não deu para validar).
func renderEnvShow(w io.Writer, name string, env *ybycontext.Environment, credenciais *cloud.CredentialStatus, quando time.Time) {
	ui.Title(w, "env", "ativo "+name)
	ui.Section(w, "ambiente")
	ui.Campos(w,
		ui.Campo{Rotulo: "nome", Valor: name},
		ui.Campo{Rotulo: "tipo", Valor: env.Type},
		ui.Campo{Rotulo: "values", Valor: env.Values},
		ui.Campo{Rotulo: "url", Valor: env.URL},
		ui.Campo{Rotulo: "kube context", Valor: env.KubeContext},
		ui.Campo{Rotulo: "namespace", Valor: env.Namespace},
	)
	if env.Cloud == nil {
		return
	}

	ui.Section(w, "cloud")
	ui.Campos(w,
		ui.Campo{Rotulo: "provider", Valor: env.Cloud.Provider},
		ui.Campo{Rotulo: "região", Valor: env.Cloud.Region},
		ui.Campo{Rotulo: "cluster", Valor: env.Cloud.Cluster},
		ui.Campo{Rotulo: "profile", Valor: env.Cloud.Profile},
		ui.Campo{Rotulo: "role arn", Valor: env.Cloud.RoleARN},
	)
	if credenciais == nil {
		return
	}
	fmt.Fprintln(w)
	if !credenciais.Authenticated {
		checksAlinhados(w, linhaCheck{ui.CheckErro, "credenciais", "inválidas", cloud.DicaDeLogin(env.Cloud.Provider)})
		return
	}
	linhas := []linhaCheck{{ui.CheckOk, "credenciais", "válidas", credenciais.Identity}}
	if exp := credenciais.ExpiresAt; exp != nil {
		validade := exp.In(quando.Location()).Format("02/01/2006 15:04")
		if exp.After(quando) {
			linhas = append(linhas, linhaCheck{ui.CheckOk, "token", "válido", "até " + validade})
		} else {
			linhas = append(linhas, linhaCheck{ui.CheckAviso, "token", "expirado", "desde " + validade + " · yby cloud refresh renova"})
		}
	}
	checksAlinhados(w, linhas...)
}

// env create
var envCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Cria um novo ambiente e gera values correspondente",
	Example: `  yby env create qa --type remote --description "Quality Assurance"
  # Cria config/values-qa.yaml e adiciona entry em .yby/environments.yaml`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}

		envType, _ := cmd.Flags().GetString("type")
		description, _ := cmd.Flags().GetString("description")

		infraRoot, err := FindInfraRoot()
		if err != nil {
			infraRoot = "."
		}
		// sem projeto não há onde criar o ambiente: avisa antes de perguntar o nome
		if _, err := os.Stat(filepath.Join(infraRoot, ".yby", "environments.yaml")); err != nil {
			return errors.New(errors.ErrCodeConfig, "nenhum projeto yby nesta pasta").
				WithHint("entre na pasta do projeto ou crie um com yby init")
		}

		// Interactive prompts if needed
		if name == "" {
			name, err = prompter.Input("Nome do ambiente (ex: qa, uat)", "")
			if err != nil {
				return errors.Wrap(err, errors.ErrCodeValidation, "nome do ambiente não informado").
					WithHint("informe o nome: yby env create <nome>")
			}
			name = strings.TrimSpace(name)
			if name == "" {
				return errors.New(errors.ErrCodeValidation, "nome do ambiente vazio").
					WithHint("informe o nome: yby env create <nome>")
			}
		}
		if envType == "" {
			envType = "remote" // default
			// Could add prompt here
		}
		if description == "" {
			description = fmt.Sprintf("ambiente %s", name)
		}

		kubeContext, _ := cmd.Flags().GetString("kube-context")
		namespace, _ := cmd.Flags().GetString("namespace")

		// Flags cloud
		cloudProvider, _ := cmd.Flags().GetString("cloud-provider")
		cloudRegion, _ := cmd.Flags().GetString("cloud-region")
		cloudCluster, _ := cmd.Flags().GetString("cloud-cluster")

		w := cmd.OutOrStdout()
		var linhas []linhaCheck

		// Validação do cloud provider
		var cloudCfg *ybycontext.CloudConfig
		if cloudProvider != "" {
			validProviders := map[string]bool{"aws": true, "azure": true, "gcp": true}
			if !validProviders[cloudProvider] {
				return errors.New(errors.ErrCodeValidation,
					fmt.Sprintf("Cloud provider inválido: '%s'", cloudProvider)).
					WithHint("use --cloud-provider aws, azure ou gcp")
			}

			runner := &shared.RealRunner{}
			provider := obterProviderCloud(runner, cloudProvider)
			if provider == nil {
				return errors.New(errors.ErrCodeValidation,
					fmt.Sprintf("provider %s não está registrado", cloudProvider)).
					WithHint("use --cloud-provider aws, azure ou gcp")
			}

			// Configurar kubeconfig se cluster especificado
			if cloudCluster != "" {
				clusterInfo := cloud.ClusterInfo{
					Name:     cloudCluster,
					Region:   cloudRegion,
					Provider: cloudProvider,
				}
				ctx := gocontext.Background()
				if err := provider.ConfigureKubeconfig(ctx, clusterInfo); err != nil {
					linhas = append(linhas, linhaCheck{ui.CheckAviso, "kubeconfig", "falhou", fmt.Sprintf("cluster %s: %v", cloudCluster, err)})
				} else {
					linhas = append(linhas, linhaCheck{ui.CheckOk, "kubeconfig", "configurado", "cluster " + cloudCluster})
				}
			}

			cloudCfg = &ybycontext.CloudConfig{
				Provider: cloudProvider,
				Region:   cloudRegion,
				Cluster:  cloudCluster,
			}
		}

		// Gera values estruturados a partir do project manifest, se disponível
		var valuesContent string
		if manifest, err := scaffold.LoadProjectManifest(infraRoot); err == nil {
			bpCtx := scaffold.ManifestToContext(manifest)
			valuesContent = scaffold.RenderEnvironmentValues(bpCtx, name)
		}

		env := ybycontext.Environment{
			Type:        envType,
			Description: description,
			KubeContext: kubeContext,
			Namespace:   namespace,
			Cloud:       cloudCfg,
		}

		mgr := ybycontext.NewManager(infraRoot)
		if err := mgr.AddEnvironment(name, env, valuesContent); err != nil {
			return errors.Wrap(err, errors.ErrCodeConfig, "falha ao criar o ambiente").
				WithHint("confira a causa; yby env list mostra os ambientes que já existem")
		}

		criados := []linhaCheck{
			{ui.CheckOk, "ambiente", "criado", name + " · " + envType},
			{ui.CheckOk, "values", "criado", fmt.Sprintf("config/values-%s.yaml", name)},
		}
		tituloComando(w, "env", "create")
		checksAlinhados(w, append(criados, linhas...)...)
		if cloudCfg != nil {
			ui.Section(w, "cloud")
			ui.Campos(w,
				ui.Campo{Rotulo: "provider", Valor: cloudCfg.Provider},
				ui.Campo{Rotulo: "região", Valor: cloudCfg.Region},
				ui.Campo{Rotulo: "cluster", Valor: cloudCfg.Cluster},
			)
		}

		// Validação de integridade após criação
		warnings, err := mgr.ValidateIntegrity()
		if err == nil && len(warnings) > 0 {
			ui.Section(w, "integridade")
			renderAvisosIntegridade(w, warnings)
		}

		return nil
	},
}

// renderAvisosIntegridade escreve um aviso por problema encontrado nos ambientes.
func renderAvisosIntegridade(w io.Writer, warnings []string) {
	ordenados := append([]string(nil), warnings...)
	sort.Strings(ordenados)
	for _, aviso := range ordenados {
		ui.Check(w, ui.CheckAviso, aviso, "", "")
	}
}

// env check
var envCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Valida integridade dos ambientes configurados",
	RunE: func(cmd *cobra.Command, args []string) error {
		infraRoot, err := FindInfraRoot()
		if err != nil {
			infraRoot = "."
		}
		mgr := ybycontext.NewManager(infraRoot)

		warnings, err := mgr.ValidateIntegrity()
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeConfig, "falha ao validar integridade dos ambientes").
				WithHint("entre na pasta do projeto ou crie um com yby init")
		}

		w := cmd.OutOrStdout()
		tituloComando(w, "env", "check")
		if len(warnings) == 0 {
			ui.Check(w, ui.CheckOk, "todos os ambientes estão íntegros", "", "")
		} else {
			renderAvisosIntegridade(w, warnings)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(envCmd)
	envCmd.AddCommand(envListCmd)
	envCmd.AddCommand(envUseCmd)
	envCmd.AddCommand(envShowCmd)
	envCmd.AddCommand(envCreateCmd)
	envCmd.AddCommand(envCheckCmd)

	envCreateCmd.Flags().String("type", "", "Tipo do ambiente (local/remote)")
	envCreateCmd.Flags().String("description", "", "Descrição do ambiente")
	envCreateCmd.Flags().String("kube-context", "", "Contexto kubectl associado ao ambiente")
	envCreateCmd.Flags().String("namespace", "", "Namespace padrão do ambiente")
	envCreateCmd.Flags().String("cloud-provider", "", "Cloud provider (aws, azure, gcp)")
	envCreateCmd.Flags().String("cloud-region", "", "Região do cloud provider")
	envCreateCmd.Flags().String("cloud-cluster", "", "Nome do cluster cloud")
}
