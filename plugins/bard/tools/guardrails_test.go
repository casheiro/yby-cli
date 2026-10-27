package tools

import (
	"strings"
	"testing"
)

// O que vem da IA não pode transformar a ferramenta numa operação destrutiva.
func TestConferirGuardas_Bloqueia(t *testing.T) {
	casos := []struct{ modelo, linha string }{
		{"kubectl {{verbo}} {{recurso}} {{nome}}", "kubectl delete namespace production"},
		{"kubectl {{verbo}} {{recurso}} {{nome}}", "kubectl delete ns staging"},
		{"kubectl {{verbo}} {{alvo}}", "kubectl delete namespaces/production"},
		{"kubectl {{verbo}} {{recurso}} {{nome}}", "kubectl DELETE Namespace prod"},
		{"kubectl {{verbo}} {{recurso}} {{nome}}", "kubectl delete clusterrole admin"},
		{"kubectl {{verbo}} {{recurso}} {{nome}}", "kubectl delete clusterrolebinding cluster-admin"},
		{"kubectl {{verbo}} {{no}}", "kubectl drain node-1"},
		{"kubectl {{verbo}} {{no}}", "kubectl drain node-1 --ignore-daemonsets"},
		{"kubectl {{verbo}} {{no}}", "kubectl cordon node-1"},
		{"kubectl apply -f {{arquivo}} {{extra}}", "kubectl apply -f x.yaml --force"},
		{"kubectl scale deploy {{nome}} --replicas={{n}}", "kubectl scale deploy api --replicas=0"},
		// o valor que chega só pela env (sh -c com $YBY_PARAM_ACAO) também conta
		{`sh -c 'kubectl $YBY_PARAM_ACAO'`, "sh -c kubectl $YBY_PARAM_ACAO\ndelete namespace prod"},
	}
	for _, c := range casos {
		err := conferirGuardas(c.modelo, c.linha)
		if err == nil || !strings.Contains(err.Error(), "operação bloqueada") {
			t.Errorf("%q (modelo %q): esperava bloqueio, obteve %v", c.linha, c.modelo, err)
		}
	}
}

// Leitura passa; e a operação que o próprio YAML escolheu (o usuário confiou nele) também.
func TestConferirGuardas_Permite(t *testing.T) {
	casos := []struct{ modelo, linha string }{
		{"kubectl get {{recurso}}", "kubectl get pods"},
		{"kubectl logs {{pod}} --tail={{n}}", "kubectl logs nginx-abc --tail=100"},
		{"kubectl logs {{pod}}", "kubectl logs drain-worker-1"},
		{"kubectl logs {{pod}}", "kubectl logs cordon-api"},
		{"kubectl get {{recurso}}", "kubectl get namespaces"},
		{"kubectl scale deploy {{nome}} --replicas={{n}}", "kubectl scale deploy api --replicas=3"},
		{"kubectl scale deploy {{nome}} --replicas={{n}}", "kubectl scale deploy api --replicas=10"},
		{"kubectl cordon {{no}}", "kubectl cordon node-1"},
		{"kubectl delete namespace {{ns}}", "kubectl delete namespace preview-42"},
	}
	for _, c := range casos {
		if err := conferirGuardas(c.modelo, c.linha); err != nil {
			t.Errorf("%q (modelo %q): não deveria bloquear: %v", c.linha, c.modelo, err)
		}
	}
}
