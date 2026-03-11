package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ═══════════════════════════════════════════════════════════════════════════════
// PLAY-02: targetRevision usa GitBranch
// ═══════════════════════════════════════════════════════════════════════════════

func TestPlay02_RootAppTargetRevision(t *testing.T) {
	tmpDir := t.TempDir()

	mockFS := fstest.MapFS{
		"assets/manifests/argocd/root-app.yaml.tmpl": &fstest.MapFile{
			Data: []byte(`targetRevision: {{ .GitBranch }}`),
		},
	}

	tests := []struct {
		name     string
		branch   string
		expected string
	}{
		{"branch develop", "develop", "targetRevision: develop"},
		{"branch main", "main", "targetRevision: main"},
		{"branch custom", "release/v1.0", "targetRevision: release/v1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := filepath.Join(tmpDir, tt.name)
			require.NoError(t, os.MkdirAll(dir, 0755))

			ctx := &BlueprintContext{GitBranch: tt.branch}
			err := Apply(dir, ctx, mockFS)
			require.NoError(t, err)

			data, err := os.ReadFile(filepath.Join(dir, "manifests/argocd/root-app.yaml"))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(data))
		})
	}
}

func TestPlay02_BootstrapTargetRevision(t *testing.T) {
	tmpDir := t.TempDir()

	mockFS := fstest.MapFS{
		"assets/charts/bootstrap/values.yaml.tmpl": &fstest.MapFile{
			Data: []byte(`targetRevision: {{ .GitBranch }}`),
		},
	}

	ctx := &BlueprintContext{GitBranch: "develop"}
	err := Apply(tmpDir, ctx, mockFS)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmpDir, "charts/bootstrap/values.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "targetRevision: develop", string(data))
}

// ═══════════════════════════════════════════════════════════════════════════════
// PLAY-03: Sem chaves YAML duplicadas no bootstrap
// ═══════════════════════════════════════════════════════════════════════════════

func TestPlay03_BootstrapNoDuplicateKeys(t *testing.T) {
	tmpDir := t.TempDir()

	// Simular o template real de bootstrap/values.yaml.tmpl
	mockFS := fstest.MapFS{
		"assets/charts/bootstrap/values.yaml.tmpl": &fstest.MapFile{
			Data: []byte(`events:
  enabled: true
  eventbus:
    replicas: 1
  webhook:
    port: 12000
    serviceType: NodePort
discovery:
  organization: "{{ .GithubOrg }}"
  topic: "yby-cluster"
ingress:
  tls:
    email: {{ .Email }}`),
		},
	}

	ctx := &BlueprintContext{
		GithubOrg: "acme",
		Email:     "devops@acme.com",
	}

	err := Apply(tmpDir, ctx, mockFS)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmpDir, "charts/bootstrap/values.yaml"))
	require.NoError(t, err)
	content := string(data)

	// Verificar que organization aparece apenas 1 vez
	orgCount := strings.Count(content, "organization:")
	assert.Equal(t, 1, orgCount, "organization deve aparecer apenas 1 vez")

	// Verificar que events aparece apenas 1 vez
	eventsCount := strings.Count(content, "events:")
	assert.Equal(t, 1, eventsCount, "events deve aparecer apenas 1 vez")

	// Verificar que webhook está presente (não removido pela duplicata)
	assert.Contains(t, content, "webhook:")
	assert.Contains(t, content, "serviceType: NodePort")

	// Verificar que email usa o campo do contexto
	assert.Contains(t, content, "email: devops@acme.com")
	assert.NotContains(t, content, "admin@example.com")
}

// ═══════════════════════════════════════════════════════════════════════════════
// PLAY-05: blueprint.yaml usa ProjectName
// ═══════════════════════════════════════════════════════════════════════════════

