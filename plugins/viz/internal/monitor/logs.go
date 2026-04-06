package monitor

import (
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// GetPodLogs busca as últimas linhas de log de um pod
func GetPodLogs(clientset kubernetes.Interface, namespace, podName string, tailLines int64) (string, error) {
	opts := &corev1.PodLogOptions{
		TailLines: &tailLines,
	}

	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, opts)
	stream, err := req.Stream(context.Background())
	if err != nil {
		return "", fmt.Errorf("falha ao obter logs do pod '%s': %w", podName, err)
	}
	defer stream.Close()

	data, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("falha ao ler logs do pod '%s': %w", podName, err)
	}

	if len(data) == 0 {
		return "(nenhum log disponível)", nil
	}

	return string(data), nil
}
