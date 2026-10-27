//go:build k8s

package remediation

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// alvo é o recurso que recebe a correção: o dono de mais alto nível de um pod, com o
// template dos pods dele.
type alvo struct {
	kind, nome, namespace string
	meta                  metav1.Object
	spec                  *corev1.PodSpec
}

func (a alvo) recurso() string { return a.kind + "/" + a.nome }

// naoCorrigivel é o recurso que o --fix não corrige, com o motivo para a tela.
type naoCorrigivel struct{ motivo string }

func (n naoCorrigivel) Error() string { return n.motivo }

// maxDonos limita a subida pelos ownerReferences (Pod → Job → CronJob são três níveis).
const maxDonos = 4

// donoDe sobe pelos ownerReferences (o controller de cada nível) a partir de kind/nome até
// o recurso de mais alto nível, e o devolve quando o template dele chega aos pods:
// Deployment, StatefulSet, DaemonSet ou CronJob. Pod sem dono, ReplicaSet sem Deployment,
// Job sem CronJob e donos que o --fix não conhece (um operador, um Rollout) voltam como
// naoCorrigivel; uma falha de leitura volta como o erro do cliente.
func donoDe(ctx context.Context, client kubernetes.Interface, namespace, kind, nome string) (alvo, error) {
	for range maxDonos {
		a, err := lerAlvo(ctx, client, namespace, kind, nome)
		if err != nil {
			return alvo{}, err
		}
		if dono := metav1.GetControllerOf(a.meta); dono != nil {
			kind, nome = dono.Kind, dono.Name
			continue
		}
		switch kind {
		case "Deployment", "StatefulSet", "DaemonSet", "CronJob":
			return a, nil
		case "Pod":
			return alvo{}, naoCorrigivel{"o pod não tem dono, e o spec de um pod em execução não muda: corrija o manifesto e recrie o pod"}
		case "ReplicaSet":
			return alvo{}, naoCorrigivel{"o ReplicaSet não tem Deployment, e o template novo não chega aos pods que já existem: corrija o manifesto"}
		default: // Job
			return alvo{}, naoCorrigivel{"o Job não tem CronJob, e o template de um Job não muda depois de criado: corrija o manifesto"}
		}
	}
	return alvo{}, naoCorrigivel{"a cadeia de donos do recurso é longa demais"}
}

// lerAlvo lê o recurso e aponta o template dos pods dele.
func lerAlvo(ctx context.Context, client kubernetes.Interface, namespace, kind, nome string) (alvo, error) {
	a := alvo{kind: kind, nome: nome, namespace: namespace}
	opts := metav1.GetOptions{}
	switch kind {
	case "Pod":
		o, err := client.CoreV1().Pods(namespace).Get(ctx, nome, opts)
		if err != nil {
			return alvo{}, err
		}
		a.meta, a.spec = o, &o.Spec
	case "ReplicaSet":
		o, err := client.AppsV1().ReplicaSets(namespace).Get(ctx, nome, opts)
		if err != nil {
			return alvo{}, err
		}
		a.meta, a.spec = o, &o.Spec.Template.Spec
	case "Deployment":
		o, err := client.AppsV1().Deployments(namespace).Get(ctx, nome, opts)
		if err != nil {
			return alvo{}, err
		}
		a.meta, a.spec = o, &o.Spec.Template.Spec
	case "StatefulSet":
		o, err := client.AppsV1().StatefulSets(namespace).Get(ctx, nome, opts)
		if err != nil {
			return alvo{}, err
		}
		a.meta, a.spec = o, &o.Spec.Template.Spec
	case "DaemonSet":
		o, err := client.AppsV1().DaemonSets(namespace).Get(ctx, nome, opts)
		if err != nil {
			return alvo{}, err
		}
		a.meta, a.spec = o, &o.Spec.Template.Spec
	case "Job":
		o, err := client.BatchV1().Jobs(namespace).Get(ctx, nome, opts)
		if err != nil {
			return alvo{}, err
		}
		a.meta, a.spec = o, &o.Spec.Template.Spec
	case "CronJob":
		o, err := client.BatchV1().CronJobs(namespace).Get(ctx, nome, opts)
		if err != nil {
			return alvo{}, err
		}
		a.meta, a.spec = o, &o.Spec.JobTemplate.Spec.Template.Spec
	default:
		return alvo{}, naoCorrigivel{fmt.Sprintf("o --fix não corrige %s: corrija o manifesto", kind)}
	}
	return a, nil
}

// Marcas de que o Argo CD gere o recurso: a anotação do rastreio por anotação (o padrão
// do Argo CD 3) e o rótulo que o cluster-config do yby usa (application.instanceLabelKey).
const (
	anotacaoDoArgo = "argocd.argoproj.io/tracking-id"
	rotuloDoArgo   = "argocd.argoproj.io/instance"
)

// geridoPeloArgo diz se o Argo CD gere o recurso e, se gere, o nome da Application.
func geridoPeloArgo(m metav1.Object) (app string, gerido bool) {
	if v := m.GetAnnotations()[anotacaoDoArgo]; v != "" {
		app, _, _ = strings.Cut(v, ":")
		return app, true
	}
	if v := m.GetLabels()[rotuloDoArgo]; v != "" {
		return v, true
	}
	return "", false
}

// motivoDaLeitura diz em PT-BR por que o recurso não pôde ser lido.
func motivoDaLeitura(err error) string {
	switch {
	case apierrors.IsNotFound(err):
		return "o recurso não existe mais no cluster"
	case apierrors.IsForbidden(err):
		return "sem permissão para ler o recurso e os donos dele"
	}
	return "não deu para ler o recurso: " + err.Error()
}