func TestPlay05_BlueprintUsesProjectName(t *testing.T) {
	tmpDir := t.TempDir()

	mockFS := fstest.MapFS{
		"assets/.yby/blueprint.yaml.tmpl": &fstest.MapFile{
			Data: []byte(`metadata:
  name: {{ .ProjectName }}`),
		},
	}

	ctx := &BlueprintContext{ProjectName: "finpay-gateway"}
	err := Apply(tmpDir, ctx, mockFS)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmpDir, ".yby/blueprint.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "name: finpay-gateway")
	assert.NotContains(t, string(data), "standard-init")
}

// ═══════════════════════════════════════════════════════════════════════════════
// PLAY-07: Security context ativo por padrão
// ═══════════════════════════════════════════════════════════════════════════════

func TestPlay07_SecurityContextDefaults(t *testing.T) {
	tmpDir := t.TempDir()

	mockFS := fstest.MapFS{
		"assets/charts/app-template/values.yaml.tmpl": &fstest.MapFile{
			Data: []byte(`podSecurityContext:
  runAsNonRoot: true
  seccompProfile:
    type: RuntimeDefault

securityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  runAsNonRoot: true
  runAsUser: 1000
  capabilities:
    drop:
      - ALL`),
		},
	}

	ctx := &BlueprintContext{}
	err := Apply(tmpDir, ctx, mockFS)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmpDir, "charts/app-template/values.yaml"))
	require.NoError(t, err)
	content := string(data)

	// Verificar valores de segurança presentes
	assert.Contains(t, content, "runAsNonRoot: true")
	assert.Contains(t, content, "allowPrivilegeEscalation: false")
	assert.Contains(t, content, "readOnlyRootFilesystem: true")
	assert.Contains(t, content, "runAsUser: 1000")
	assert.Contains(t, content, "- ALL")
	assert.Contains(t, content, "RuntimeDefault")

	// Verificar que NÃO tem security context vazio
	assert.NotContains(t, content, "podSecurityContext: {}")
	assert.NotContains(t, content, "securityContext: {}")
}

// ═══════════════════════════════════════════════════════════════════════════════
// PLAY-08: AppProject RBAC restrito
// ═══════════════════════════════════════════════════════════════════════════════

func TestPlay08_AppProjectRBACRestricted(t *testing.T) {
	tmpDir := t.TempDir()

	mockFS := fstest.MapFS{
		"assets/manifests/projects/yby-project.yaml.tmpl": &fstest.MapFile{
			Data: []byte(`destinations:
  - namespace: {{ .ProjectName }}
    server: https://kubernetes.default.svc
  - namespace: argocd
    server: https://kubernetes.default.svc
clusterResourceWhitelist:
  - group: ''
    kind: Namespace
  - group: rbac.authorization.k8s.io
    kind: ClusterRole`),
		},
	}

	ctx := &BlueprintContext{ProjectName: "finpay"}
	err := Apply(tmpDir, ctx, mockFS)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmpDir, "manifests/projects/yby-project.yaml"))
	require.NoError(t, err)
	content := string(data)

	// Verificar que namespace usa ProjectName
	assert.Contains(t, content, "namespace: finpay")
	// Verificar que NÃO tem wildcard em namespace
	assert.NotContains(t, content, "namespace: '*'")
	// Verificar que clusterResourceWhitelist é restrito
	assert.Contains(t, content, "kind: Namespace")
	assert.NotContains(t, content, "kind: '*'")
}

// ═══════════════════════════════════════════════════════════════════════════════
// PLAY-09: Metrics-server condicional por ambiente
// ═══════════════════════════════════════════════════════════════════════════════

func TestPlay09_MetricsServerInsecureTLS_Local(t *testing.T) {
	tmpDir := t.TempDir()

	mockFS := fstest.MapFS{
		"assets/manifests/observability/metrics-server.yaml.tmpl": &fstest.MapFile{
			Data: []byte(`args:
        - --secure-port=4443
        {{- if or (eq .Environment "local") (eq .Environment "dev") }}
        - --kubelet-insecure-tls
        {{- end }}`),
		},
	}

	ctx := &BlueprintContext{
		Environment:         "local",
		EnableMetricsServer: true,
	}
	err := Apply(tmpDir, ctx, mockFS)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmpDir, "manifests/observability/metrics-server.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "--kubelet-insecure-tls")
}

