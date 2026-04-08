package analyzers

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTestFile cria um arquivo temporário com o conteúdo dado e retorna o caminho.
func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestK8sAnalyzer_Name(t *testing.T) {
	a := NewK8sAnalyzer()
	if a.Name() != "k8s" {
		t.Errorf("esperado 'k8s', obtido '%s'", a.Name())
	}
}

func TestK8sAnalyzer_SimpleDeployment(t *testing.T) {
	dir := t.TempDir()
	file := writeTestFile(t, dir, "deploy.yaml", `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: default
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.21
`)

	a := NewK8sAnalyzer()
	result, err := a.Analyze(dir, []string{file})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("esperado 1 recurso, obtido %d", len(result.Resources))
	}

	res := result.Resources[0]
	if res.Kind != "Deployment" {
		t.Errorf("esperado Kind 'Deployment', obtido '%s'", res.Kind)
	}
	if res.Name != "nginx" {
		t.Errorf("esperado Name 'nginx', obtido '%s'", res.Name)
	}
	if res.Namespace != "default" {
		t.Errorf("esperado Namespace 'default', obtido '%s'", res.Namespace)
	}
	if res.APIGroup != "apps/v1" {
		t.Errorf("esperado APIGroup 'apps/v1', obtido '%s'", res.APIGroup)
	}
	if res.Path != "deploy.yaml" {
		t.Errorf("esperado Path 'deploy.yaml', obtido '%s'", res.Path)
	}
	if res.Labels["app"] != "nginx" {
		t.Errorf("esperado label app=nginx, obtido '%s'", res.Labels["app"])
	}
}

func TestK8sAnalyzer_MultiDocument(t *testing.T) {
	dir := t.TempDir()
	file := writeTestFile(t, dir, "multi.yaml", `
apiVersion: v1
kind: Service
metadata:
  name: web-svc
  namespace: default
spec:
  selector:
    app: web
  ports:
  - port: 80
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: default
  labels:
    app: web
spec:
  replicas: 2
  template:
    metadata:
      labels:
        app: web
    spec:
      containers:
      - name: web
        image: web:latest
`)

	a := NewK8sAnalyzer()
	result, err := a.Analyze(dir, []string{file})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Resources) != 2 {
		t.Fatalf("esperado 2 recursos, obtido %d", len(result.Resources))
	}

	kinds := map[string]bool{}
	for _, r := range result.Resources {
		kinds[r.Kind] = true
	}
	if !kinds["Service"] {
		t.Error("esperado recurso do tipo Service")
	}
	if !kinds["Deployment"] {
		t.Error("esperado recurso do tipo Deployment")
	}
}

func TestK8sAnalyzer_ServiceSelectsDeployment(t *testing.T) {
	dir := t.TempDir()
	file := writeTestFile(t, dir, "svc-deploy.yaml", `
apiVersion: v1
kind: Service
metadata:
  name: api-svc
  namespace: prod
spec:
  selector:
    app: api
    tier: backend
  ports:
  - port: 8080
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: prod
  labels:
    app: api
    tier: backend
    version: v2
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: api
        image: api:v2
`)

	a := NewK8sAnalyzer()
	result, err := a.Analyze(dir, []string{file})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Relations) == 0 {
		t.Fatal("esperado ao menos 1 relação")
	}

	found := false
	for _, rel := range result.Relations {
		if rel.Type == "selects" && rel.From == "Service/prod/api-svc" && rel.To == "Deployment/prod/api" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("esperado relação 'selects' de Service/prod/api-svc para Deployment/prod/api, relações: %+v", result.Relations)
	}
}

func TestK8sAnalyzer_IngressRoutesToService(t *testing.T) {
	dir := t.TempDir()
	file := writeTestFile(t, dir, "ingress.yaml", `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: web-ingress
  namespace: default
spec:
  rules:
  - host: example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: web-svc
            port:
              number: 80
---
apiVersion: v1
kind: Service
metadata:
  name: web-svc
  namespace: default
spec:
  ports:
  - port: 80
`)

	a := NewK8sAnalyzer()
	result, err := a.Analyze(dir, []string{file})
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, rel := range result.Relations {
		if rel.Type == "routes" && rel.From == "Ingress/default/web-ingress" && rel.To == "Service/default/web-svc" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("esperado relação 'routes' de Ingress para Service, relações: %+v", result.Relations)
	}
}

