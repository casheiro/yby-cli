package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helperWriteFile cria um arquivo com conteúdo no diretório especificado.
func helperWriteFile(t *testing.T, dir, relPath, content string) {
	t.Helper()
	fullPath := filepath.Join(dir, relPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
	require.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
}

// helperHash retorna o hash SHA-256 de um conteúdo string.
func helperHash(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "tmp")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	h, err := ComputeFileHash(path)
	require.NoError(t, err)
	return h
}

func TestComputeMergePlan_NoChanges(t *testing.T) {
	diskDir := t.TempDir()
	newDir := t.TempDir()

	content := "conteúdo original"
	helperWriteFile(t, diskDir, "arquivo.txt", content)
	helperWriteFile(t, newDir, "arquivo.txt", content)

	hash := helperHash(t, content)
	manifestHashes := map[string]string{"arquivo.txt": hash}

	plan, err := ComputeMergePlan(manifestHashes, diskDir, newDir)
	require.NoError(t, err)
	require.Len(t, plan.Entries, 1)
	assert.Equal(t, ActionNone, plan.Entries[0].Action)
}

func TestComputeMergePlan_ScaffoldChanged(t *testing.T) {
	diskDir := t.TempDir()
	newDir := t.TempDir()

	originalContent := "conteúdo original"
	newContent := "conteúdo atualizado pelo scaffold"

	helperWriteFile(t, diskDir, "arquivo.txt", originalContent)
	helperWriteFile(t, newDir, "arquivo.txt", newContent)

	originalHash := helperHash(t, originalContent)
	manifestHashes := map[string]string{"arquivo.txt": originalHash}

	plan, err := ComputeMergePlan(manifestHashes, diskDir, newDir)
	require.NoError(t, err)
	require.Len(t, plan.Entries, 1)
	assert.Equal(t, ActionUpdate, plan.Entries[0].Action)
}

func TestComputeMergePlan_UserChanged(t *testing.T) {
	diskDir := t.TempDir()
	newDir := t.TempDir()

	originalContent := "conteúdo original"
	userContent := "conteúdo modificado pelo usuário"

	helperWriteFile(t, diskDir, "arquivo.txt", userContent)
	helperWriteFile(t, newDir, "arquivo.txt", originalContent)

	originalHash := helperHash(t, originalContent)
	manifestHashes := map[string]string{"arquivo.txt": originalHash}

	plan, err := ComputeMergePlan(manifestHashes, diskDir, newDir)
	require.NoError(t, err)
	require.Len(t, plan.Entries, 1)
	assert.Equal(t, ActionPreserve, plan.Entries[0].Action)
}

func TestComputeMergePlan_BothChanged(t *testing.T) {
	diskDir := t.TempDir()
	newDir := t.TempDir()

	originalContent := "conteúdo original"
	userContent := "conteúdo do usuário"
	scaffoldContent := "conteúdo do scaffold"

	helperWriteFile(t, diskDir, "arquivo.txt", userContent)
	helperWriteFile(t, newDir, "arquivo.txt", scaffoldContent)

	originalHash := helperHash(t, originalContent)
	manifestHashes := map[string]string{"arquivo.txt": originalHash}

	plan, err := ComputeMergePlan(manifestHashes, diskDir, newDir)
	require.NoError(t, err)
	require.Len(t, plan.Entries, 1)
	assert.Equal(t, ActionConflict, plan.Entries[0].Action)
}

func TestComputeMergePlan_NewFile(t *testing.T) {
	diskDir := t.TempDir()
	newDir := t.TempDir()

	helperWriteFile(t, newDir, "novo.txt", "arquivo novo")

	manifestHashes := map[string]string{} // nenhum arquivo anterior

	plan, err := ComputeMergePlan(manifestHashes, diskDir, newDir)
	require.NoError(t, err)
	require.Len(t, plan.Entries, 1)
	assert.Equal(t, ActionNew, plan.Entries[0].Action)
	assert.Equal(t, "novo.txt", plan.Entries[0].RelPath)
}

