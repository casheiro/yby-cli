package scaffold

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOverrides_FileNotFound(t *testing.T) {
	ov, err := LoadOverrides("/tmp/nao-existe-xyz.yaml")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if ov == nil {
		t.Fatal("esperava overrides default, obteve nil")
	}
	if ov.Registry.URL != "" {
		t.Errorf("esperava registry.url vazio, obteve %q", ov.Registry.URL)
	}
}

func TestLoadOverrides_EmptyPaths(t *testing.T) {
	ov, err := LoadOverrides()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if ov == nil {
		t.Fatal("esperava overrides default, obteve nil")
	}
}

func TestLoadOverrides_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overrides.yaml")
	content := `
registry:
  url: my-registry.corp
  pullSecret: regcred
cloud:
  provider: aws
  storageClass: gp3
namespaces:
  prefix: fintech
  labels:
    cost-center: platform
    team: devops
ingress:
  className: nginx
tls:
  issuer: custom
  caSecretName: corp-ca
  acmeSolver: dns01
helm:
  repoBaseURL: https://charts.internal.corp
  versions:
    argocd: "6.7.0"
    keda: "2.16.0"
images:
  overrides:
    "ubuntu:22.04": "registry.corp/ubuntu:22.04"
git:
  provider: gitlab
profiles:
  resources: large
`
	os.WriteFile(path, []byte(content), 0644)

	ov, err := LoadOverrides(path)
	if err != nil {
		t.Fatalf("erro ao carregar overrides: %v", err)
	}

	if ov.Registry.URL != "my-registry.corp" {
		t.Errorf("registry.url: esperava 'my-registry.corp', obteve %q", ov.Registry.URL)
	}
	if ov.Cloud.StorageClass != "gp3" {
		t.Errorf("cloud.storageClass: esperava 'gp3', obteve %q", ov.Cloud.StorageClass)
	}
	if ov.Namespaces.Prefix != "fintech" {
		t.Errorf("namespaces.prefix: esperava 'fintech', obteve %q", ov.Namespaces.Prefix)
	}
	if ov.Namespaces.Labels["cost-center"] != "platform" {
		t.Errorf("namespaces.labels.cost-center: esperava 'platform', obteve %q", ov.Namespaces.Labels["cost-center"])
	}
	if ov.Ingress.ClassName != "nginx" {
		t.Errorf("ingress.className: esperava 'nginx', obteve %q", ov.Ingress.ClassName)
	}
	if ov.TLS.Issuer != "custom" {
		t.Errorf("tls.issuer: esperava 'custom', obteve %q", ov.TLS.Issuer)
	}
	if ov.TLS.ACMESolver != "dns01" {
		t.Errorf("tls.acmeSolver: esperava 'dns01', obteve %q", ov.TLS.ACMESolver)
	}
	if ov.Helm.Versions["argocd"] != "6.7.0" {
		t.Errorf("helm.versions.argocd: esperava '6.7.0', obteve %q", ov.Helm.Versions["argocd"])
	}
	if ov.Images.Overrides["ubuntu:22.04"] != "registry.corp/ubuntu:22.04" {
		t.Errorf("images.overrides: esperava override para ubuntu:22.04")
	}
	if ov.Git.Provider != "gitlab" {
		t.Errorf("git.provider: esperava 'gitlab', obteve %q", ov.Git.Provider)
	}
	if ov.Profiles.Resources != "large" {
		t.Errorf("profiles.resources: esperava 'large', obteve %q", ov.Profiles.Resources)
	}
}

func TestLoadOverrides_Precedence(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.yaml")
	second := filepath.Join(dir, "second.yaml")

	os.WriteFile(first, []byte("registry:\n  url: first-registry"), 0644)
	os.WriteFile(second, []byte("registry:\n  url: second-registry"), 0644)

	ov, err := LoadOverrides(first, second)
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if ov.Registry.URL != "first-registry" {
		t.Errorf("esperava primeiro arquivo ter precedência, obteve %q", ov.Registry.URL)
	}
}