func TestPlay09_MetricsServerInsecureTLS_Prod(t *testing.T) {
	tmpDir := t.TempDir()

	mockFS := fstest.MapFS{
		"assets/manifests/observability/metrics-server.yaml.tmpl": &fstest.MapFile{
			Data: []byte(`args:
        - --secure-port=4443
        {{- if or (eq .Environment "local") (eq .Environment "dev") }}
        - --kubelet-insecure-tls
        {{- end }}`),
		},
	}

	ctx := &BlueprintContext{
		Environment:         "prod",
		EnableMetricsServer: true,
	}
	err := Apply(tmpDir, ctx, mockFS)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmpDir, "manifests/observability/metrics-server.yaml"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "--kubelet-insecure-tls")
}

// ═══════════════════════════════════════════════════════════════════════════════
// PLAY-11: Versão k3s consistente
// ═══════════════════════════════════════════════════════════════════════════════

func TestPlay11_K3sVersionConsistent(t *testing.T) {
	tmpDir := t.TempDir()

	mockFS := fstest.MapFS{
		"assets/config/cluster-values.yaml.tmpl": &fstest.MapFile{
			Data: []byte(`k3s:
    version: "v1.31.2+k3s1"`),
		},
		"assets/charts/system/values.yaml": &fstest.MapFile{
			Data: []byte(`k3s:
  version: v1.31.2+k3s1`),
		},
	}

	ctx := &BlueprintContext{}
	err := Apply(tmpDir, ctx, mockFS)
	require.NoError(t, err)

	clusterValues, err := os.ReadFile(filepath.Join(tmpDir, "config/cluster-values.yaml"))
	require.NoError(t, err)

	systemValues, err := os.ReadFile(filepath.Join(tmpDir, "charts/system/values.yaml"))
	require.NoError(t, err)

	// Ambos devem conter a mesma versão
	assert.Contains(t, string(clusterValues), "v1.31.2+k3s1")
	assert.Contains(t, string(systemValues), "v1.31.2+k3s1")
}

// ═══════════════════════════════════════════════════════════════════════════════
// PLAY-12: Grafana sem senha hardcoded
// ═══════════════════════════════════════════════════════════════════════════════

func TestPlay12_GrafanaNoHardcodedPassword(t *testing.T) {
	tmpDir := t.TempDir()

	mockFS := fstest.MapFS{
		"assets/charts/bootstrap/values.yaml.tmpl": &fstest.MapFile{
			Data: []byte(`grafana:
    adminPassword: ""`),
		},
	}

	ctx := &BlueprintContext{}
	err := Apply(tmpDir, ctx, mockFS)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmpDir, "charts/bootstrap/values.yaml"))
	require.NoError(t, err)
	content := string(data)

	assert.NotContains(t, content, `adminPassword: "admin"`)
}

// ═══════════════════════════════════════════════════════════════════════════════
// PLAY-13: Network Policy com Egress
// ═══════════════════════════════════════════════════════════════════════════════

func TestPlay13_NetworkPolicyEgress(t *testing.T) {
	// Este template usa Helm template syntax (.Values), não scaffold syntax (.Field)
	// Verificamos o conteúdo do arquivo estático
	tmpDir := t.TempDir()

	content := `{{- if .Values.security.networkPolicy.enabled -}}
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-all
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-dns-egress
spec:
  podSelector: {}
  policyTypes:
  - Egress
  egress:
  - ports:
    - protocol: UDP
      port: 53
    - protocol: TCP
      port: 53
{{- end -}}`

	mockFS := fstest.MapFS{
		"assets/charts/cluster-config/templates/network-policies/default-deny.yaml": &fstest.MapFile{
			Data: []byte(content),
		},
	}

	ctx := &BlueprintContext{}
	err := Apply(tmpDir, ctx, mockFS)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmpDir, "charts/cluster-config/templates/network-policies/default-deny.yaml"))
	require.NoError(t, err)
	fileContent := string(data)

	// Verificar que contém Egress
	assert.Contains(t, fileContent, "- Egress")
	// Verificar que permite DNS
	assert.Contains(t, fileContent, "allow-dns-egress")
	assert.Contains(t, fileContent, "port: 53")
}