func TestApplyMergePlan_Update(t *testing.T) {
	diskDir := t.TempDir()
	newDir := t.TempDir()

	helperWriteFile(t, diskDir, "arquivo.txt", "antigo")
	helperWriteFile(t, newDir, "arquivo.txt", "novo")

	plan := &MergePlan{
		Entries: []MergeEntry{
			{RelPath: "arquivo.txt", Action: ActionUpdate},
		},
	}

	err := ApplyMergePlan(plan, diskDir, newDir, nil)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(diskDir, "arquivo.txt"))
	require.NoError(t, err)
	assert.Equal(t, "novo", string(data))
}

func TestApplyMergePlan_Preserve(t *testing.T) {
	diskDir := t.TempDir()
	newDir := t.TempDir()

	helperWriteFile(t, diskDir, "arquivo.txt", "conteúdo do usuário")
	helperWriteFile(t, newDir, "arquivo.txt", "conteúdo do scaffold")

	plan := &MergePlan{
		Entries: []MergeEntry{
			{RelPath: "arquivo.txt", Action: ActionPreserve},
		},
	}

	err := ApplyMergePlan(plan, diskDir, newDir, nil)
	require.NoError(t, err)

	// Conteúdo do usuário deve ser preservado
	data, err := os.ReadFile(filepath.Join(diskDir, "arquivo.txt"))
	require.NoError(t, err)
	assert.Equal(t, "conteúdo do usuário", string(data))
}

func TestApplyMergePlan_ConflictMarkers(t *testing.T) {
	diskDir := t.TempDir()
	newDir := t.TempDir()

	userContent := "conteúdo do usuário"
	scaffoldContent := "conteúdo do scaffold"

	helperWriteFile(t, diskDir, "arquivo.txt", userContent)
	helperWriteFile(t, newDir, "arquivo.txt", scaffoldContent)

	plan := &MergePlan{
		Entries: []MergeEntry{
			{RelPath: "arquivo.txt", Action: ActionConflict},
		},
	}

	resolver := &NonInteractiveResolver{Strategy: "conflict-markers"}
	err := ApplyMergePlan(plan, diskDir, newDir, resolver)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(diskDir, "arquivo.txt"))
	require.NoError(t, err)

	result := string(data)
	assert.True(t, strings.Contains(result, "<<<<<<< USUARIO (atual)"))
	assert.True(t, strings.Contains(result, userContent))
	assert.True(t, strings.Contains(result, "======="))
	assert.True(t, strings.Contains(result, scaffoldContent))
	assert.True(t, strings.Contains(result, ">>>>>>> SCAFFOLD (novo)"))
}

func TestApplyMergePlan_New(t *testing.T) {
	diskDir := t.TempDir()
	newDir := t.TempDir()

	helperWriteFile(t, newDir, "sub/novo.txt", "conteúdo novo")

	plan := &MergePlan{
		Entries: []MergeEntry{
			{RelPath: "sub/novo.txt", Action: ActionNew},
		},
	}

	err := ApplyMergePlan(plan, diskDir, newDir, nil)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(diskDir, "sub", "novo.txt"))
	require.NoError(t, err)
	assert.Equal(t, "conteúdo novo", string(data))
}

func TestComputeFileHash_Deterministic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "teste.txt")
	require.NoError(t, os.WriteFile(path, []byte("conteúdo fixo"), 0644))

	hash1, err := ComputeFileHash(path)
	require.NoError(t, err)

	hash2, err := ComputeFileHash(path)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2)
	assert.Len(t, hash1, 64) // SHA-256 em hex = 64 caracteres
}

func TestNonInteractiveResolver_KeepUser(t *testing.T) {
	resolver := &NonInteractiveResolver{Strategy: "keep-user"}
	result, err := resolver.Resolve(MergeEntry{}, []byte("user"), []byte("scaffold"))
	require.NoError(t, err)
	assert.Equal(t, "user", string(result))
}

func TestNonInteractiveResolver_KeepScaffold(t *testing.T) {
	resolver := &NonInteractiveResolver{Strategy: "keep-scaffold"}
	result, err := resolver.Resolve(MergeEntry{}, []byte("user"), []byte("scaffold"))
	require.NoError(t, err)
	assert.Equal(t, "scaffold", string(result))
}

func TestNonInteractiveResolver_Unknown(t *testing.T) {
	resolver := &NonInteractiveResolver{Strategy: "invalid"}
	_, err := resolver.Resolve(MergeEntry{}, []byte("user"), []byte("scaffold"))
	assert.Error(t, err)
}