func TestLoadOverrides_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	os.WriteFile(path, []byte("{{invalid yaml"), 0644)

	_, err := LoadOverrides(path)
	if err == nil {
		t.Fatal("esperava erro para YAML inválido")
	}
}

func TestResolveImage_NoOverrides(t *testing.T) {
	ov := DefaultOverrides()
	img := ov.ResolveImage("ubuntu:22.04")
	if img != "ubuntu:22.04" {
		t.Errorf("esperava imagem original, obteve %q", img)
	}
}

func TestResolveImage_WithRegistry(t *testing.T) {
	ov := &EnterpriseOverrides{
		Registry: RegistryOverrides{URL: "registry.corp"},
	}
	img := ov.ResolveImage("ubuntu:22.04")
	if img != "registry.corp/ubuntu:22.04" {
		t.Errorf("esperava 'registry.corp/ubuntu:22.04', obteve %q", img)
	}
}

func TestResolveImage_ExplicitOverride(t *testing.T) {
	ov := &EnterpriseOverrides{
		Registry: RegistryOverrides{URL: "registry.corp"},
		Images: ImageOverrides{
			Overrides: map[string]string{
				"ubuntu:22.04": "custom-registry/my-ubuntu:22.04-hardened",
			},
		},
	}
	img := ov.ResolveImage("ubuntu:22.04")
	if img != "custom-registry/my-ubuntu:22.04-hardened" {
		t.Errorf("override explícito deveria ter precedência, obteve %q", img)
	}
}

func TestResolveImage_Nil(t *testing.T) {
	var ov *EnterpriseOverrides
	img := ov.ResolveImage("ubuntu:22.04")
	if img != "ubuntu:22.04" {
		t.Errorf("nil receiver deveria retornar original, obteve %q", img)
	}
}

func TestResolveNamespace_WithPrefix(t *testing.T) {
	ov := &EnterpriseOverrides{
		Namespaces: NamespaceOverrides{Prefix: "fintech"},
	}
	ns := ov.ResolveNamespace("argocd")
	if ns != "fintech-argocd" {
		t.Errorf("esperava 'fintech-argocd', obteve %q", ns)
	}
}

func TestResolveNamespace_NoPrefix(t *testing.T) {
	ov := DefaultOverrides()
	ns := ov.ResolveNamespace("argocd")
	if ns != "argocd" {
		t.Errorf("esperava 'argocd', obteve %q", ns)
	}
}

func TestResolveStorageClass(t *testing.T) {
	ov := &EnterpriseOverrides{Cloud: CloudOverrides{StorageClass: "gp3"}}
	sc := ov.ResolveStorageClass("local-path")
	if sc != "gp3" {
		t.Errorf("esperava 'gp3', obteve %q", sc)
	}

	ov2 := DefaultOverrides()
	sc2 := ov2.ResolveStorageClass("local-path")
	if sc2 != "local-path" {
		t.Errorf("esperava fallback 'local-path', obteve %q", sc2)
	}
}

func TestResolveIngressClass(t *testing.T) {
	ov := &EnterpriseOverrides{Ingress: IngressOverrides{ClassName: "nginx"}}
	ic := ov.ResolveIngressClass("traefik")
	if ic != "nginx" {
		t.Errorf("esperava 'nginx', obteve %q", ic)
	}
}

func TestResolveHelmRepo(t *testing.T) {
	ov := &EnterpriseOverrides{Helm: HelmOverrides{RepoBaseURL: "https://charts.corp"}}
	repo := ov.ResolveHelmRepo("https://argoproj.github.io/argo-helm")
	if repo != "https://charts.corp" {
		t.Errorf("esperava mirror, obteve %q", repo)
	}
}

func TestResolveChartVersion(t *testing.T) {
	ov := &EnterpriseOverrides{
		Helm: HelmOverrides{
			Versions: map[string]string{"argocd": "6.7.0"},
		},
	}
	v := ov.ResolveChartVersion("argocd", "5.51.6")
	if v != "6.7.0" {
		t.Errorf("esperava '6.7.0', obteve %q", v)
	}

	v2 := ov.ResolveChartVersion("keda", "2.14.2")
	if v2 != "2.14.2" {
		t.Errorf("esperava fallback '2.14.2', obteve %q", v2)
	}
}

