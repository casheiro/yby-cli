package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// EnterpriseOverrides contém configurações enterprise carregadas de .yby/overrides.yaml ou --config.
type EnterpriseOverrides struct {
	Registry      RegistryOverrides      `yaml:"registry,omitempty"`
	Cloud         CloudOverrides         `yaml:"cloud,omitempty"`
	Namespaces    NamespaceOverrides     `yaml:"namespaces,omitempty"`
	Ingress       IngressOverrides       `yaml:"ingress,omitempty"`
	TLS           TLSOverrides           `yaml:"tls,omitempty"`
	Helm          HelmOverrides          `yaml:"helm,omitempty"`
	Images        ImageOverrides         `yaml:"images,omitempty"`
	Git           GitOverrides           `yaml:"git,omitempty"`
	Profiles      ProfileOverrides       `yaml:"profiles,omitempty"`
	Observability ObservabilityOverrides `yaml:"observability,omitempty"`
}

// RegistryOverrides configura o registry de imagens.
type RegistryOverrides struct {
	URL        string `yaml:"url,omitempty"`
	PullSecret string `yaml:"pullSecret,omitempty"`
}

// CloudOverrides configura o provider cloud.
type CloudOverrides struct {
	Provider     string `yaml:"provider,omitempty"`
	StorageClass string `yaml:"storageClass,omitempty"`
}

// NamespaceOverrides configura prefixo e labels de namespaces.
type NamespaceOverrides struct {
	Prefix string            `yaml:"prefix,omitempty"`
	Labels map[string]string `yaml:"labels,omitempty"`
}

// IngressOverrides configura o ingress controller.
type IngressOverrides struct {
	ClassName string `yaml:"className,omitempty"`
}

// TLSOverrides configura TLS e certificados.
type TLSOverrides struct {
	Issuer       string `yaml:"issuer,omitempty"`
	CASecretName string `yaml:"caSecretName,omitempty"`
	ACMESolver   string `yaml:"acmeSolver,omitempty"`
}

// HelmOverrides configura repositórios e versões de charts Helm.
type HelmOverrides struct {
	RepoBaseURL string            `yaml:"repoBaseURL,omitempty"`
	Versions    map[string]string `yaml:"versions,omitempty"`
}

// ImageOverrides configura overrides explícitos de imagens.
type ImageOverrides struct {
	Overrides map[string]string `yaml:"overrides,omitempty"`
}

// GitOverrides configura o provider Git.
type GitOverrides struct {
	Provider string `yaml:"provider,omitempty"`
}

// ProfileOverrides configura profiles de recursos.
type ProfileOverrides struct {
	Resources string `yaml:"resources,omitempty"`
}

// ObservabilityOverrides configura o backend de observabilidade.
type ObservabilityOverrides struct {
	Mode string `yaml:"mode,omitempty"` // prometheus, thanos, loki
}

// ResourceProfileValues contém os valores de recursos para um profile.
type ResourceProfileValues struct {
	CPURequest    string
	CPULimit      string
	MemoryRequest string
	MemoryLimit   string
}

// DefaultOverrides retorna overrides com valores zero (backward-compat).
func DefaultOverrides() *EnterpriseOverrides {
	return &EnterpriseOverrides{}
}

// LoadOverrides tenta carregar overrides dos paths fornecidos em ordem.
// Retorna defaults vazios se nenhum arquivo existir.
func LoadOverrides(paths ...string) (*EnterpriseOverrides, error) {
	for _, path := range paths {
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("erro ao ler arquivo de overrides %s: %w", path, err)
		}

		var ov EnterpriseOverrides
		if err := yaml.Unmarshal(data, &ov); err != nil {
			return nil, fmt.Errorf("erro ao parsear overrides %s: %w", path, err)
		}
		return &ov, nil
	}
	return DefaultOverrides(), nil
}

// ResolveOverridePaths retorna os caminhos de busca de overrides na ordem de precedência.
func ResolveOverridePaths(explicitPath, projectDir string) []string {
	paths := []string{}
	if explicitPath != "" {
		paths = append(paths, explicitPath)
	}
	if projectDir != "" {
		paths = append(paths, filepath.Join(projectDir, ".yby", "overrides.yaml"))
	}
	home, err := os.UserHomeDir()
	if err == nil {
		paths = append(paths, filepath.Join(home, ".yby", "overrides.yaml"))
	}
	return paths
}

