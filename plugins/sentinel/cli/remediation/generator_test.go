//go:build k8s

package remediation

import (
	"encoding/json"
	"testing"

	"github.com/casheiro/yby-cli/plugins/sentinel/cli/checks"
)

func TestGeneratePatches_RootContainer(t *testing.T) {
	findings := []checks.SecurityFinding{{
		CheckID:   "POD_ROOT_CONTAINER",
		Pod:       "pod-root",
		Container: "app",
		Namespace: "default",
	}}

	patches := GeneratePatches(findings)
	if len(patches) != 1 {
		t.Fatalf("esperava 1 patch, obteve %d", len(patches))
	}
	if patches[0].ResourceKind != "Pod" {
		t.Errorf("esperava ResourceKind=Pod, obteve %s", patches[0].ResourceKind)
	}
	if patches[0].PatchType != "strategic-merge" {
		t.Errorf("esperava PatchType=strategic-merge, obteve %s", patches[0].PatchType)
	}

	var patchData map[string]interface{}
	if err := json.Unmarshal([]byte(patches[0].Patch), &patchData); err != nil {
		t.Fatalf("patch JSON inválido: %v", err)
	}
}

func TestGeneratePatches_ResourceLimits(t *testing.T) {
	findings := []checks.SecurityFinding{{
		CheckID:   "POD_RESOURCE_LIMITS",
		Pod:       "pod-nolimits",
		Container: "app",
		Namespace: "default",
	}}

	patches := GeneratePatches(findings)
	if len(patches) != 1 {
		t.Fatalf("esperava 1 patch, obteve %d", len(patches))
	}
	if patches[0].ResourceName != "pod-nolimits" {
		t.Errorf("esperava ResourceName=pod-nolimits, obteve %s", patches[0].ResourceName)
	}
}

func TestGeneratePatches_Privileged(t *testing.T) {
	findings := []checks.SecurityFinding{{
		CheckID:   "POD_PRIVILEGED",
		Pod:       "pod-priv",
		Container: "app",
		Namespace: "default",
	}}

	patches := GeneratePatches(findings)
	if len(patches) != 1 {
		t.Fatalf("esperava 1 patch, obteve %d", len(patches))
	}
}

func TestGeneratePatches_SemPatchDisponivel(t *testing.T) {
	findings := []checks.SecurityFinding{{
		CheckID:   "RBAC_CLUSTER_ADMIN",
		Pod:       "",
		Namespace: "default",
	}}

	patches := GeneratePatches(findings)
	if len(patches) != 0 {
		t.Errorf("esperava 0 patches para RBAC_CLUSTER_ADMIN, obteve %d", len(patches))
	}
}

func TestGeneratePatches_SysAdminCapability(t *testing.T) {
	findings := []checks.SecurityFinding{{
		CheckID:   "POD_CAPABILITIES",
		Severity:  checks.SeverityCritical,
		Pod:       "pod-sysadmin",
		Container: "app",
		Namespace: "default",
	}}

	patches := GeneratePatches(findings)
	if len(patches) != 0 {
		t.Errorf("esperava 0 patches para SYS_ADMIN (requer análise manual), obteve %d", len(patches))
	}
}

func TestGeneratePatches_DropAllCapabilities(t *testing.T) {
	findings := []checks.SecurityFinding{{
		CheckID:   "POD_CAPABILITIES",
		Severity:  checks.SeverityMedium,
		Pod:       "pod-nocap",
		Container: "app",
		Namespace: "default",
	}}

	patches := GeneratePatches(findings)
	if len(patches) != 1 {
		t.Fatalf("esperava 1 patch para drop ALL, obteve %d", len(patches))
	}
}

func TestGeneratePatches_MultiplosFindings(t *testing.T) {
	findings := []checks.SecurityFinding{
		{CheckID: "POD_ROOT_CONTAINER", Pod: "pod-1", Container: "app", Namespace: "default"},
		{CheckID: "POD_PRIVILEGED", Pod: "pod-2", Container: "app", Namespace: "default"},
		{CheckID: "NETPOL_COVERAGE", Pod: "pod-3", Namespace: "default"},
	}

	patches := GeneratePatches(findings)
	if len(patches) != 2 {
		t.Errorf("esperava 2 patches (NETPOL_COVERAGE sem patch), obteve %d", len(patches))
	}
}
