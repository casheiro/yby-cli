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
