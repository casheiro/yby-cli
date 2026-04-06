//go:build k8s

package checks

import (
	"context"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNetpolDefaultDenyCheck_SemPolicies(t *testing.T) {
	client := fake.NewSimpleClientset()

	check := &NetpolDefaultDenyCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("esperava 2 findings (ingress + egress), obteve %d", len(findings))
	}
}

func TestNetpolDefaultDenyCheck_ComDefaultDenyIngress(t *testing.T) {
	client := fake.NewSimpleClientset(&networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "deny-ingress", Namespace: "default"},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			// Sem regras de ingress = deny all ingress
		},
	})

	check := &NetpolDefaultDenyCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	// Deve ter apenas o finding de egress
	if len(findings) != 1 {
		t.Fatalf("esperava 1 finding (apenas egress), obteve %d", len(findings))
	}
	if findings[0].CheckID != "NETPOL_DEFAULT_DENY" {
		t.Errorf("finding inesperado: %s", findings[0].CheckID)
	}
}

func TestNetpolDefaultDenyCheck_ComDefaultDenyAmbosTipos(t *testing.T) {
	client := fake.NewSimpleClientset(&networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "deny-all", Namespace: "default"},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
		},
	})

	check := &NetpolDefaultDenyCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("esperava zero findings para default deny ingress+egress, obteve %d", len(findings))
	}
}
