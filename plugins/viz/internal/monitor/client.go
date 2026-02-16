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

type Pod struct {
	Name      string
	Namespace string
	Status    string
	CPU       string
}

type Client interface {
	GetPods() ([]Pod, error)
}

// K8sClient conecta ao cluster real
type K8sClient struct {
	clientset *kubernetes.Clientset
}

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

func (c *K8sClient) GetPods() ([]Pod, error) {
	// Lista pods de todos os namespaces
	list, err := c.clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
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
