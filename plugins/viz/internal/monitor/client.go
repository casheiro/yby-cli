package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ListFilter contém filtros para listagem de recursos
type ListFilter struct {
	Namespace     string
	LabelSelector string
}

// Client define a interface para acessar recursos Kubernetes
type Client interface {
	GetPods(filter ListFilter) ([]Pod, error)
	GetDeployments(filter ListFilter) ([]Deployment, error)
	GetServices(filter ListFilter) ([]Service, error)
	GetNodes(filter ListFilter) ([]Node, error)
	GetStatefulSets(filter ListFilter) ([]StatefulSet, error)
	GetJobs(filter ListFilter) ([]Job, error)
	GetIngresses(filter ListFilter) ([]Ingress, error)
	GetConfigMaps(filter ListFilter) ([]ConfigMap, error)
	GetEvents(filter ListFilter) ([]Event, error)
}

// K8sClient conecta ao cluster real
type K8sClient struct {
	clientset  *kubernetes.Clientset
	restConfig *rest.Config
}

// Clientset retorna o clientset Kubernetes para uso em operações avançadas
func (c *K8sClient) Clientset() kubernetes.Interface {
	return c.clientset
}

// RESTConfig retorna a configuração REST para uso com dynamic client
func (c *K8sClient) RESTConfig() *rest.Config {
	return c.restConfig
}

// NewK8sClient cria um novo client Kubernetes a partir do kubeconfig
func NewK8sClient() (*K8sClient, error) {
	fmt.Println("🔌 Conectando ao Cluster K8s Real...")

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("falha ao carregar kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("falha ao criar clientset: %w", err)
	}

	return &K8sClient{clientset: clientset, restConfig: config}, nil
}

// podMetricsResult representa a resposta da API de métricas de pods.
type podMetricsResult struct {
	Items []struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Containers []struct {
			Usage struct {
				CPU    string `json:"cpu"`
				Memory string `json:"memory"`
			} `json:"usage"`
		} `json:"containers"`
	} `json:"items"`
}

// podMetricsKey gera uma chave única para identificar um pod nas métricas.
func podMetricsKey(namespace, name string) string {
	return namespace + "/" + name
}

// fetchPodMetrics busca métricas de CPU e memória do metrics-server.
// Retorna um mapa de "namespace/name" -> {cpu, memory}. Falha silenciosamente.
func (c *K8sClient) fetchPodMetrics() map[string][2]string {
	result := make(map[string][2]string)

	data, err := c.clientset.RESTClient().
		Get().
		AbsPath("/apis/metrics.k8s.io/v1beta1/pods").
		DoRaw(context.Background())
	if err != nil {
		return result // metrics-server indisponível
	}

	var metrics podMetricsResult
	if err := json.Unmarshal(data, &metrics); err != nil {
		return result
	}

	for _, item := range metrics.Items {
		var totalCPU, totalMem string
		if len(item.Containers) > 0 {
			totalCPU = item.Containers[0].Usage.CPU
			totalMem = item.Containers[0].Usage.Memory
		}
		key := podMetricsKey(item.Metadata.Namespace, item.Metadata.Name)
		result[key] = [2]string{totalCPU, totalMem}
	}

	return result
}

// GetPods lista pods com filtros opcionais de namespace e label.
func (c *K8sClient) GetPods(filter ListFilter) ([]Pod, error) {
	opts := metav1.ListOptions{}
	if filter.LabelSelector != "" {
		opts.LabelSelector = filter.LabelSelector
	}
	ns := filter.Namespace
	list, err := c.clientset.CoreV1().Pods(ns).List(context.Background(), opts)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar pods: %w", err)
	}

	// Buscar métricas (fallback graceful se metrics-server indisponível)
	metrics := c.fetchPodMetrics()

	var pods []Pod
	for _, p := range list.Items {
		cpu := "N/A"
		memory := "N/A"
		key := podMetricsKey(p.Namespace, p.Name)
		if m, ok := metrics[key]; ok {
			if m[0] != "" {
				cpu = m[0]
			}
			if m[1] != "" {
				memory = m[1]
			}
		}
		pods = append(pods, Pod{
			Name:      p.Name,
			Namespace: p.Namespace,
			Status:    string(p.Status.Phase),
			CPU:       cpu,
			Memory:    memory,
		})
	}
	return pods, nil
}

