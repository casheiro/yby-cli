package plugin

import (
	"encoding/json"
	"testing"
)

// ---- PluginManifest Tests ----

func TestPluginManifest_JSONRoundtrip(t *testing.T) {
	m := PluginManifest{
		Name:        "atlas",
		Version:     "1.0.0",
		Description: "Atlas plugin",
		Hooks:       []string{"context", "command", "assets"},
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var m2 PluginManifest
	if err := json.Unmarshal(data, &m2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if m2.Name != m.Name {
		t.Errorf("Name mismatch: %s vs %s", m.Name, m2.Name)
	}
	if m2.Version != m.Version {
		t.Errorf("Version mismatch: %s vs %s", m.Version, m2.Version)
	}
	if len(m2.Hooks) != len(m.Hooks) {
		t.Errorf("Hooks mismatch: %v vs %v", m.Hooks, m2.Hooks)
	}
}

func TestPluginManifest_EmptyHooks(t *testing.T) {
	m := PluginManifest{Name: "test", Version: "1.0"}
	if len(m.Hooks) != 0 {
		t.Errorf("expected no hooks, got %v", m.Hooks)
	}
}

// ---- PluginRequest Tests ----

func TestPluginRequest_JSONRoundtrip(t *testing.T) {
	req := PluginRequest{
		Hook: "context",
		Args: []string{"arg1", "arg2"},
		Context: map[string]interface{}{
			"env": "local",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var req2 PluginRequest
	if err := json.Unmarshal(data, &req2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if req2.Hook != "context" {
		t.Errorf("Hook mismatch: %s vs %s", req.Hook, req2.Hook)
	}
}

// ---- PluginResponse Tests ----

func TestPluginResponse_WithError(t *testing.T) {
	resp := PluginResponse{Error: "something went wrong"}
	data, _ := json.Marshal(resp)

	var resp2 PluginResponse
	json.Unmarshal(data, &resp2)
	if resp2.Error != "something went wrong" {
		t.Errorf("Error mismatch: %q", resp2.Error)
	}
}

func TestPluginResponse_WithData(t *testing.T) {
	resp := PluginResponse{Data: map[string]string{"key": "value"}}
	data, _ := json.Marshal(resp)

	var resp2 PluginResponse
	json.Unmarshal(data, &resp2)
	if resp2.Error != "" {
		t.Errorf("expected empty error, got %q", resp2.Error)
	}
}

// ---- PluginFullContext Tests ----

func TestPluginFullContext_Fields(t *testing.T) {
	ctx := PluginFullContext{
		ProjectName: "yby-cli",
		Environment: "local",
		Infra: PluginInfrastructure{
			KubeConfig:  "~/.kube/config",
			KubeContext: "local-ctx",
			Namespace:   "default",
		},
		Values: map[string]interface{}{
			"replicas": 1,
		},
		Data: map[string]interface{}{
			"extra": "data",
		},
	}

	if ctx.ProjectName != "yby-cli" {
		t.Errorf("ProjectName mismatch")
	}
	if ctx.Infra.KubeContext != "local-ctx" {
		t.Errorf("KubeContext mismatch")
	}
}

func TestPluginFullContext_JSONRoundtrip(t *testing.T) {
	ctx := PluginFullContext{
		ProjectName: "test",
		Environment: "prod",
	}

	data, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var ctx2 PluginFullContext
	if err := json.Unmarshal(data, &ctx2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if ctx2.ProjectName != ctx.ProjectName {
		t.Errorf("ProjectName mismatch")
	}
}

// ---- ContextPatch and AssetsDefinition Tests ----

func TestContextPatch_Map(t *testing.T) {
	patch := ContextPatch{"env": "staging", "url": "http://example.com"}
	if patch["env"] != "staging" {
		t.Error("ContextPatch key access failed")
	}
}

func TestAssetsDefinition_Field(t *testing.T) {
	a := AssetsDefinition{Path: "/usr/local/lib/plugin/assets"}
	if a.Path != "/usr/local/lib/plugin/assets" {
		t.Error("AssetsDefinition.Path mismatch")
	}

	data, _ := json.Marshal(a)
	var a2 AssetsDefinition
	json.Unmarshal(data, &a2)
	if a2.Path != a.Path {
		t.Errorf("JSONRoundtrip failed: %s vs %s", a.Path, a2.Path)
	}
}

// ---- PluginInfrastructure Tests ----

func TestPluginInfrastructure_JSONRoundtrip(t *testing.T) {
	infra := PluginInfrastructure{
		KubeConfig:  "~/.kube/config",
		KubeContext: "my-cluster",
		Namespace:   "backend",
	}

	data, _ := json.Marshal(infra)
	var infra2 PluginInfrastructure
	json.Unmarshal(data, &infra2)

	if infra2.KubeConfig != infra.KubeConfig {
		t.Errorf("KubeConfig mismatch")
	}
	if infra2.KubeContext != infra.KubeContext {
		t.Errorf("KubeContext mismatch")
	}
	if infra2.Namespace != infra.Namespace {
		t.Errorf("Namespace mismatch")
	}
}
