package tools

import (
	"testing"
)

// TestValidateToolCall_OperacoesSeguras verifica que operações read-only passam.
func TestValidateToolCall_OperacoesSeguras(t *testing.T) {
	safeCalls := []ToolCall{
		{Name: "kubectl_get", Params: map[string]string{"resource": "pods"}},
		{Name: "kubectl_get", Params: map[string]string{"resource": "services", "namespace": "default"}},
		{Name: "kubectl_logs", Params: map[string]string{"pod": "nginx-abc", "tail": "100"}},
		{Name: "kubectl_events", Params: map[string]string{"namespace": "kube-system"}},
		{Name: "kubectl_describe", Params: map[string]string{"resource": "pod", "name": "nginx"}},
	}

	for _, call := range safeCalls {
		if err := ValidateToolCall(call); err != nil {
			t.Errorf("operação segura '%s' bloqueada: %v", call.Name, err)
		}
	}
}

// TestValidateToolCall_DeleteNamespace verifica bloqueio de exclusão de namespace.
func TestValidateToolCall_DeleteNamespace(t *testing.T) {
	call := ToolCall{
		Name:   "kubectl_exec",
		Params: map[string]string{"command": "delete namespace production"},
	}
	if err := ValidateToolCall(call); err == nil {
		t.Error("esperava bloqueio para delete namespace")
	}
}

// TestValidateToolCall_DeleteNS verifica bloqueio com abreviação 'ns'.
func TestValidateToolCall_DeleteNS(t *testing.T) {
	call := ToolCall{
		Name:   "kubectl_exec",
		Params: map[string]string{"command": "delete ns staging"},
	}
	if err := ValidateToolCall(call); err == nil {
		t.Error("esperava bloqueio para delete ns")
	}
}

// TestValidateToolCall_DeleteClusterRole verifica bloqueio de exclusão de ClusterRole.
func TestValidateToolCall_DeleteClusterRole(t *testing.T) {
	call := ToolCall{
		Name:   "kubectl_exec",
		Params: map[string]string{"command": "delete clusterrole admin"},
	}
	if err := ValidateToolCall(call); err == nil {
		t.Error("esperava bloqueio para delete clusterrole")
	}
}

// TestValidateToolCall_DeleteClusterRoleBinding verifica bloqueio.
func TestValidateToolCall_DeleteClusterRoleBinding(t *testing.T) {
	call := ToolCall{
		Name:   "kubectl_exec",
		Params: map[string]string{"command": "delete clusterrolebinding admin-binding"},
	}
	if err := ValidateToolCall(call); err == nil {
		t.Error("esperava bloqueio para delete clusterrolebinding")
	}
}

// TestValidateToolCall_DrainSemIgnore verifica bloqueio de drain sem --ignore-daemonsets.
func TestValidateToolCall_DrainSemIgnore(t *testing.T) {
	call := ToolCall{
		Name:   "kubectl_exec",
		Params: map[string]string{"command": "drain node worker-1"},
	}
	if err := ValidateToolCall(call); err == nil {
		t.Error("esperava bloqueio para drain sem --ignore-daemonsets")
	}
}

// TestValidateToolCall_Cordon verifica bloqueio de cordon.
func TestValidateToolCall_Cordon(t *testing.T) {
	call := ToolCall{
		Name:   "kubectl_exec",
		Params: map[string]string{"command": "cordon worker-1"},
	}
	if err := ValidateToolCall(call); err == nil {
		t.Error("esperava bloqueio para cordon")
	}
}

// TestValidateToolCall_ApplyForce verifica bloqueio de apply --force.
func TestValidateToolCall_ApplyForce(t *testing.T) {
	call := ToolCall{
		Name:   "kubectl_exec",
		Params: map[string]string{"command": "apply -f deploy.yaml --force"},
	}
	if err := ValidateToolCall(call); err == nil {
		t.Error("esperava bloqueio para apply --force")
	}
}

// TestValidateToolCall_ScaleZero verifica bloqueio de scale para 0 replicas.
func TestValidateToolCall_ScaleZero(t *testing.T) {
	call := ToolCall{
		Name:   "kubectl_exec",
		Params: map[string]string{"command": "scale deployment nginx replicas=0"},
	}
	if err := ValidateToolCall(call); err == nil {
		t.Error("esperava bloqueio para scale replicas=0")
	}
}

// TestValidateToolCall_CaseInsensitive verifica que a validação é case-insensitive.
func TestValidateToolCall_CaseInsensitive(t *testing.T) {
	call := ToolCall{
		Name:   "kubectl_exec",
		Params: map[string]string{"command": "DELETE NAMESPACE production"},
	}
	if err := ValidateToolCall(call); err == nil {
		t.Error("esperava bloqueio independente de case")
	}
}
