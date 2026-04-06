package monitor

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// DeleteResource deleta um recurso Kubernetes pelo kind, nome e namespace
func DeleteResource(clientset kubernetes.Interface, kind, name, namespace string) error {
	ctx := context.Background()

	switch kind {
	case "pod":
		return clientset.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "deployment":
		return clientset.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "service":
		return clientset.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "statefulset":
		return clientset.AppsV1().StatefulSets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "job":
		return clientset.BatchV1().Jobs(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "configmap":
		return clientset.CoreV1().ConfigMaps(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	default:
		return fmt.Errorf("deleção não suportada para tipo '%s'", kind)
	}
}

// ScaleDeployment escala um deployment para o número de réplicas desejado
func ScaleDeployment(clientset kubernetes.Interface, name, namespace string, replicas int32) error {
	ctx := context.Background()

	scale, err := clientset.AppsV1().Deployments(namespace).GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("falha ao obter scale do deployment '%s': %w", name, err)
	}

	scale.Spec.Replicas = replicas
	_, err = clientset.AppsV1().Deployments(namespace).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("falha ao escalar deployment '%s': %w", name, err)
	}

	return nil
}

// RestartDeployment realiza um rollout restart de um deployment
// (adiciona/atualiza a annotation kubectl.kubernetes.io/restartedAt)
func RestartDeployment(clientset kubernetes.Interface, name, namespace string) error {
	ctx := context.Background()

	patch := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`,
		time.Now().Format(time.RFC3339))

	_, err := clientset.AppsV1().Deployments(namespace).Patch(
		ctx, name, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("falha ao reiniciar deployment '%s': %w", name, err)
	}

	return nil
}