func TestK8sAnalyzer_ArgoCDApplication(t *testing.T) {
	dir := t.TempDir()
	file := writeTestFile(t, dir, "argoapp.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: argocd
spec:
  source:
    repoURL: https://github.com/org/repo
    path: k8s/overlays/prod
    targetRevision: HEAD
  destination:
    server: https://kubernetes.default.svc
    namespace: prod
`)

	a := NewK8sAnalyzer()
	result, err := a.Analyze(dir, []string{file})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("esperado 1 recurso, obtido %d", len(result.Resources))
	}

	if result.Resources[0].Kind != "Application" {
		t.Errorf("esperado Kind 'Application', obtido '%s'", result.Resources[0].Kind)
	}

	found := false
	for _, rel := range result.Relations {
		if rel.Type == "syncs" && rel.To == "HelmChart/prod" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("esperado relação 'syncs' para 'HelmChart/prod', relações: %+v", result.Relations)
	}
}

func TestK8sAnalyzer_ArgoCDApplicationWithChart(t *testing.T) {
	dir := t.TempDir()
	file := writeTestFile(t, dir, "argohelm.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: redis
  namespace: argocd
spec:
  source:
    repoURL: https://charts.bitnami.com/bitnami
    chart: redis
    targetRevision: 17.0.0
  destination:
    server: https://kubernetes.default.svc
    namespace: cache
`)

	a := NewK8sAnalyzer()
	result, err := a.Analyze(dir, []string{file})
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, rel := range result.Relations {
		if rel.Type == "syncs" && rel.To == "HelmChart/redis" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("esperado relação 'syncs' para 'HelmChart/redis', relações: %+v", result.Relations)
	}
}

func TestK8sAnalyzer_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	file := writeTestFile(t, dir, "invalid.yaml", `
this: is not
  a valid: kubernetes
    manifest: [broken
`)

	a := NewK8sAnalyzer()
	result, err := a.Analyze(dir, []string{file})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Resources) != 0 {
		t.Errorf("esperado 0 recursos para YAML inválido, obtido %d", len(result.Resources))
	}
}

func TestK8sAnalyzer_NonK8sYAML(t *testing.T) {
	dir := t.TempDir()
	file := writeTestFile(t, dir, "config.yaml", `
database:
  host: localhost
  port: 5432
  name: mydb
`)

	a := NewK8sAnalyzer()
	result, err := a.Analyze(dir, []string{file})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Resources) != 0 {
		t.Errorf("esperado 0 recursos para YAML não-K8s, obtido %d", len(result.Resources))
	}
}

func TestK8sAnalyzer_HelmTemplateSkipped(t *testing.T) {
	dir := t.TempDir()
	file := writeTestFile(t, dir, "helm-template.yaml", `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}-app
  namespace: default
spec:
  replicas: 1
`)

	a := NewK8sAnalyzer()
	result, err := a.Analyze(dir, []string{file})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Resources) != 0 {
		t.Errorf("esperado 0 recursos para template Helm não renderizado, obtido %d", len(result.Resources))
	}
}

func TestK8sAnalyzer_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	file := writeTestFile(t, dir, "empty.yaml", "")

	a := NewK8sAnalyzer()
	result, err := a.Analyze(dir, []string{file})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Resources) != 0 {
		t.Errorf("esperado 0 recursos para arquivo vazio, obtido %d", len(result.Resources))
	}
}

func TestK8sAnalyzer_WhitespaceOnlyFile(t *testing.T) {
	dir := t.TempDir()
	file := writeTestFile(t, dir, "whitespace.yaml", "   \n\n  \n")

	a := NewK8sAnalyzer()
	result, err := a.Analyze(dir, []string{file})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Resources) != 0 {
		t.Errorf("esperado 0 recursos para arquivo só com espaços, obtido %d", len(result.Resources))
	}
}

func TestK8sAnalyzer_DeploymentReferencesServiceAccount(t *testing.T) {
	dir := t.TempDir()
	file := writeTestFile(t, dir, "deploy-sa.yaml", `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: app-sa
  namespace: prod
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
  namespace: prod
  labels:
    app: myapp
spec:
  template:
    spec:
      serviceAccountName: app-sa
      containers:
      - name: app
        image: app:latest
`)

	a := NewK8sAnalyzer()
	result, err := a.Analyze(dir, []string{file})
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, rel := range result.Relations {
		if rel.Type == "references" && rel.From == "Deployment/prod/app" && rel.To == "ServiceAccount/prod/app-sa" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("esperado relação 'references' de Deployment para ServiceAccount, relações: %+v", result.Relations)
	}
}