// GetDeployments lista deployments com filtros opcionais de namespace e label.
func (c *K8sClient) GetDeployments(filter ListFilter) ([]Deployment, error) {
	opts := metav1.ListOptions{}
	if filter.LabelSelector != "" {
		opts.LabelSelector = filter.LabelSelector
	}
	ns := filter.Namespace
	list, err := c.clientset.AppsV1().Deployments(ns).List(context.Background(), opts)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar deployments: %w", err)
	}
	var deps []Deployment
	for _, d := range list.Items {
		ready := d.Status.ReadyReplicas
		available := d.Status.AvailableReplicas
		replicas := int32(0)
		if d.Spec.Replicas != nil {
			replicas = *d.Spec.Replicas
		}
		deps = append(deps, Deployment{
			Name:      d.Name,
			Namespace: d.Namespace,
			Replicas:  replicas,
			Ready:     ready,
			Available: available,
		})
	}
	return deps, nil
}

// GetServices lista services com filtros opcionais de namespace e label.
func (c *K8sClient) GetServices(filter ListFilter) ([]Service, error) {
	opts := metav1.ListOptions{}
	if filter.LabelSelector != "" {
		opts.LabelSelector = filter.LabelSelector
	}
	ns := filter.Namespace
	list, err := c.clientset.CoreV1().Services(ns).List(context.Background(), opts)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar services: %w", err)
	}
	var svcs []Service
	for _, s := range list.Items {
		ports := ""
		for i, p := range s.Spec.Ports {
			if i > 0 {
				ports += ", "
			}
			ports += fmt.Sprintf("%d/%s", p.Port, p.Protocol)
		}
		svcs = append(svcs, Service{
			Name:      s.Name,
			Namespace: s.Namespace,
			Type:      string(s.Spec.Type),
			ClusterIP: s.Spec.ClusterIP,
			Ports:     ports,
		})
	}
	return svcs, nil
}

// GetNodes lista nodes com filtros opcionais de label.
func (c *K8sClient) GetNodes(filter ListFilter) ([]Node, error) {
	opts := metav1.ListOptions{}
	if filter.LabelSelector != "" {
		opts.LabelSelector = filter.LabelSelector
	}
	list, err := c.clientset.CoreV1().Nodes().List(context.Background(), opts)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar nodes: %w", err)
	}
	var nodes []Node
	for _, n := range list.Items {
		status := "NotReady"
		for _, cond := range n.Status.Conditions {
			if cond.Type == "Ready" && cond.Status == "True" {
				status = "Ready"
				break
			}
		}
		cpuCap := n.Status.Allocatable.Cpu().String()
		memCap := n.Status.Allocatable.Memory().String()
		nodes = append(nodes, Node{
			Name:           n.Name,
			Status:         status,
			CPUCapacity:    cpuCap,
			MemoryCapacity: memCap,
			Version:        n.Status.NodeInfo.KubeletVersion,
		})
	}
	return nodes, nil
}

// GetStatefulSets lista statefulsets com filtros opcionais de namespace e label.
func (c *K8sClient) GetStatefulSets(filter ListFilter) ([]StatefulSet, error) {
	opts := metav1.ListOptions{}
	if filter.LabelSelector != "" {
		opts.LabelSelector = filter.LabelSelector
	}
	ns := filter.Namespace
	list, err := c.clientset.AppsV1().StatefulSets(ns).List(context.Background(), opts)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar statefulsets: %w", err)
	}
	var result []StatefulSet
	for _, s := range list.Items {
		replicas := int32(0)
		if s.Spec.Replicas != nil {
			replicas = *s.Spec.Replicas
		}
		result = append(result, StatefulSet{
			Name:      s.Name,
			Namespace: s.Namespace,
			Replicas:  replicas,
			Ready:     s.Status.ReadyReplicas,
		})
	}
	return result, nil
}

