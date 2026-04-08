package context

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// AuthConfig contém configuração de autenticação avançada para providers cloud.
type AuthConfig struct {
	Method      string `yaml:"method,omitempty"`        // sso, assume-role, web-identity, mfa
	SSOStartURL string `yaml:"sso_start_url,omitempty"` // URL inicial do SSO (ex: https://my-sso.awsapps.com/start)
	SSORegion   string `yaml:"sso_region,omitempty"`    // Região do SSO (pode diferir da região do cluster)
	SSOAccount  string `yaml:"sso_account,omitempty"`   // ID da conta AWS para SSO
	SSORoleName string `yaml:"sso_role_name,omitempty"` // Nome do role SSO
	MFASerial   string `yaml:"mfa_serial,omitempty"`    // ARN do dispositivo MFA (ex: arn:aws:iam::123456789012:mfa/user)
}

// CloudConfig contém metadados opcionais do cloud provider para ambientes K8s gerenciados.
type CloudConfig struct {
	Provider        string      `yaml:"provider,omitempty"`
	Region          string      `yaml:"region,omitempty"`
	Cluster         string      `yaml:"cluster,omitempty"`
	Profile         string      `yaml:"profile,omitempty"`
	RoleARN         string      `yaml:"role_arn,omitempty"`
	Auth            *AuthConfig `yaml:"auth,omitempty"`
	ResourceGroup   string      `yaml:"resource_group,omitempty"`
	Subscription    string      `yaml:"subscription,omitempty"`
	TenantID        string      `yaml:"tenant_id,omitempty"`
	LoginMode       string      `yaml:"login_mode,omitempty"`
	ProjectID       string      `yaml:"project_id,omitempty"`
	Zone            string      `yaml:"zone,omitempty"`
	ServiceAccount  string      `yaml:"service_account,omitempty"`  // Email da SA para impersonation (GCP)
	CredentialsFile string      `yaml:"credentials_file,omitempty"` // Path para workload identity config (GCP)
	FleetMembership string      `yaml:"fleet_membership,omitempty"` // Nome do membership para GKE Connect Gateway
}

// Environment definition in environments.yaml
type Environment struct {
	Type        string `yaml:"type"` // local, remote, eks, aks, gke
	Description string `yaml:"description"`
	Values      string `yaml:"values"` // path to values file
	URL         string `yaml:"url,omitempty"`

	// Infra
	KubeConfig  string `yaml:"kube_config,omitempty"`
	KubeContext string `yaml:"kube_context,omitempty"`
	Namespace   string `yaml:"namespace,omitempty"`

	// Cloud
	Cloud *CloudConfig `yaml:"cloud,omitempty"`
}

// EnvironmentsManifest represents .yby/environments.yaml
type EnvironmentsManifest struct {
	Current      string                 `yaml:"current"`
	Environments map[string]Environment `yaml:"environments"`
}

// Manager handles environment context operations
type Manager struct {
	RootDir string
}

func NewManager(rootDir string) *Manager {
	return &Manager{RootDir: rootDir}
}

func (m *Manager) LoadManifest() (*EnvironmentsManifest, error) {
	path := filepath.Join(m.RootDir, ".yby", "environments.yaml")

	// Strict Check: No legacy .env fallback
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("arquivo de ambientes não encontrado (%s). Execute 'yby init' primeiro", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest EnvironmentsManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("erro lendo environments.yaml: %w", err)
	}

	return &manifest, nil
}

func (m *Manager) SaveManifest(manifest *EnvironmentsManifest) error {
	path := filepath.Join(m.RootDir, ".yby", "environments.yaml")

	data, err := yaml.Marshal(manifest)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (m *Manager) GetCurrent() (string, *Environment, error) {
	manifest, err := m.LoadManifest()
	if err != nil {
		return "", nil, err
	}

	// 1. Env Override (YBY_ENV)
	if env := os.Getenv("YBY_ENV"); env != "" {
		if val, ok := manifest.Environments[env]; ok {
			return env, &val, nil
		}
		return "", nil, fmt.Errorf("ambiente '%s' (YBY_ENV) não definido em environments.yaml", env)
	}

	// 2. Manifest Current
	currentName := manifest.Current
	if val, ok := manifest.Environments[currentName]; ok {
		return currentName, &val, nil
	}

	return "", nil, fmt.Errorf("ambiente atual '%s' inválido ou não encontrado", currentName)
}

func (m *Manager) SetCurrent(name string) error {
	manifest, err := m.LoadManifest()
	if err != nil {
		return err
	}

	if _, ok := manifest.Environments[name]; !ok {
		return fmt.Errorf("ambiente '%s' não existe", name)
	}

	manifest.Current = name
	return m.SaveManifest(manifest)
}

// AddEnvironment adiciona um novo ambiente ao manifesto.
// O parâmetro env contém os campos do ambiente (Type, Description, KubeContext, Namespace, etc.).
// O parâmetro valuesContent permite fornecer conteúdo estruturado para o arquivo de values;
// se vazio, um comentário genérico será usado como fallback.
func (m *Manager) AddEnvironment(name string, env Environment, valuesContent string) error {
	manifest, err := m.LoadManifest()
	if err != nil {
		return err
	}

	if _, exists := manifest.Environments[name]; exists {
		return fmt.Errorf("environment '%s' already exists", name)
	}

	// Cria arquivo de values se não existir
	valuesFile := fmt.Sprintf("config/values-%s.yaml", name)
	if _, err := os.Stat(filepath.Join(m.RootDir, valuesFile)); os.IsNotExist(err) {
		content := valuesContent
		if content == "" {
			content = fmt.Sprintf("# Values for %s environment\n", name)
		}
		if err := os.WriteFile(filepath.Join(m.RootDir, valuesFile), []byte(content), 0644); err != nil {
			return fmt.Errorf("falha ao criar arquivo de values: %w", err)
		}
	}

	env.Values = valuesFile
	manifest.Environments[name] = env

	return m.SaveManifest(manifest)
}

// ValidateIntegrity verifica a integridade dos ambientes configurados,
// retornando uma lista de avisos para problemas encontrados.
func (m *Manager) ValidateIntegrity() ([]string, error) {
	manifest, err := m.LoadManifest()
	if err != nil {
		return nil, err
	}

	var warnings []string
	for name, env := range manifest.Environments {
		valuesPath := filepath.Join(m.RootDir, env.Values)
		if _, err := os.Stat(valuesPath); os.IsNotExist(err) {
			warnings = append(warnings, fmt.Sprintf("ambiente '%s': arquivo de values '%s' não encontrado", name, env.Values))
		}
	}
	return warnings, nil
}