// ═══════════════════════════════════════════════════════════════════════════════
// Testes de DetectOrphanedFiles (P1.7)
// ═══════════════════════════════════════════════════════════════════════════════

func TestDetectOrphanedFiles_CompleteParaStandard(t *testing.T) {
	dir := t.TempDir()
	// Topologia "complete" tem: local, dev, staging, prod
	// Topologia "standard" tem: local, prod
	// Órfãos: values-dev.yaml, values-staging.yaml
	helperWriteFile(t, dir, "charts/argocd/values-local.yaml", "local")
	helperWriteFile(t, dir, "charts/argocd/values-dev.yaml", "dev")
	helperWriteFile(t, dir, "charts/argocd/values-staging.yaml", "staging")
	helperWriteFile(t, dir, "charts/argocd/values-prod.yaml", "prod")

	orphans := DetectOrphanedFiles(dir, "complete", "standard")

	assert.Len(t, orphans, 2, "deve detectar 2 arquivos órfãos")
	joined := strings.Join(orphans, "\n")
	assert.Contains(t, joined, "values-dev.yaml")
	assert.Contains(t, joined, "values-staging.yaml")
}

func TestDetectOrphanedFiles_StandardParaSingle(t *testing.T) {
	dir := t.TempDir()
	helperWriteFile(t, dir, "values-local.yaml", "local")
	helperWriteFile(t, dir, "values-prod.yaml", "prod")

	orphans := DetectOrphanedFiles(dir, "standard", "single")
	assert.Len(t, orphans, 1)
	assert.Contains(t, orphans[0], "values-prod.yaml")
}

func TestDetectOrphanedFiles_MesmaTopologia(t *testing.T) {
	dir := t.TempDir()
	helperWriteFile(t, dir, "values-local.yaml", "local")

	orphans := DetectOrphanedFiles(dir, "standard", "standard")
	assert.Empty(t, orphans, "mesma topologia não deve gerar órfãos")
}

func TestDetectOrphanedFiles_TopologiaVazia(t *testing.T) {
	dir := t.TempDir()
	orphans := DetectOrphanedFiles(dir, "", "standard")
	assert.Empty(t, orphans)

	orphans = DetectOrphanedFiles(dir, "standard", "")
	assert.Empty(t, orphans)
}

func TestDetectOrphanedFiles_SemArquivosNoDisco(t *testing.T) {
	dir := t.TempDir()
	orphans := DetectOrphanedFiles(dir, "complete", "single")
	assert.Empty(t, orphans, "sem arquivos no disco não deve retornar órfãos")
}

func TestDetectOrphanedFiles_SingleParaComplete(t *testing.T) {
	dir := t.TempDir()
	helperWriteFile(t, dir, "values-local.yaml", "local")

	// Expandindo de single para complete — nenhum órfão, pois complete contém todos
	orphans := DetectOrphanedFiles(dir, "single", "complete")
	assert.Empty(t, orphans, "expandir topologia não deve gerar órfãos")
}

func TestTopologyEnvironments(t *testing.T) {
	assert.Equal(t, []string{"local"}, topologyEnvironments("single"))
	assert.Equal(t, []string{"local", "prod"}, topologyEnvironments("standard"))
	assert.Equal(t, []string{"local", "dev", "staging", "prod"}, topologyEnvironments("complete"))
	assert.Equal(t, []string{"local"}, topologyEnvironments("desconhecido"))
}

func TestMergePlan_Summary(t *testing.T) {
	plan := &MergePlan{
		Entries: []MergeEntry{
			{Action: ActionNone},
			{Action: ActionNone},
			{Action: ActionUpdate},
			{Action: ActionPreserve},
			{Action: ActionConflict},
			{Action: ActionNew},
			{Action: ActionNew},
		},
	}

	summary := plan.Summary()
	assert.Equal(t, 2, summary[ActionNone])
	assert.Equal(t, 1, summary[ActionUpdate])
	assert.Equal(t, 1, summary[ActionPreserve])
	assert.Equal(t, 1, summary[ActionConflict])
	assert.Equal(t, 2, summary[ActionNew])
}