// ResolveImage retorna a imagem customizada ou a original.
// Precedência: overrides explícitos > registry prefix > original.
func (o *EnterpriseOverrides) ResolveImage(original string) string {
	if o == nil {
		return original
	}
	if o.Images.Overrides != nil {
		if custom, ok := o.Images.Overrides[original]; ok {
			return custom
		}
	}
	if o.Registry.URL != "" {
		return o.Registry.URL + "/" + original
	}
	return original
}

// ResolveNamespace aplica o prefixo de namespace se definido.
func (o *EnterpriseOverrides) ResolveNamespace(ns string) string {
	if o == nil || o.Namespaces.Prefix == "" {
		return ns
	}
	return o.Namespaces.Prefix + "-" + ns
}

// ResolveStorageClass retorna o storage class customizado ou o fallback.
func (o *EnterpriseOverrides) ResolveStorageClass(fallback string) string {
	if o == nil || o.Cloud.StorageClass == "" {
		return fallback
	}
	return o.Cloud.StorageClass
}

// ResolveIngressClass retorna o ingress class customizado ou o fallback.
func (o *EnterpriseOverrides) ResolveIngressClass(fallback string) string {
	if o == nil || o.Ingress.ClassName == "" {
		return fallback
	}
	return o.Ingress.ClassName
}

// ResolveHelmRepo substitui a base URL do repositório Helm se definido.
func (o *EnterpriseOverrides) ResolveHelmRepo(original string) string {
	if o == nil || o.Helm.RepoBaseURL == "" {
		return original
	}
	return o.Helm.RepoBaseURL
}

// ResolveChartVersion retorna a versão customizada do chart ou o fallback.
func (o *EnterpriseOverrides) ResolveChartVersion(chart, fallback string) string {
	if o == nil || o.Helm.Versions == nil {
		return fallback
	}
	if v, ok := o.Helm.Versions[chart]; ok {
		return v
	}
	return fallback
}

// ResolveTLSIssuer retorna o issuer customizado ou o fallback.
func (o *EnterpriseOverrides) ResolveTLSIssuer(fallback string) string {
	if o == nil || o.TLS.Issuer == "" {
		return fallback
	}
	return o.TLS.Issuer
}

// HasRegistryPullSecret verifica se um pull secret está configurado.
func (o *EnterpriseOverrides) HasRegistryPullSecret() bool {
	return o != nil && o.Registry.PullSecret != ""
}

// RegistryPullSecret retorna o nome do pull secret.
func (o *EnterpriseOverrides) RegistryPullSecret() string {
	if o == nil {
		return ""
	}
	return o.Registry.PullSecret
}

// NamespaceLabelsYAML retorna as labels de namespace formatadas como YAML indentado.
func (o *EnterpriseOverrides) NamespaceLabelsYAML(indent int) string {
	if o == nil || len(o.Namespaces.Labels) == 0 {
		return ""
	}
	prefix := strings.Repeat(" ", indent)
	var sb strings.Builder
	for k, v := range o.Namespaces.Labels {
		sb.WriteString(fmt.Sprintf("%s%s: \"%s\"\n", prefix, k, v))
	}
	return sb.String()
}

// ResourceProfile retorna os valores de recursos para o profile configurado.
func (o *EnterpriseOverrides) ResourceProfile() ResourceProfileValues {
	profile := "medium"
	if o != nil && o.Profiles.Resources != "" {
		profile = o.Profiles.Resources
	}
	switch profile {
	case "small":
		return ResourceProfileValues{
			CPURequest: "100m", CPULimit: "500m",
			MemoryRequest: "128Mi", MemoryLimit: "256Mi",
		}
	case "large":
		return ResourceProfileValues{
			CPURequest: "500m", CPULimit: "2000m",
			MemoryRequest: "512Mi", MemoryLimit: "2Gi",
		}
	default: // medium
		return ResourceProfileValues{
			CPURequest: "250m", CPULimit: "1000m",
			MemoryRequest: "256Mi", MemoryLimit: "512Mi",
		}
	}
}

// ResolveGitProvider retorna o provider Git customizado ou o fallback.
func (o *EnterpriseOverrides) ResolveGitProvider(fallback string) string {
	if o == nil || o.Git.Provider == "" {
		return fallback
	}
	return o.Git.Provider
}

// ResolveObservability retorna o modo de observabilidade customizado ou o fallback.
func (o *EnterpriseOverrides) ResolveObservability(fallback string) string {
	if o == nil || o.Observability.Mode == "" {
		return fallback
	}
	return o.Observability.Mode
}

// ResolveCloudProvider retorna o cloud provider configurado.
func (o *EnterpriseOverrides) ResolveCloudProvider() string {
	if o == nil {
		return ""
	}
	return o.Cloud.Provider
}