func TestResourceProfile(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		wantMem string
	}{
		{"small", "small", "128Mi"},
		{"medium", "medium", "256Mi"},
		{"large", "large", "512Mi"},
		{"default", "", "256Mi"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ov := &EnterpriseOverrides{Profiles: ProfileOverrides{Resources: tt.profile}}
			rp := ov.ResourceProfile()
			if rp.MemoryRequest != tt.wantMem {
				t.Errorf("profile %q: esperava memoryRequest %q, obteve %q", tt.profile, tt.wantMem, rp.MemoryRequest)
			}
		})
	}
}

func TestNamespaceLabelsYAML(t *testing.T) {
	ov := &EnterpriseOverrides{
		Namespaces: NamespaceOverrides{
			Labels: map[string]string{
				"team": "devops",
			},
		},
	}
	yaml := ov.NamespaceLabelsYAML(4)
	if yaml == "" {
		t.Error("esperava labels YAML, obteve vazio")
	}
	if !contains(yaml, "team") {
		t.Errorf("esperava conter 'team', obteve %q", yaml)
	}
}

func TestNamespaceLabelsYAML_Empty(t *testing.T) {
	ov := DefaultOverrides()
	yaml := ov.NamespaceLabelsYAML(4)
	if yaml != "" {
		t.Errorf("esperava vazio, obteve %q", yaml)
	}
}

func TestResolveOverridePaths(t *testing.T) {
	paths := ResolveOverridePaths("/explicit/path.yaml", "/project")
	if len(paths) < 2 {
		t.Fatalf("esperava pelo menos 2 paths, obteve %d", len(paths))
	}
	if paths[0] != "/explicit/path.yaml" {
		t.Errorf("primeiro path deveria ser o explícito, obteve %q", paths[0])
	}
	if paths[1] != "/project/.yby/overrides.yaml" {
		t.Errorf("segundo path deveria ser do projeto, obteve %q", paths[1])
	}
}

func TestResolveGitProvider(t *testing.T) {
	tests := []struct {
		name     string
		ov       *EnterpriseOverrides
		fallback string
		want     string
	}{
		{"nil receiver", nil, "github", "github"},
		{"vazio usa fallback", DefaultOverrides(), "github", "github"},
		{"gitlab override", &EnterpriseOverrides{Git: GitOverrides{Provider: "gitlab"}}, "github", "gitlab"},
		{"bitbucket override", &EnterpriseOverrides{Git: GitOverrides{Provider: "bitbucket"}}, "github", "bitbucket"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ov.ResolveGitProvider(tt.fallback)
			if got != tt.want {
				t.Errorf("ResolveGitProvider(%q) = %q, esperava %q", tt.fallback, got, tt.want)
			}
		})
	}
}

func TestResolveObservability(t *testing.T) {
	tests := []struct {
		name     string
		ov       *EnterpriseOverrides
		fallback string
		want     string
	}{
		{"nil receiver", nil, "prometheus", "prometheus"},
		{"vazio usa fallback", DefaultOverrides(), "prometheus", "prometheus"},
		{"thanos override", &EnterpriseOverrides{Observability: ObservabilityOverrides{Mode: "thanos"}}, "prometheus", "thanos"},
		{"loki override", &EnterpriseOverrides{Observability: ObservabilityOverrides{Mode: "loki"}}, "prometheus", "loki"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ov.ResolveObservability(tt.fallback)
			if got != tt.want {
				t.Errorf("ResolveObservability(%q) = %q, esperava %q", tt.fallback, got, tt.want)
			}
		})
	}
}

func TestLoadOverrides_WithObservability(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overrides.yaml")
	content := `
observability:
  mode: thanos
`
	os.WriteFile(path, []byte(content), 0644)

	ov, err := LoadOverrides(path)
	if err != nil {
		t.Fatalf("erro ao carregar overrides: %v", err)
	}
	if ov.Observability.Mode != "thanos" {
		t.Errorf("observability.mode: esperava 'thanos', obteve %q", ov.Observability.Mode)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
