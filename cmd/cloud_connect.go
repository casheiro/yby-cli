/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	gocontext "context"
	"fmt"
	"strings"

	"github.com/casheiro/yby-cli/pkg/cloud"
	ybycontext "github.com/casheiro/yby-cli/pkg/context"
	"github.com/casheiro/yby-cli/pkg/services/shared"
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

	if providerFlag != "" {
		// Modo não-interativo: validar provider
		validProviders := map[string]bool{"aws": true, "azure": true, "gcp": true}
		if !validProviders[strings.ToLower(providerFlag)] {
			return fmt.Errorf("provider inválido: '%s'. Valores aceitos: aws, azure, gcp", providerFlag)
		}
		p := cloud.GetProvider(runner, strings.ToLower(providerFlag))
		if p == nil {
			return fmt.Errorf("provider '%s' não está registrado", providerFlag)
		}
		if !p.IsAvailable(ctx) {
			return fmt.Errorf("CLI do provider '%s' não está instalado", providerFlag)
		}
		provider = p
	} else {
		// Modo interativo: detectar providers
		providers := cloud.Detect(ctx, runner)
		if len(providers) == 0 {
			return fmt.Errorf("nenhum CLI de cloud provider detectado. Instale aws-cli, az-cli ou gcloud")
		}

		if len(providers) == 1 {
			provider = providers[0]
			fmt.Printf("%sProvider detectado: %s\n", checkStyle.Render(""), provider.Name())
		} else {
			names := make([]string, len(providers))
			for i, p := range providers {
				names[i] = p.Name()
			}
			selected, err := prompter.Select("Selecione o cloud provider:", names, "")
			if err != nil {
				return fmt.Errorf("seleção cancelada: %w", err)
			}
			for _, p := range providers {
				if p.Name() == selected {
					provider = p
					break
				}
			}
		}
	}

	// Validar credenciais
	fmt.Println("Verificando credenciais...")
	status, err := provider.ValidateCredentials(ctx)
	if err != nil || !status.Authenticated {
		return fmt.Errorf("credenciais inválidas para %s. Execute o login do provider antes de conectar", provider.Name())
	}
	fmt.Printf("%sAutenticado como: %s\n", checkStyle.Render(""), status.Identity)

	// Listar clusters
	opts := cloud.ListOptions{Region: regionFlag}
	clusters, err := provider.ListClusters(ctx, opts)
	if err != nil {
		return fmt.Errorf("falha ao listar clusters: %w", err)
	}
	if len(clusters) == 0 {
		return fmt.Errorf("nenhum cluster encontrado para %s na região '%s'", provider.Name(), regionFlag)
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
			return fmt.Errorf("cluster '%s' não encontrado. Clusters disponíveis: %s",
				clusterFlag, clusterNames(clusters))
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
			return fmt.Errorf("seleção cancelada: %w", err)
		}
		for i, n := range names {
			if n == selected {
				selectedCluster = clusters[i]
				break
			}
		}
	}

	// Configurar kubeconfig
	fmt.Printf("Configurando kubeconfig para cluster '%s'...\n", selectedCluster.Name)
	if err := provider.ConfigureKubeconfig(ctx, selectedCluster); err != nil {
		return fmt.Errorf("falha ao configurar kubeconfig: %w", err)
	}
	fmt.Printf("%sKubeconfig configurado com sucesso!\n", checkStyle.Render(""))

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
			fmt.Printf("%sFalha ao criar ambiente: %v\n", warningStyle.Render(""), err)
		} else {
			fmt.Printf("%sAmbiente '%s' criado com sucesso!\n", checkStyle.Render(""), envName)
		}
	}

	return nil
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
