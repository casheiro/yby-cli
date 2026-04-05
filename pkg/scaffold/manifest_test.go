package scaffold

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndLoadProjectManifest_RoundTrip(t *testing.T) {
	dir := t.TempDir()

	ctx := &BlueprintContext{
		ProjectName:         "meu-projeto",
		Topology:            "standard",
		WorkflowPattern:     "gitflow",
		SecretsStrategy:     "sops",
		EnableKepler:        true,
		EnableMinio:         true,
		EnableKEDA:          false,
		EnableMetricsServer: false,
		EnableDevContainer:  true,
		EnableCI:            true,
		EnableDiscovery:     false,
		GitRepo:             "https://github.com/org/repo.git",
		GitBranch:           "main",
		Domain:              "cluster.com",
		Email:               "admin@cluster.com",
	}

	err := SaveProjectManifest(dir, ctx)
	require.NoError(t, err)

	// Verificar que o arquivo foi criado
	_, err = os.Stat(filepath.Join(dir, ".yby", "project.yaml"))
	require.NoError(t, err)

	// Carregar de volta
	manifest, err := LoadProjectManifest(dir)
	require.NoError(t, err)

	assert.Equal(t, "yby/v1", manifest.APIVersion)
	assert.Equal(t, "ProjectManifest", manifest.Kind)
	assert.Equal(t, "meu-projeto", manifest.Metadata.Name)
	assert.NotEmpty(t, manifest.Metadata.CreatedAt)

	assert.Equal(t, "standard", manifest.Spec.Topology)
	assert.Equal(t, "gitflow", manifest.Spec.Workflow)
	assert.Equal(t, "sops", manifest.Spec.SecretsStrategy)
	assert.Equal(t, "https://github.com/org/repo.git", manifest.Spec.Git.Repo)
	assert.Equal(t, "main", manifest.Spec.Git.Branch)
	assert.Equal(t, "cluster.com", manifest.Spec.Domain)
	assert.Equal(t, "admin@cluster.com", manifest.Spec.Email)

	assert.True(t, manifest.Spec.Features.Kepler)
	assert.True(t, manifest.Spec.Features.Minio)
	assert.False(t, manifest.Spec.Features.KEDA)
	assert.True(t, manifest.Spec.Features.DevContainer)
	assert.True(t, manifest.Spec.Features.CI)
	assert.False(t, manifest.Spec.Features.Discovery)
}

func TestLoadProjectManifest_ArquivoInexistente(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadProjectManifest(dir)
	assert.Error(t, err)
}

func TestManifestToContext(t *testing.T) {
	manifest := &ProjectManifest{
		Metadata: ProjectMetadata{Name: "teste"},
		Spec: ProjectSpec{
			Topology:        "complete",
			Workflow:        "trunkbased",
			SecretsStrategy: "external-secrets",
			Features: FeatureFlags{
				Kepler:    true,
				Minio:     false,
				KEDA:      true,
				CI:        true,
				Discovery: true,
			},
			Git:    GitSpec{Repo: "https://github.com/x/y.git", Branch: "develop"},
			Domain: "example.com",
			Email:  "test@example.com",
		},
	}

	ctx := ManifestToContext(manifest)

	assert.Equal(t, "teste", ctx.ProjectName)
	assert.Equal(t, "complete", ctx.Topology)
	assert.Equal(t, "trunkbased", ctx.WorkflowPattern)
	assert.Equal(t, "external-secrets", ctx.SecretsStrategy)
	assert.True(t, ctx.EnableKepler)
	assert.False(t, ctx.EnableMinio)
	assert.True(t, ctx.EnableKEDA)
	assert.True(t, ctx.EnableCI)
	assert.True(t, ctx.EnableDiscovery)
	assert.Equal(t, "https://github.com/x/y.git", ctx.GitRepo)
	assert.Equal(t, "develop", ctx.GitBranch)
	assert.Equal(t, "example.com", ctx.Domain)
	assert.Equal(t, "test@example.com", ctx.Email)
}

func TestMergeContextDefaults_CamposVaziosPreenchidos(t *testing.T) {
	target := &BlueprintContext{
		ProjectName: "novo",
		GitRepo:     "https://github.com/novo/repo.git",
	}
	defaults := &BlueprintContext{
		ProjectName:     "antigo",
		Topology:        "standard",
		WorkflowPattern: "gitflow",
		SecretsStrategy: "sops",
		GitRepo:         "https://github.com/antigo/repo.git",
		GitBranch:       "main",
		Domain:          "old.com",
		Email:           "old@old.com",
		EnableKepler:    true,
		EnableMinio:     true,
	}

	MergeContextDefaults(target, defaults)

	// Campos que target já tinha — mantém
	assert.Equal(t, "novo", target.ProjectName)
	assert.Equal(t, "https://github.com/novo/repo.git", target.GitRepo)

	// Campos vazios — preenche do defaults
	assert.Equal(t, "standard", target.Topology)
	assert.Equal(t, "gitflow", target.WorkflowPattern)
	assert.Equal(t, "sops", target.SecretsStrategy)
	assert.Equal(t, "main", target.GitBranch)
	assert.Equal(t, "old.com", target.Domain)
	assert.Equal(t, "old@old.com", target.Email)

	// Features — defaults true propagam
	assert.True(t, target.EnableKepler)
	assert.True(t, target.EnableMinio)
}

func TestMergeContextDefaults_NaoSobrescreve(t *testing.T) {
	target := &BlueprintContext{
		Topology:   "complete",
		Domain:     "new.com",
		EnableKEDA: true,
	}
	defaults := &BlueprintContext{
		Topology:   "single",
		Domain:     "old.com",
		EnableKEDA: false,
	}

	MergeContextDefaults(target, defaults)

	assert.Equal(t, "complete", target.Topology)
	assert.Equal(t, "new.com", target.Domain)
	assert.True(t, target.EnableKEDA)
}

func TestSaveProjectManifest_DiretorioInexistente(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "dir")
	ctx := &BlueprintContext{ProjectName: "teste"}

	err := SaveProjectManifest(dir, ctx)
	assert.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, ".yby", "project.yaml"))
	assert.NoError(t, err)
}