// ═══════════════════════════════════════════════════════════════════════════════
// PLAY-14: className traefik
// ═══════════════════════════════════════════════════════════════════════════════

func TestPlay14_IngressClassTraefik(t *testing.T) {
	tmpDir := t.TempDir()

	mockFS := fstest.MapFS{
		"assets/charts/app-template/values.yaml.tmpl": &fstest.MapFile{
			Data: []byte(`ingress:
  className: "traefik"
  hosts:
    - host: {{ .ProjectName }}.{{ .Domain }}`),
		},
	}

	ctx := &BlueprintContext{
		ProjectName: "my-app",
		Domain:      "example.com",
	}
	err := Apply(tmpDir, ctx, mockFS)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmpDir, "charts/app-template/values.yaml"))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, `className: "traefik"`)
	assert.NotContains(t, content, "nginx")
}

// ═══════════════════════════════════════════════════════════════════════════════
// PLAY-15: Config validator corrigido
// ═══════════════════════════════════════════════════════════════════════════════

func TestPlay15_ConfigValidatorFixedCommand(t *testing.T) {
	tmpDir := t.TempDir()

	content := `apiVersion: batch/v1
kind: Job
metadata:
  name: config-validator
spec:
  template:
    spec:
      containers:
      - name: validator
        image: bitnami/kubectl:1.31.2
        command: ["/bin/bash", "-c"]
        args:
        - |
          echo "Validando..."`

	mockFS := fstest.MapFS{
		"assets/charts/cluster-config/templates/hooks/config-validator.yaml": &fstest.MapFile{
			Data: []byte(content),
		},
	}

	ctx := &BlueprintContext{}
	err := Apply(tmpDir, ctx, mockFS)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmpDir, "charts/cluster-config/templates/hooks/config-validator.yaml"))
	require.NoError(t, err)
	fileContent := string(data)

	// Verificar que command usa -c (consistente com args inline)
	assert.Contains(t, fileContent, `command: ["/bin/bash", "-c"]`)
	// Verificar que NÃO referencia /scripts/validate.sh
	assert.NotContains(t, fileContent, "/scripts/validate.sh")
}

// ═══════════════════════════════════════════════════════════════════════════════
// PLAY-16: Workflow trunkbased diferenciado
// ═══════════════════════════════════════════════════════════════════════════════

func TestPlay16_TrunkbasedWorkflowFiltering(t *testing.T) {
	// Testar via filtro shouldSkip que trunkbased é aceito e gitflow é rejeitado
	ctx := &BlueprintContext{
		EnableCI:        true,
		WorkflowPattern: "trunkbased",
	}

	// Workflows trunkbased devem passar
	assert.False(t, shouldSkip("assets/.github/workflows/trunkbased/pr-main-checks.yaml.tmpl", ctx),
		"trunkbased/pr-main-checks deve ser incluído")
	assert.False(t, shouldSkip("assets/.github/workflows/trunkbased/continuous-deploy.yaml.tmpl", ctx),
		"trunkbased/continuous-deploy deve ser incluído")
	assert.False(t, shouldSkip("assets/.github/workflows/trunkbased/sync-labels.yaml.tmpl", ctx),
		"trunkbased/sync-labels deve ser incluído")

	// Workflows gitflow devem ser pulados
	assert.True(t, shouldSkip("assets/.github/workflows/gitflow/release.yaml.tmpl", ctx),
		"gitflow/release não deve ser incluído para trunkbased")
	assert.True(t, shouldSkip("assets/.github/workflows/essential/pr-main-checks.yaml.tmpl", ctx),
		"essential/pr-main-checks não deve ser incluído para trunkbased")
}

