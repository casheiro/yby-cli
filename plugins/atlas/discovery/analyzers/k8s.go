package analyzers

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// k8sManifest representa um documento YAML de manifesto Kubernetes.
type k8sManifest struct {
	APIVersion string                 `yaml:"apiVersion"`
	Kind       string                 `yaml:"kind"`
	Metadata   k8sMetadata            `yaml:"metadata"`
	Spec       map[string]interface{} `yaml:"spec"`
}

// k8sMetadata representa os metadados de um recurso Kubernetes.
type k8sMetadata struct {
	Name      string            `yaml:"name"`
	Namespace string            `yaml:"namespace"`
	Labels    map[string]string `yaml:"labels"`
}

// parsedResource agrupa um InfraResource com os dados brutos do manifesto para extração de relações.
type parsedResource struct {
	Resource InfraResource
	Raw      map[string]interface{}
}

// K8sAnalyzer analisa manifestos Kubernetes e extrai recursos e relações.
type K8sAnalyzer struct{}

// NewK8sAnalyzer cria uma nova instância do analyzer de manifestos Kubernetes.
func NewK8sAnalyzer() *K8sAnalyzer {
	return &K8sAnalyzer{}
}

// Name retorna o identificador do analyzer.
func (a *K8sAnalyzer) Name() string {
	return "k8s"
}

// Analyze recebe o path raiz do projeto e os arquivos relevantes,
// retornando os recursos e relações encontrados nos manifestos Kubernetes.
func (a *K8sAnalyzer) Analyze(rootPath string, files []string) (*AnalyzerResult, error) {
	result := &AnalyzerResult{
		Type: "k8s",
	}

	// Primeiro passo: coletar todos os recursos com dados brutos
	var parsed []parsedResource
	for _, file := range files {
		items, err := a.parseFile(rootPath, file)
		if err != nil {
			slog.Warn("falha ao parsear arquivo k8s", "file", file, "error", err)
			continue
		}
		parsed = append(parsed, items...)
	}

	for _, p := range parsed {
		result.Resources = append(result.Resources, p.Resource)
	}

	// Segundo passo: extrair relações semânticas entre os recursos
	result.Relations = a.extractRelations(parsed)

	return result, nil
}

// parseFile lê e parseia um arquivo YAML, suportando multi-documentos separados por ---.
func (a *K8sAnalyzer) parseFile(rootPath, filePath string) ([]parsedResource, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	content := string(data)
	if strings.TrimSpace(content) == "" {
		return nil, nil
	}

	relPath, err := filepath.Rel(rootPath, filePath)
	if err != nil {
		relPath = filePath
	}

	docs := splitYAMLDocuments(content)
	var resources []parsedResource

	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		var manifest k8sManifest
		if err := yaml.Unmarshal([]byte(doc), &manifest); err != nil {
			slog.Debug("documento YAML inválido, pulando", "file", filePath, "error", err)
			continue
		}

		// Pular documentos que não são manifestos K8s (sem apiVersion ou kind)
		if manifest.APIVersion == "" || manifest.Kind == "" {
			continue
		}

		// Pular templates Helm não renderizados
		if strings.Contains(manifest.Metadata.Name, "{{") {
			slog.Debug("pulando template Helm não renderizado", "file", filePath, "name", manifest.Metadata.Name)
			continue
		}

		// Pular recursos com nomes encriptados por SOPS
		if strings.Contains(manifest.Metadata.Name, "ENC[") {
			slog.Debug("pulando recurso encriptado por SOPS", "file", filePath)
			continue
		}
		if strings.Contains(manifest.Kind, "ENC[") {
			slog.Debug("pulando recurso com kind encriptado por SOPS", "file", filePath)
			continue
		}

		// Parsear dados brutos para extração de relações
		var raw map[string]interface{}
		_ = yaml.Unmarshal([]byte(doc), &raw)

		resource := InfraResource{
			Kind:      manifest.Kind,
			APIGroup:  manifest.APIVersion,
			Name:      manifest.Metadata.Name,
			Namespace: manifest.Metadata.Namespace,
			Path:      relPath,
			Labels:    manifest.Metadata.Labels,
		}

		resources = append(resources, parsedResource{
			Resource: resource,
			Raw:      raw,
		})
	}

	return resources, nil
}

// splitYAMLDocuments divide um conteúdo YAML em documentos individuais,
// respeitando o separador --- no início de linha.
func splitYAMLDocuments(content string) []string {
	return strings.Split(content, "\n---")
}

