/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	gocontext "context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/casheiro/yby-cli/pkg/cloud"
	ybycontext "github.com/casheiro/yby-cli/pkg/context"
	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/spf13/cobra"
)

var cloudConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Conecta a um cluster K8s em um provedor cloud",
	Long: `Configura o kubeconfig local para acessar um cluster K8s gerenciado.

Modo interativo (sem flags): detecta provedores, lista clusters e guia a configuração.
Modo não-interativo: forneça --provider, --region e --cluster.`,
	Example: `  # Modo interativo (guiado)
  yby cloud connect

  # Modo não-interativo
  yby cloud connect --provider aws --region us-east-1 --cluster meu-cluster

  # Com criação de ambiente
  yby cloud connect --provider gcp --region us-central1 --cluster prod --env-name prod`,
	RunE: runCloudConnect,
}

func init() {
	cloudCmd.AddCommand(cloudConnectCmd)

	cloudConnectCmd.Flags().String("provider", "", "Cloud provider (aws, azure, gcp)")
	cloudConnectCmd.Flags().String("region", "", "Região do cluster")
	cloudConnectCmd.Flags().String("cluster", "", "Nome do cluster")
	cloudConnectCmd.Flags().String("env-name", "", "Nome do ambiente a criar")
}

func runCloudConnect(cmd *cobra.Command, args []string) error {
	ctx := gocontext.Background()
	runner := &shared.RealRunner{}

	providerFlag, _ := cmd.Flags().GetString("provider")
	regionFlag, _ := cmd.Flags().GetString("region")
	clusterFlag, _ := cmd.Flags().GetString("cluster")
	envNameFlag, _ := cmd.Flags().GetString("env-name")

	var provider cloud.CloudProvider
	w := cmd.OutOrStdout()

	if providerFlag != "" {
		// Modo não-interativo: validar provider
		validProviders := map[string]bool{"aws": true, "azure": true, "gcp": true}
		if !validProviders[strings.ToLower(providerFlag)] {
			return erroProviderDesconhecido(providerFlag)
		}
		p := obterProviderCloud(runner, strings.ToLower(providerFlag))
		if p == nil {
			return errors.New(errors.ErrCodeValidation, "provider '"+providerFlag+"' não está registrado").
				WithHint("o binário yby-cloud traz os providers com SDK; confira com yby doctor")
		}
		if !p.IsAvailable(ctx) {
			return erroCLICloudAusente(providerFlag)
		}
		provider = p
		tituloComando(w, "cloud", "connect · "+provider.Name())
	} else {
		// Modo interativo: detectar providers
		providers := detectarProvidersCloud(ctx, runner)
		if len(providers) == 0 {
			return errors.New(errors.ErrCodeCloudCLIMissing, "nenhum CLI de cloud provider detectado").
				WithHint("instale aws, az ou gcloud e confira com yby doctor")
		}

		if len(providers) == 1 {
			provider = providers[0]
			tituloComando(w, "cloud", "connect · "+provider.Name())
			linhaConnect(w, linhaCheck{ui.CheckOk, "provider", "detectado", provider.Name()})
		} else {
			names := make([]string, len(providers))
			for i, p := range providers {
				names[i] = p.Name()
			}
			selected, err := prompter.Select("Selecione o cloud provider:", names, "")
			if err != nil {
				return errors.Wrap(err, errors.ErrCodeValidation, "seleção do provider cancelada").
					WithHint("informe o provider: yby cloud connect --provider aws|azure|gcp")
			}
			for _, p := range providers {
				if p.Name() == selected {
					provider = p
					break
				}
			}
			tituloComando(w, "cloud", "connect · "+provider.Name())
		}
	}

	// Validar credenciais
	linhaEmAndamento(w, "verificando as credenciais")
	status, err := provider.ValidateCredentials(ctx)
	registrarAutenticacao(provider.Name(), status, err)
	if err != nil || !status.Authenticated {
		e := errors.New(errors.ErrCodeCloudTokenExpired, "credenciais inválidas para "+provider.Name())
		if err != nil {
			e = errors.Wrap(err, errors.ErrCodeCloudTokenExpired, "credenciais inválidas para "+provider.Name())
		}
		return e.WithHint(cloud.DicaDeLogin(provider.Name()) + " e depois yby cloud connect de novo")
	}
	linhaConnect(w, linhaCheck{ui.CheckOk, "credenciais", "válidas", status.Identity})

	// Listar clusters
	opts := cloud.ListOptions{Region: regionFlag}
	clusters, err := provider.ListClusters(ctx, opts)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeExec, "falha ao listar os clusters").
			WithHint("confira a região e as permissões da conta no CLI do provider")
	}
	if len(clusters) == 0 {
		return errors.New(errors.ErrCodeValidation, fmt.Sprintf("nenhum cluster encontrado para %s na região '%s'", provider.Name(), regionFlag)).
			WithHint("confira a região com --region ou liste com yby cloud list")
	}

	var selectedCluster cloud.ClusterInfo

	if clusterFlag != "" {
		// Modo não-interativo: buscar cluster pelo nome
		found := false
		for _, c := range clusters {
			if c.Name == clusterFlag {
				selectedCluster = c
				found = true
				break
			}
		}
		if !found {
			return errors.New(errors.ErrCodeValidation, fmt.Sprintf("cluster '%s' não encontrado", clusterFlag)).
				WithHint("clusters disponíveis: " + clusterNames(clusters))
		}
	} else {
		// Modo interativo: selecionar cluster
		names := make([]string, len(clusters))
		for i, c := range clusters {
			label := c.Name
			if c.Region != "" {
				label += fmt.Sprintf(" (%s)", c.Region)
			}
			if c.Version != "" {
				label += fmt.Sprintf(" [k8s %s]", c.Version)
			}
			names[i] = label
		}

		selected, err := prompter.Select("Selecione o cluster:", names, "")
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeValidation, "seleção do cluster cancelada").
				WithHint("informe o cluster: yby cloud connect --provider P --region R --cluster C")
		}
		for i, n := range names {
			if n == selected {
				selectedCluster = clusters[i]
				break
			}
		}
	}

	// Configurar kubeconfig
	linhaEmAndamento(w, "configurando o kubeconfig do cluster "+selectedCluster.Name)
	if err := provider.ConfigureKubeconfig(ctx, selectedCluster); err != nil {
		return errors.Wrap(err, errors.ErrCodeExec, "falha ao configurar o kubeconfig").
			WithHint("confira se o cluster existe na região e se ~/.kube/config pode ser gravado")
	}
	linhaConnect(w, linhaCheck{ui.CheckOk, "kubeconfig", "configurado", selectedCluster.Name})

	// Oferecer criação de ambiente
	envName := envNameFlag
	if envName == "" {
		createEnv, err := prompter.Confirm("Deseja criar um ambiente no Yby para este cluster?", true)
		if err == nil && createEnv {
			envName, err = prompter.Input("Nome do ambiente:", selectedCluster.Name)
			if err != nil {
				return nil // Usuário cancelou, mas kubeconfig já foi configurado
			}
		}
	}

	if envName != "" {
		if err := createCloudEnvironment(envName, selectedCluster, provider.Name()); err != nil {
			linhaConnect(w, linhaCheck{ui.CheckAviso, "ambiente", "não criado", err.Error()})
		} else {
			linhaConnect(w, linhaCheck{ui.CheckOk, "ambiente", "criado", envName})
		}
	}

	return nil
}

