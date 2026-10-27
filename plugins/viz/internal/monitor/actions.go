package monitor

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// PrazoDaAcao é quanto uma ação espera pela API: sem resposta nesse tempo, ela
// falha com context.DeadlineExceeded e a tela diz que o cluster não respondeu.
const PrazoDaAcao = 30 * time.Second

// contextoDaAcao é o contexto de uma ação, com o PrazoDaAcao.
func contextoDaAcao() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), PrazoDaAcao)
}

// opcoesDeExclusao monta as opções da exclusão: propagação em segundo plano
// (o batch/v1 deixaria os pods do Job órfãos por padrão) e, havendo UID, a
// precondição que recusa apagar um objeto recriado com o mesmo nome. UID vazio
// não vira precondição.
func opcoesDeExclusao(uid string) metav1.DeleteOptions {
	politica := metav1.DeletePropagationBackground
	op := metav1.DeleteOptions{PropagationPolicy: &politica}
	if uid != "" {
		u := types.UID(uid)
		op.Preconditions = &metav1.Preconditions{UID: &u}
	}
	return op
}

// DeleteResource deleta um recurso Kubernetes pelo kind, nome e namespace. Com
// uid, só apaga o objeto que a pessoa viu; se ele foi recriado, a API responde
// conflito.
func DeleteResource(clientset kubernetes.Interface, kind, name, namespace, uid string) error {
	ctx, cancelar := contextoDaAcao()
	defer cancelar()
	op := opcoesDeExclusao(uid)

	switch kind {
	case "pod":
		return clientset.CoreV1().Pods(namespace).Delete(ctx, name, op)
	case "deployment":
		return clientset.AppsV1().Deployments(namespace).Delete(ctx, name, op)
	case "service":
		return clientset.CoreV1().Services(namespace).Delete(ctx, name, op)
	case "statefulset":
		return clientset.AppsV1().StatefulSets(namespace).Delete(ctx, name, op)
	case "job":
		return clientset.BatchV1().Jobs(namespace).Delete(ctx, name, op)
	case "configmap":
		return clientset.CoreV1().ConfigMaps(namespace).Delete(ctx, name, op)
	case "ingress":
		return clientset.NetworkingV1().Ingresses(namespace).Delete(ctx, name, op)
	default:
		return fmt.Errorf("deleção não suportada para tipo '%s'", kind)
	}
}

// ScaleDeployment escala um deployment para o número de réplicas desejado
func ScaleDeployment(clientset kubernetes.Interface, name, namespace string, replicas int32) error {
	ctx, cancelar := contextoDaAcao()
	defer cancelar()

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
	ctx, cancelar := contextoDaAcao()
	defer cancelar()

	patch := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`,
		time.Now().Format(time.RFC3339))

	_, err := clientset.AppsV1().Deployments(namespace).Patch(
		ctx, name, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("falha ao reiniciar deployment '%s': %w", name, err)
	}

	return nil
}

// ScaleStatefulSet escala um statefulset para o número de réplicas desejado
func ScaleStatefulSet(clientset kubernetes.Interface, name, namespace string, replicas int32) error {
	ctx, cancelar := contextoDaAcao()
	defer cancelar()

	scale, err := clientset.AppsV1().StatefulSets(namespace).GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("falha ao obter scale do statefulset '%s': %w", name, err)
	}

	scale.Spec.Replicas = replicas
	_, err = clientset.AppsV1().StatefulSets(namespace).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("falha ao escalar statefulset '%s': %w", name, err)
	}

	return nil
}

// RestartStatefulSet realiza um rollout restart de um statefulset
// (adiciona/atualiza a annotation kubectl.kubernetes.io/restartedAt)
func RestartStatefulSet(clientset kubernetes.Interface, name, namespace string) error {
	ctx, cancelar := contextoDaAcao()
	defer cancelar()

	patch := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`,
		time.Now().Format(time.RFC3339))

	_, err := clientset.AppsV1().StatefulSets(namespace).Patch(
		ctx, name, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("falha ao reiniciar statefulset '%s': %w", name, err)
	}

	return nil
}
