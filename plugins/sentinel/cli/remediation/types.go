//go:build k8s

// Package remediation gera e aplica patches de remediação baseados em findings do Sentinel.
package remediation

// RemediationPatch representa um patch de remediação para um recurso K8s.
type RemediationPatch struct {
	ResourceKind string `json:"resource_kind"` // "Pod", "Deployment", etc.
	ResourceName string `json:"resource_name"`
	Namespace    string `json:"namespace"`
	PatchType    string `json:"patch_type"` // "strategic-merge" ou "json-patch"
	Patch        string `json:"patch"`      // JSON do patch
	Description  string `json:"description"`
}
