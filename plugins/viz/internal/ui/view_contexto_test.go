package ui

import "testing"

// TestNomeDoCluster: o masthead mostra o nome do cluster, não o ARN do EKS nem o
// gke_<projeto>_<local>_<nome> do GKE (folha VizPods: "staging ▸ payments").
func TestNomeDoCluster(t *testing.T) {
	for contexto, esperado := range map[string]string{
		"arn:aws:eks:us-east-1:482100009310:cluster/staging": "staging",
		"gke_ml-plataforma_us-central1_gke-ml":               "gke-ml",
		"aks-dados":                                          "aks-dados",
		"k3d-yby-local":                                      "k3d-yby-local",
		"gke_só_dois":                                        "gke_só_dois",
		"arn:aws:iam::1:role/x":                              "arn:aws:iam::1:role/x",
	} {
		if obtido := nomeDoCluster(contexto); obtido != esperado {
			t.Errorf("nomeDoCluster(%q) = %q, esperado %q", contexto, obtido, esperado)
		}
	}
}
