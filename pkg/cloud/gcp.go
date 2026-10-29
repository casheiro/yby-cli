package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/casheiro/yby-cli/v5/pkg/services/shared"
)

func init() {
	RegisterProvider(func(runner shared.Runner) CloudProvider {
		return &gcpProvider{runner: runner}
	})
}

type gcpProvider struct {
	runner shared.Runner
}

func (g *gcpProvider) Name() string {
	return "gcp"
}

// IsAvailable verifica se o CLI `gcloud` está instalado sem fazer chamadas de rede.
func (g *gcpProvider) IsAvailable(_ context.Context) bool {
	_, err := g.runner.LookPath("gcloud")
	return err == nil
}

// CLIVersion retorna a versão do gcloud CLI via `gcloud --version`.
func (g *gcpProvider) CLIVersion(ctx context.Context) (string, error) {
	out, err := g.runner.RunCombinedOutput(ctx, "gcloud", "--version")
	if err != nil {
		return "", erroDoCLI("gcloud --version", out, err)
	}
	lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0]), nil
	}
	return strings.TrimSpace(string(out)), nil
}

// gcpAuthAccount representa uma conta na saída de `gcloud auth list --format json`.
type gcpAuthAccount struct {
	Account string `json:"account"`
	Status  string `json:"status"`
}

// ValidateCredentials verifica credenciais via `gcloud auth list`.
// Retorna a conta com status ACTIVE quando autenticado.
func (g *gcpProvider) ValidateCredentials(ctx context.Context) (*CredentialStatus, error) {
	out, err := saidaDoCLI(ctx, g.runner, "gcloud", "auth", "list", "--format", "json")
	if err != nil {
		return &CredentialStatus{Authenticated: false}, erroDoCLI("gcloud auth list", out, err)
	}

	var accounts []gcpAuthAccount
	if err := json.Unmarshal(out, &accounts); err != nil {
		return &CredentialStatus{Authenticated: false}, fmt.Errorf("parse auth list: %w", err)
	}

	for _, acc := range accounts {
		if strings.EqualFold(acc.Status, "ACTIVE") {
			return &CredentialStatus{
				Authenticated: true,
				Identity:      acc.Account,
				Method:        "gcloud-cli",
				ExpiresAt:     validadeGCP(ctx, g),
			}, nil
		}
	}

	return &CredentialStatus{Authenticated: false}, nil
}

// gcpCluster representa um cluster GKE na saída de `gcloud container clusters list --format json`.
type gcpCluster struct {
	Name                 string `json:"name"`
	Location             string `json:"location"`
	CurrentMasterVersion string `json:"currentMasterVersion"`
	Status               string `json:"status"`
	// SelfLink é https://container.googleapis.com/v1/projects/<projeto>/locations/<local>/clusters/<nome>.
	SelfLink string `json:"selfLink"`
}

// projetoDoSelfLink tira o projeto do selfLink do cluster; vazio se não houver.
func projetoDoSelfLink(link string) string {
	_, depois, ok := strings.Cut(link, "/projects/")
	if !ok {
		return ""
	}
	projeto, _, _ := strings.Cut(depois, "/")
	return projeto
}

// ListClusters lista clusters GKE via `gcloud container clusters list`.
// Filtra por projeto (opts.Project) e região (opts.Region) quando informados. A região é
// filtrada aqui, não com --region: o gcloud só traria os clusters regionais, e os zonais
// da região (us-central1-a em us-central1) ficariam de fora.
func (g *gcpProvider) ListClusters(ctx context.Context, opts ListOptions) ([]ClusterInfo, error) {
	args := []string{"container", "clusters", "list", "--format", "json"}
	if opts.Project != "" {
		args = append(args, "--project", opts.Project)
	}

	out, err := saidaDoCLI(ctx, g.runner, "gcloud", args...)
	if err != nil {
		return nil, erroDoCLI("gcloud container clusters list", out, err)
	}

	var gcpClusters []gcpCluster
	if err := json.Unmarshal(out, &gcpClusters); err != nil {
		return nil, fmt.Errorf("parse clusters list: %w", err)
	}

	clusters := make([]ClusterInfo, 0, len(gcpClusters))
	for _, c := range gcpClusters {
		if opts.Region != "" && c.Location != opts.Region && !strings.HasPrefix(c.Location, opts.Region+"-") {
			continue
		}
		projeto := opts.Project
		if projeto == "" {
			// sem --project, o gcloud lista o projeto padrão, que está no selfLink
			projeto = projetoDoSelfLink(c.SelfLink)
		}
		clusters = append(clusters, ClusterInfo{
			Name:      c.Name,
			Region:    c.Location,
			Provider:  "gcp",
			Version:   c.CurrentMasterVersion,
			Status:    c.Status,
			ProjectID: projeto,
		})
	}
	return clusters, nil
}

// ConfigureKubeconfig configura o kubeconfig via `gcloud container clusters get-credentials`.
func (g *gcpProvider) ConfigureKubeconfig(ctx context.Context, cluster ClusterInfo) error {
	args := []string{"container", "clusters", "get-credentials", cluster.Name}
	if cluster.Region != "" {
		// --location aceita zona e região; --region é só de cluster regional
		args = append(args, "--location", cluster.Region)
	}
	if cluster.ProjectID != "" {
		args = append(args, "--project", cluster.ProjectID)
	}
	if err := g.runner.Run(ctx, "gcloud", args...); err != nil {
		return fmt.Errorf("gcloud container clusters get-credentials: %w", err)
	}
	return nil
}

// RefreshToken renova o token de autenticação via `gcloud auth print-access-token`.
func (g *gcpProvider) RefreshToken(ctx context.Context, _ ClusterInfo) error {
	if out, err := saidaDoCLI(ctx, g.runner, "gcloud", "auth", "print-access-token"); err != nil {
		return erroDoCLI("gcloud auth print-access-token", out, err)
	}
	return nil
}