func TestK8sAnalyzer_ClusterRoleBindingReferences(t *testing.T) {
	dir := t.TempDir()
	file := writeTestFile(t, dir, "rbac.yaml", `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: admin-role
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: admin-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: admin-role
subjects:
- kind: ServiceAccount
  name: admin-sa
  namespace: kube-system
`)

	a := NewK8sAnalyzer()
	result, err := a.Analyze(dir, []string{file})
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, rel := range result.Relations {
		if rel.Type == "references" && rel.From == "ClusterRoleBinding/admin-binding" && rel.To == "ClusterRole/admin-role" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("esperado relação 'references' de ClusterRoleBinding para ClusterRole, relações: %+v", result.Relations)
	}
}

func TestK8sAnalyzer_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	file1 := writeTestFile(t, dir, "svc.yaml", `
apiVersion: v1
kind: Service
metadata:
  name: db-svc
spec:
  ports:
  - port: 5432
`)
	file2 := writeTestFile(t, dir, "deploy.yaml", `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: db
spec:
  replicas: 1
`)

	a := NewK8sAnalyzer()
	result, err := a.Analyze(dir, []string{file1, file2})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Resources) != 2 {
		t.Errorf("esperado 2 recursos de arquivos diferentes, obtido %d", len(result.Resources))
	}
}

func TestK8sAnalyzer_NonexistentFile(t *testing.T) {
	dir := t.TempDir()
	fakePath := filepath.Join(dir, "nao-existe.yaml")

	a := NewK8sAnalyzer()
	result, err := a.Analyze(dir, []string{fakePath})
	// Não deve retornar erro fatal, apenas avisar via log
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Resources) != 0 {
		t.Errorf("esperado 0 recursos para arquivo inexistente, obtido %d", len(result.Resources))
	}
}

func TestK8sAnalyzer_RelativePathCalculation(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "k8s", "base")
	file := writeTestFile(t, dir, "k8s/base/deploy.yaml", `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nested-app
`)
	_ = subDir

	a := NewK8sAnalyzer()
	result, err := a.Analyze(dir, []string{file})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("esperado 1 recurso, obtido %d", len(result.Resources))
	}

	expectedPath := filepath.Join("k8s", "base", "deploy.yaml")
	if result.Resources[0].Path != expectedPath {
		t.Errorf("esperado Path '%s', obtido '%s'", expectedPath, result.Resources[0].Path)
	}
}

func TestK8sAnalyzer_ImplementsInterface(t *testing.T) {
	var _ Analyzer = NewK8sAnalyzer()
}

func TestGetNestedString(t *testing.T) {
	m := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": "valor",
			},
		},
	}

	if v := getNestedString(m, "a", "b", "c"); v != "valor" {
		t.Errorf("esperado 'valor', obtido '%s'", v)
	}

	if v := getNestedString(m, "a", "x"); v != "" {
		t.Errorf("esperado string vazia para chave inexistente, obtido '%s'", v)
	}

	if v := getNestedString(nil, "a"); v != "" {
		t.Errorf("esperado string vazia para map nil, obtido '%s'", v)
	}
}

func TestGetNestedMap(t *testing.T) {
	inner := map[string]interface{}{"key": "val"}
	m := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": inner,
		},
	}

	result := getNestedMap(m, "level1", "level2")
	if result == nil {
		t.Fatal("esperado map não-nil")
	}
	if result["key"] != "val" {
		t.Errorf("esperado 'val', obtido '%v'", result["key"])
	}

	if getNestedMap(m, "inexistente") != nil {
		t.Error("esperado nil para chave inexistente")
	}
}

func TestLabelsMatch(t *testing.T) {
	selector := map[string]string{"app": "web", "tier": "frontend"}
	labels := map[string]string{"app": "web", "tier": "frontend", "version": "v1"}

	if !labelsMatch(selector, labels) {
		t.Error("esperado match quando labels contêm todas as chaves do selector")
	}

	labelsParciais := map[string]string{"app": "web"}
	if labelsMatch(selector, labelsParciais) {
		t.Error("esperado não-match quando labels não contêm todas as chaves do selector")
	}
}