// GetJobs lista jobs com filtros opcionais de namespace e label.
func (c *K8sClient) GetJobs(filter ListFilter) ([]Job, error) {
	opts := metav1.ListOptions{}
	if filter.LabelSelector != "" {
		opts.LabelSelector = filter.LabelSelector
	}
	ns := filter.Namespace
	list, err := c.clientset.BatchV1().Jobs(ns).List(context.Background(), opts)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar jobs: %w", err)
	}
	var result []Job
	for _, j := range list.Items {
		completions := int32(0)
		if j.Spec.Completions != nil {
			completions = *j.Spec.Completions
		}
		result = append(result, Job{
			Name:        j.Name,
			Namespace:   j.Namespace,
			Completions: completions,
			Active:      j.Status.Active,
			Succeeded:   j.Status.Succeeded,
			Failed:      j.Status.Failed,
		})
	}
	return result, nil
}

// GetIngresses lista ingresses com filtros opcionais de namespace e label.
func (c *K8sClient) GetIngresses(filter ListFilter) ([]Ingress, error) {
	opts := metav1.ListOptions{}
	if filter.LabelSelector != "" {
		opts.LabelSelector = filter.LabelSelector
	}
	ns := filter.Namespace
	list, err := c.clientset.NetworkingV1().Ingresses(ns).List(context.Background(), opts)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar ingresses: %w", err)
	}
	var result []Ingress
	for _, ing := range list.Items {
		class := ""
		if ing.Spec.IngressClassName != nil {
			class = *ing.Spec.IngressClassName
		}
		var hosts []string
		var paths []string
		for _, rule := range ing.Spec.Rules {
			if rule.Host != "" {
				hosts = append(hosts, rule.Host)
			}
			if rule.HTTP != nil {
				for _, p := range rule.HTTP.Paths {
					paths = append(paths, p.Path)
				}
			}
		}
		result = append(result, Ingress{
			Name:      ing.Name,
			Namespace: ing.Namespace,
			Class:     class,
			Hosts:     strings.Join(hosts, ", "),
			Paths:     strings.Join(paths, ", "),
		})
	}
	return result, nil
}

// GetConfigMaps lista configmaps com filtros opcionais de namespace e label.
func (c *K8sClient) GetConfigMaps(filter ListFilter) ([]ConfigMap, error) {
	opts := metav1.ListOptions{}
	if filter.LabelSelector != "" {
		opts.LabelSelector = filter.LabelSelector
	}
	ns := filter.Namespace
	list, err := c.clientset.CoreV1().ConfigMaps(ns).List(context.Background(), opts)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar configmaps: %w", err)
	}
	var result []ConfigMap
	for _, cm := range list.Items {
		totalSize := 0
		for _, v := range cm.Data {
			totalSize += len(v)
		}
		for _, v := range cm.BinaryData {
			totalSize += len(v)
		}
		dataSize := fmt.Sprintf("%dB", totalSize)
		if totalSize >= 1024 {
			dataSize = fmt.Sprintf("%.1fKi", float64(totalSize)/1024)
		}
		result = append(result, ConfigMap{
			Name:      cm.Name,
			Namespace: cm.Namespace,
			Keys:      len(cm.Data) + len(cm.BinaryData),
			DataSize:  dataSize,
		})
	}
	return result, nil
}

// formatAge formata uma duração em formato legível (ex: "5m", "2h", "3d")
func formatAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// GetEvents lista eventos com filtros opcionais de namespace e label.
func (c *K8sClient) GetEvents(filter ListFilter) ([]Event, error) {
	opts := metav1.ListOptions{}
	if filter.LabelSelector != "" {
		opts.LabelSelector = filter.LabelSelector
	}
	ns := filter.Namespace
	list, err := c.clientset.CoreV1().Events(ns).List(context.Background(), opts)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar eventos: %w", err)
	}

	// Ordenar por LastTimestamp desc
	sort.Slice(list.Items, func(i, j int) bool {
		return list.Items[i].LastTimestamp.Time.After(list.Items[j].LastTimestamp.Time)
	})

	var result []Event
	for _, e := range list.Items {
		age := "N/A"
		if !e.LastTimestamp.Time.IsZero() {
			age = formatAge(time.Since(e.LastTimestamp.Time))
		}
		result = append(result, Event{
			Name:      e.InvolvedObject.Name,
			Namespace: e.Namespace,
			Type:      e.Type,
			Reason:    e.Reason,
			Message:   e.Message,
			Age:       age,
		})
	}
	return result, nil
}