// extractRelations analisa os recursos coletados e identifica relações semânticas.
func (a *K8sAnalyzer) extractRelations(parsed []parsedResource) []InfraRelation {
	var relations []InfraRelation

	// Coletar apenas os recursos para busca
	resources := make([]InfraResource, len(parsed))
	for i, p := range parsed {
		resources[i] = p.Resource
	}

	for _, p := range parsed {
		spec := getNestedMap(p.Raw, "spec")

		switch p.Resource.Kind {
		case "Service":
			relations = append(relations, a.extractServiceRelations(p.Resource, spec, resources)...)
		case "Ingress":
			relations = append(relations, a.extractIngressRelations(p.Resource, spec, resources)...)
		case "Deployment", "StatefulSet", "DaemonSet":
			relations = append(relations, a.extractWorkloadRelations(p.Resource, spec, resources)...)
		case "ClusterRoleBinding", "RoleBinding":
			relations = append(relations, a.extractBindingRelations(p.Resource, p.Raw, resources)...)
		case "Application":
			relations = append(relations, a.extractArgoCDRelations(p.Resource, spec)...)
		}
	}

	return relations
}

// extractServiceRelations extrai relações de seleção de um Service para os recursos com labels correspondentes.
func (a *K8sAnalyzer) extractServiceRelations(svc InfraResource, spec map[string]interface{}, resources []InfraResource) []InfraRelation {
	if spec == nil {
		return nil
	}

	// Tentar spec.selector (Service v1) ou spec.selector.matchLabels
	selector := getNestedStringMap(spec, "selector")
	if selector == nil {
		selector = getNestedStringMap(spec, "selector", "matchLabels")
	}

	if len(selector) == 0 {
		return nil
	}

	var relations []InfraRelation

	// Encontrar recursos cujos labels coincidem com o selector
	for _, res := range resources {
		if res.Labels == nil {
			continue
		}
		// Não selecionar a si próprio
		if res.ID() == svc.ID() {
			continue
		}
		if labelsMatch(selector, res.Labels) {
			relations = append(relations, InfraRelation{
				From: svc.ID(),
				To:   res.ID(),
				Type: "selects",
			})
		}
	}

	return relations
}

// extractIngressRelations extrai relações de roteamento de um Ingress para Services.
func (a *K8sAnalyzer) extractIngressRelations(ing InfraResource, spec map[string]interface{}, resources []InfraResource) []InfraRelation {
	if spec == nil {
		return nil
	}

	var relations []InfraRelation

	rules := getNestedSlice(spec, "rules")
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]interface{})
		if !ok {
			continue
		}

		paths := getNestedSlice(ruleMap, "http", "paths")
		for _, p := range paths {
			pathMap, ok := p.(map[string]interface{})
			if !ok {
				continue
			}

			// Formato networking.k8s.io/v1: spec.rules[].http.paths[].backend.service.name
			svcName := getNestedString(pathMap, "backend", "service", "name")
			if svcName == "" {
				// Formato extensions/v1beta1: spec.rules[].http.paths[].backend.serviceName
				svcName = getNestedString(pathMap, "backend", "serviceName")
			}

			if svcName != "" {
				targetID := findResourceID(resources, "Service", svcName, ing.Namespace)
				if targetID == "" {
					if ing.Namespace != "" {
						targetID = "Service/" + ing.Namespace + "/" + svcName
					} else {
						targetID = "Service/" + svcName
					}
				}
				relations = append(relations, InfraRelation{
					From: ing.ID(),
					To:   targetID,
					Type: "routes",
				})
			}
		}
	}

	return relations
}

// extractWorkloadRelations extrai relações de referência de Deployment/StatefulSet/DaemonSet para ServiceAccount.
func (a *K8sAnalyzer) extractWorkloadRelations(wl InfraResource, spec map[string]interface{}, resources []InfraResource) []InfraRelation {
	if spec == nil {
		return nil
	}

	saName := getNestedString(spec, "template", "spec", "serviceAccountName")
	if saName == "" {
		return nil
	}

	targetID := findResourceID(resources, "ServiceAccount", saName, wl.Namespace)
	if targetID == "" {
		if wl.Namespace != "" {
			targetID = "ServiceAccount/" + wl.Namespace + "/" + saName
		} else {
			targetID = "ServiceAccount/" + saName
		}
	}

	return []InfraRelation{{
		From: wl.ID(),
		To:   targetID,
		Type: "references",
	}}
}