// linhaConnect escreve uma linha do connect com as colunas iguais em todas as etapas,
// que saem uma a uma, intercaladas com as linhas em andamento.
func linhaConnect(w io.Writer, l linhaCheck) {
	col := ui.ColunasPara("credenciais")
	col.Versao = len("configurado") + 2
	ui.CheckCom(w, col, l.tipo, l.nome, l.estado, l.nota)
}

// clusterNames retorna os nomes dos clusters separados por vírgula.
func clusterNames(clusters []cloud.ClusterInfo) string {
	names := make([]string, len(clusters))
	for i, c := range clusters {
		names[i] = c.Name
	}
	return strings.Join(names, ", ")
}

// createCloudEnvironment cria um ambiente no Yby para o cluster cloud conectado.
func createCloudEnvironment(name string, cluster cloud.ClusterInfo, providerName string) error {
	infraRoot, err := FindInfraRoot()
	if err != nil {
		infraRoot = "."
	}

	env := ybycontext.Environment{
		Type:        "remote",
		Description: fmt.Sprintf("Cluster %s (%s/%s)", cluster.Name, providerName, cluster.Region),
		Cloud: &ybycontext.CloudConfig{
			Provider: providerName,
			Region:   cluster.Region,
			Cluster:  cluster.Name,
		},
	}

	mgr := ybycontext.NewManager(infraRoot)
	return mgr.AddEnvironment(name, env, "")
}

// registrarAutenticacao grava no audit log a verificação de credenciais do connect.
func registrarAutenticacao(provider string, status *cloud.CredentialStatus, err error) {
	identidade, metodo := "", ""
	if status != nil {
		identidade, metodo = status.Identity, status.Method
	}
	ok := err == nil && status != nil && status.Authenticated
	if errLog := auditoriaCloud().LogAuthentication(provider, identidade, metodo, ok, err); errLog != nil {
		slog.Debug("cloud.connect.auditoria", "erro", errLog)
	}
}
