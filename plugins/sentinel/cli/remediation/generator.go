//go:build k8s

package remediation

import (
	"encoding/json"
	"fmt"

	"github.com/casheiro/yby-cli/plugins/sentinel/cli/checks"
)

// GeneratePatches gera patches de remediação baseados nos findings.
func GeneratePatches(findings []checks.SecurityFinding) []RemediationPatch {
	var patches []RemediationPatch

	for _, f := range findings {
		patch := generatePatch(f)
		if patch != nil {
			patches = append(patches, *patch)
		}
	}

	return patches
}

func generatePatch(f checks.SecurityFinding) *RemediationPatch {
	switch f.CheckID {
	case "POD_ROOT_CONTAINER":
		return patchRunAsNonRoot(f)
	case "POD_RESOURCE_LIMITS":
		return patchResourceLimits(f)
	case "POD_PRIVILEGED":
		return patchPrivileged(f)
	case "POD_PRIVILEGE_ESCALATION":
		return patchPrivilegeEscalation(f)
	case "POD_READONLY_ROOTFS":
		return patchReadOnlyRootfs(f)
	case "POD_CAPABILITIES":
		if f.Severity == checks.SeverityCritical {
			return nil // SYS_ADMIN — remoção requer análise manual
		}
		return patchDropAllCapabilities(f)
	case "POD_SERVICE_ACCOUNT_TOKEN":
		return patchAutomountToken(f)
	default:
		return nil
	}
}

func patchRunAsNonRoot(f checks.SecurityFinding) *RemediationPatch {
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"containers": []map[string]interface{}{{
				"name": f.Container,
				"securityContext": map[string]interface{}{
					"runAsNonRoot": true,
				},
			}},
		},
	}
	patchJSON, _ := json.Marshal(patch)
	return &RemediationPatch{
		ResourceKind: "Pod",
		ResourceName: f.Pod,
		Namespace:    f.Namespace,
		PatchType:    "strategic-merge",
		Patch:        string(patchJSON),
		Description:  fmt.Sprintf("Define runAsNonRoot: true para container '%s' no pod '%s'", f.Container, f.Pod),
	}
}

func patchResourceLimits(f checks.SecurityFinding) *RemediationPatch {
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"containers": []map[string]interface{}{{
				"name": f.Container,
				"resources": map[string]interface{}{
					"limits": map[string]string{
						"cpu":    "500m",
						"memory": "256Mi",
					},
					"requests": map[string]string{
						"cpu":    "100m",
						"memory": "128Mi",
					},
				},
			}},
		},
	}
	patchJSON, _ := json.Marshal(patch)
	return &RemediationPatch{
		ResourceKind: "Pod",
		ResourceName: f.Pod,
		Namespace:    f.Namespace,
		PatchType:    "strategic-merge",
		Patch:        string(patchJSON),
		Description:  fmt.Sprintf("Define limites padrão de recursos para container '%s' no pod '%s'", f.Container, f.Pod),
	}
}

func patchPrivileged(f checks.SecurityFinding) *RemediationPatch {
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"containers": []map[string]interface{}{{
				"name": f.Container,
				"securityContext": map[string]interface{}{
					"privileged": false,
				},
			}},
		},
	}
	patchJSON, _ := json.Marshal(patch)
	return &RemediationPatch{
		ResourceKind: "Pod",
		ResourceName: f.Pod,
		Namespace:    f.Namespace,
		PatchType:    "strategic-merge",
		Patch:        string(patchJSON),
		Description:  fmt.Sprintf("Remove modo privilegiado do container '%s' no pod '%s'", f.Container, f.Pod),
	}
}

func patchPrivilegeEscalation(f checks.SecurityFinding) *RemediationPatch {
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"containers": []map[string]interface{}{{
				"name": f.Container,
				"securityContext": map[string]interface{}{
					"allowPrivilegeEscalation": false,
				},
			}},
		},
	}
	patchJSON, _ := json.Marshal(patch)
	return &RemediationPatch{
		ResourceKind: "Pod",
		ResourceName: f.Pod,
		Namespace:    f.Namespace,
		PatchType:    "strategic-merge",
		Patch:        string(patchJSON),
		Description:  fmt.Sprintf("Bloqueia escalação de privilégios no container '%s' do pod '%s'", f.Container, f.Pod),
	}
}

func patchReadOnlyRootfs(f checks.SecurityFinding) *RemediationPatch {
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"containers": []map[string]interface{}{{
				"name": f.Container,
				"securityContext": map[string]interface{}{
					"readOnlyRootFilesystem": true,
				},
			}},
		},
	}
	patchJSON, _ := json.Marshal(patch)
	return &RemediationPatch{
		ResourceKind: "Pod",
		ResourceName: f.Pod,
		Namespace:    f.Namespace,
		PatchType:    "strategic-merge",
		Patch:        string(patchJSON),
		Description:  fmt.Sprintf("Define readOnlyRootFilesystem: true no container '%s' do pod '%s'", f.Container, f.Pod),
	}
}

func patchDropAllCapabilities(f checks.SecurityFinding) *RemediationPatch {
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"containers": []map[string]interface{}{{
				"name": f.Container,
				"securityContext": map[string]interface{}{
					"capabilities": map[string]interface{}{
						"drop": []string{"ALL"},
					},
				},
			}},
		},
	}
	patchJSON, _ := json.Marshal(patch)
	return &RemediationPatch{
		ResourceKind: "Pod",
		ResourceName: f.Pod,
		Namespace:    f.Namespace,
		PatchType:    "strategic-merge",
		Patch:        string(patchJSON),
		Description:  fmt.Sprintf("Adiciona drop ALL capabilities no container '%s' do pod '%s'", f.Container, f.Pod),
	}
}

func patchAutomountToken(f checks.SecurityFinding) *RemediationPatch {
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"automountServiceAccountToken": false,
		},
	}
	patchJSON, _ := json.Marshal(patch)
	return &RemediationPatch{
		ResourceKind: "Pod",
		ResourceName: f.Pod,
		Namespace:    f.Namespace,
		PatchType:    "strategic-merge",
		Patch:        string(patchJSON),
		Description:  fmt.Sprintf("Desabilita automount do service account token no pod '%s'", f.Pod),
	}
}