func TestPlay16_TrunkbasedWorkflowRender(t *testing.T) {
	tmpDir := t.TempDir()

	mockFS := fstest.MapFS{
		"assets/config/deploy.yaml.tmpl": &fstest.MapFile{
			Data: []byte(`branch: {{ .GitBranch }}`),
		},
	}

	ctx := &BlueprintContext{
		EnableCI:        true,
		WorkflowPattern: "trunkbased",
		GitBranch:       "main",
	}

	err := Apply(tmpDir, ctx, mockFS)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmpDir, "config/deploy.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "branch: main", string(data))
}

// ═══════════════════════════════════════════════════════════════════════════════
// PLAY-17: KEDA/Kepler flags propagadas
// ═══════════════════════════════════════════════════════════════════════════════

func TestPlay17_KedaKeplerFlagsPropagated(t *testing.T) {
	tmpDir := t.TempDir()

	mockFS := fstest.MapFS{
		"assets/config/cluster-values.yaml.tmpl": &fstest.MapFile{
			Data: []byte(`keda:
  enabled: {{ .EnableKEDA }}
kepler:
  enabled: {{ .EnableKepler }}`),
		},
	}

	tests := []struct {
		name       string
		keda       bool
		kepler     bool
		expectKeda string
		expectKep  string
	}{
		{"ambos habilitados", true, true, "enabled: true", "enabled: true"},
		{"ambos desabilitados", false, false, "enabled: false", "enabled: false"},
		{"apenas keda", true, false, "enabled: true", "enabled: false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := filepath.Join(tmpDir, tt.name)
			require.NoError(t, os.MkdirAll(dir, 0755))

			ctx := &BlueprintContext{
				EnableKEDA:   tt.keda,
				EnableKepler: tt.kepler,
			}
			err := Apply(dir, ctx, mockFS)
			require.NoError(t, err)

			data, err := os.ReadFile(filepath.Join(dir, "config/cluster-values.yaml"))
			require.NoError(t, err)
			content := string(data)

			assert.Contains(t, content, "keda:\n  "+tt.expectKeda)
			assert.Contains(t, content, "kepler:\n  "+tt.expectKep)
		})
	}
}

func TestPlay17_BootstrapDefaultsFalse(t *testing.T) {
	tmpDir := t.TempDir()

	mockFS := fstest.MapFS{
		"assets/charts/bootstrap/values.yaml.tmpl": &fstest.MapFile{
			Data: []byte(`keda:
  enabled: false
kepler:
  enabled: false`),
		},
	}

	ctx := &BlueprintContext{}
	err := Apply(tmpDir, ctx, mockFS)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmpDir, "charts/bootstrap/values.yaml"))
	require.NoError(t, err)
	content := string(data)

	// Defaults devem ser false no bootstrap
	assert.Contains(t, content, "keda:\n  enabled: false")
	assert.Contains(t, content, "kepler:\n  enabled: false")
}

// ═══════════════════════════════════════════════════════════════════════════════
// PLAY-09: Filtro metrics-server com .tmpl
// ═══════════════════════════════════════════════════════════════════════════════

func TestPlay09_MetricsServerFilterWithTmpl(t *testing.T) {
	// Verificar que o filtro funciona com o novo nome .tmpl
	ctx := &BlueprintContext{EnableMetricsServer: false}
	assert.True(t, shouldSkip("assets/manifests/observability/metrics-server.yaml.tmpl", ctx),
		"metrics-server.yaml.tmpl deve ser pulado quando desabilitado")

	ctx2 := &BlueprintContext{EnableMetricsServer: true}
	assert.False(t, shouldSkip("assets/manifests/observability/metrics-server.yaml.tmpl", ctx2),
		"metrics-server.yaml.tmpl deve ser incluído quando habilitado")
}