// extractBindingRelations extrai relações de referência de RoleBinding/ClusterRoleBinding para Role/ClusterRole.
func (a *K8sAnalyzer) extractBindingRelations(binding InfraResource, raw map[string]interface{}, resources []InfraResource) []InfraRelation {
	if raw == nil {
		return nil
	}

	// roleRef está no nível raiz do manifesto, não dentro de spec
	roleRef := getNestedMap(raw, "roleRef")
	if roleRef == nil {
		return nil
	}

	roleName, _ := roleRef["name"].(string)
	roleKind, _ := roleRef["kind"].(string)
	if roleName == "" || roleKind == "" {
		return nil
	}

	targetID := findResourceID(resources, roleKind, roleName, "")
	if targetID == "" {
		targetID = roleKind + "/" + roleName
	}

	return []InfraRelation{{
		From: binding.ID(),
		To:   targetID,
		Type: "references",
	}}
}

// extractArgoCDRelations extrai relações de deploy de uma Application do Argo CD.
func (a *K8sAnalyzer) extractArgoCDRelations(app InfraResource, spec map[string]interface{}) []InfraRelation {
	if spec == nil {
		return nil
	}

	var relations []InfraRelation

	// spec.source.path (Git) — ex: "charts/connectivity"
	sourcePath := getNestedString(spec, "source", "path")
	if sourcePath != "" {
		// Tentar resolver para HelmChart pelo nome do diretório final
		chartName := filepath.Base(sourcePath)
		relations = append(relations, InfraRelation{
			From: app.ID(),
			To:   "HelmChart/" + chartName,
			Type: "syncs",
		})
	}

	// spec.source.chart (Helm repo) — ex: "cert-manager"
	chart := getNestedString(spec, "source", "chart")
	if chart != "" {
		relations = append(relations, InfraRelation{
			From: app.ID(),
			To:   "HelmChart/" + chart,
			Type: "syncs",
		})
	}

	return relations
}

// getNestedString navega uma hierarquia de maps e retorna o valor string no final do caminho.
func getNestedString(m map[string]interface{}, keys ...string) string {
	if len(keys) == 0 || m == nil {
		return ""
	}

	current := m
	for i, key := range keys {
		val, ok := current[key]
		if !ok {
			return ""
		}

		if i == len(keys)-1 {
			s, _ := val.(string)
			return s
		}

		next, ok := val.(map[string]interface{})
		if !ok {
			return ""
		}
		current = next
	}

	return ""
}

// getNestedMap navega uma hierarquia de maps e retorna o map no final do caminho.
func getNestedMap(m map[string]interface{}, keys ...string) map[string]interface{} {
	if len(keys) == 0 || m == nil {
		return nil
	}

	current := m
	for _, key := range keys {
		val, ok := current[key]
		if !ok {
			return nil
		}
		next, ok := val.(map[string]interface{})
		if !ok {
			return nil
		}
		current = next
	}

	return current
}

// getNestedSlice navega uma hierarquia de maps e retorna o slice no final do caminho.
func getNestedSlice(m map[string]interface{}, keys ...string) []interface{} {
	if len(keys) == 0 || m == nil {
		return nil
	}

	current := m
	for i, key := range keys {
		val, ok := current[key]
		if !ok {
			return nil
		}

		if i == len(keys)-1 {
			s, _ := val.([]interface{})
			return s
		}

		next, ok := val.(map[string]interface{})
		if !ok {
			return nil
		}
		current = next
	}

	return nil
}

// getNestedStringMap navega uma hierarquia de maps e retorna um map[string]string no final do caminho.
func getNestedStringMap(m map[string]interface{}, keys ...string) map[string]string {
	nested := getNestedMap(m, keys...)
	if nested == nil {
		return nil
	}

	result := make(map[string]string, len(nested))
	for k, v := range nested {
		s, ok := v.(string)
		if ok {
			result[k] = s
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

// labelsMatch verifica se todos os labels do selector estão presentes nos labels do recurso.
func labelsMatch(selector, labels map[string]string) bool {
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

// findResourceID busca um recurso pelo Kind, Name e Namespace opcionais, retornando seu ID.
func findResourceID(resources []InfraResource, kind, name, namespace string) string {
	for _, res := range resources {
		if res.Kind == kind && res.Name == name {
			if namespace == "" || res.Namespace == namespace {
				return res.ID()
			}
		}
	}
	return ""
}
