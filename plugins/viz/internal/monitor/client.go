package monitor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Client define a interface para acessar recursos Kubernetes
type Client interface {
	GetPods() ([]Pod, error)
	GetDeployments() ([]Deployment, error)
	GetServices() ([]Service, error)
	GetNodes() ([]Node, error)
}

// K8sClient conecta ao cluster real
type K8sClient struct {
	clientset *kubernetes.Clientset
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

	return &K8sClient{clientset: clientset}, nil
}

// GetPods lista todos os pods de todos os namespaces
func (c *K8sClient) GetPods() ([]Pod, error) {
	list, err := c.clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar pods: %w", err)
	}

	var pods []Pod
	for _, p := range list.Items {
		pods = append(pods, Pod{
			Name:      p.Name,
			Namespace: p.Namespace,
			Status:    string(p.Status.Phase),
			CPU:       "N/A", // Requer metrics-server
		})
	}
	return pods, nil
}

// GetDeployments lista todos os deployments de todos os namespaces
func (c *K8sClient) GetDeployments() ([]Deployment, error) {
	list, err := c.clientset.AppsV1().Deployments("").List(context.Background(), metav1.ListOptions{})
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

// GetServices lista todos os services de todos os namespaces
func (c *K8sClient) GetServices() ([]Service, error) {
	list, err := c.clientset.CoreV1().Services("").List(context.Background(), metav1.ListOptions{})
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

// GetNodes lista todos os nodes do cluster
func (c *K8sClient) GetNodes() ([]Node, error) {
	list, err := c.clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
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
