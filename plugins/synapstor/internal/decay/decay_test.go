package decay

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// MockRunner simula execução de comandos para testes.
type MockRunner struct {
	Responses map[string]string
}

// Run retorna respostas pré-configuradas baseado no path do arquivo.
func (m *MockRunner) Run(name string, args ...string) (string, error) {
	// O último argumento é o path do arquivo
	if len(args) > 0 {
		lastArg := args[len(args)-1]
		if resp, ok := m.Responses[lastArg]; ok {
			return resp, nil
		}
	}
	return "", fmt.Errorf("sem resposta mock")
}

func criarUKIsDecay(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	// UKI recente (timestamp = agora - 30 dias)
	recentTS := time.Now().Add(-30 * 24 * time.Hour).Unix()
	recentFile := fmt.Sprintf("UKI-%d-recente.md", recentTS)
	recentContent := `# Documento Recente
**ID:** UKI-RECENTE
**Type:** Reference
**Status:** Active

## Content
Este documento é recente.
`
	if err := os.WriteFile(filepath.Join(dir, recentFile), []byte(recentContent), 0644); err != nil {
		t.Fatal(err)
	}

	// UKI antigo (timestamp = agora - 200 dias)
	oldTS := time.Now().Add(-200 * 24 * time.Hour).Unix()
	oldFile := fmt.Sprintf("UKI-%d-antigo.md", oldTS)
	oldContent := `# Documento Antigo
**ID:** UKI-ANTIGO
**Type:** Decision
**Status:** Draft

## Content
Este documento é antigo e não tem atividade git.
`
	if err := os.WriteFile(filepath.Join(dir, oldFile), []byte(oldContent), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestAnalyzeDecayWithRunner_DetectaStale(t *testing.T) {
	dir := t.TempDir()
	ukiDir := filepath.Join(dir, "ukis")
	criarUKIsDecay(t, ukiDir)

	// Mock: nenhuma atividade git (retorna erro)
	mock := &MockRunner{Responses: map[string]string{}}

	infos, err := AnalyzeDecayWithRunner(ukiDir, dir, mock)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if len(infos) != 2 {
		t.Fatalf("esperado 2 infos, obtido %d", len(infos))
	}

	// O documento antigo (>90 dias) deve ser stale
	stale := filtrarStale(infos)
	if len(stale) == 0 {
		t.Error("esperado ao menos 1 UKI stale")
	}

	// Verificar que o recente não é stale
	for _, info := range infos {
		if info.Title == "Documento Recente" && info.IsStale {
			t.Error("documento recente não deveria ser stale")
		}
	}
}

// filtrarStale devolve só os UKIs marcados como stale; os testes conferem o IsStale pelo
// próprio resultado, sem uma função de produção só para isso.
func filtrarStale(infos []DecayInfo) []DecayInfo {
	var stale []DecayInfo
	for _, info := range infos {
		if info.IsStale {
			stale = append(stale, info)
		}
	}
	return stale
}

func TestAnalyzeDecayWithRunner_ComAtividadeGit(t *testing.T) {
	dir := t.TempDir()
	ukiDir := filepath.Join(dir, "ukis")
	criarUKIsDecay(t, ukiDir)

	// Mock: atividade git recente para todos os arquivos
	recentDate := time.Now().Add(-10 * 24 * time.Hour).Format("2006-01-02 15:04:05 -0700")
	entries, _ := os.ReadDir(ukiDir)
	responses := make(map[string]string)
	for _, e := range entries {
		responses[filepath.Join(ukiDir, e.Name())] = recentDate
	}

	mock := &MockRunner{Responses: responses}

	infos, err := AnalyzeDecayWithRunner(ukiDir, dir, mock)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	stale := filtrarStale(infos)
	if len(stale) != 0 {
		t.Errorf("nenhum UKI deveria ser stale com atividade git recente, obtido %d", len(stale))
	}
}

func TestAnalyzeDecayWithRunner_DiretorioInexistente(t *testing.T) {
	mock := &MockRunner{}
	_, err := AnalyzeDecayWithRunner("/caminho/inexistente", "/tmp", mock)
	if err == nil {
		t.Error("esperado erro para diretório inexistente")
	}
}

func TestAnalyzeDecayWithRunner_ExtraiTimestampDoFilename(t *testing.T) {
	dir := t.TempDir()
	ukiDir := filepath.Join(dir, "ukis")
	if err := os.MkdirAll(ukiDir, 0755); err != nil {
		t.Fatal(err)
	}

	targetDate := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	ts := targetDate.Unix()
	filename := fmt.Sprintf("UKI-%d-teste.md", ts)
	content := "# Teste\nConteúdo.\n"
	if err := os.WriteFile(filepath.Join(ukiDir, filename), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	mock := &MockRunner{Responses: map[string]string{}}
	infos, err := AnalyzeDecayWithRunner(ukiDir, dir, mock)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("esperado 1 info, obtido %d", len(infos))
	}

	if infos[0].CreatedAt.IsZero() {
		t.Error("CreatedAt não deveria ser zero")
	}
	if infos[0].CreatedAt.Year() != targetDate.Year() {
		t.Errorf("esperado ano %d, obtido %d", targetDate.Year(), infos[0].CreatedAt.Year())
	}
}

// TestAnalyzeDecay_SubpastasEFrontmatter: o UKI de uma subpasta de domínio também entra,
// com o título do corpo e a data de criação do frontmatter.
func TestAnalyzeDecay_SubpastasEFrontmatter(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "dominio"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "UKI-1-a.md"), []byte("# A"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := "---\nid: d.b\ntipo: regra\nstatus: ativo\ncriado_em: 2026-01-02\n---\n\n# Regra B\n"
	if err := os.WriteFile(filepath.Join(dir, "dominio", "B.md"), []byte(b), 0o644); err != nil {
		t.Fatal(err)
	}
	infos, err := AnalyzeDecayWithRunner(dir, dir, &MockRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 {
		t.Fatalf("esperados 2 UKIs, obtido %d", len(infos))
	}
	for _, info := range infos {
		if filepath.Base(info.Path) == "B.md" {
			if info.Title != "Regra B" || !info.CreatedAt.Equal(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)) {
				t.Errorf("esperado título e data do frontmatter, obtido %+v", info)
			}
		}
	}
}

// git roda o git num repositório temporário, com identidade fixa.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	args = append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)
	if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func escrever(t *testing.T, caminho, conteudo string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caminho, []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestAnalyzeDecay_ContaCommitsNoCodigo: a nota que cita pkg/x/y.go fica desatualizada
// quando o código muda depois dela, mesmo com a nota recém-commitada.
func TestAnalyzeDecay_ContaCommitsNoCodigo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git indisponível")
	}
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	escrever(t, filepath.Join(dir, "pkg", "x", "y.go"), "package x\n")
	escrever(t, filepath.Join(dir, ".synapstor", ".uki", "regra.md"),
		"# Regra\n\nVeja `pkg/x/y.go` e `pkg/nao/existe.go` e https://exemplo.com/a/b.go.\n")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "nota")
	for i := 0; i < 3; i++ {
		escrever(t, filepath.Join(dir, "pkg", "x", "y.go"), fmt.Sprintf("package x\n// %d\n", i))
		git(t, dir, "commit", "-q", "-am", fmt.Sprintf("muda %d", i))
	}

	infos, err := AnalyzeDecay(filepath.Join(dir, ".synapstor", ".uki"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 1 {
		t.Fatalf("esperado 1 UKI, obtido %d", len(infos))
	}
	if infos[0].CodeCommits != 3 {
		t.Errorf("esperados 3 commits no código, obtido %d", infos[0].CodeCommits)
	}
	if len(infos[0].CodePaths) != 1 || infos[0].CodePaths[0] != "pkg/x/y.go" {
		t.Errorf("só o arquivo que existe deveria entrar: %v", infos[0].CodePaths)
	}
	if !infos[0].IsStale {
		t.Error("código mudou depois da nota: deveria estar desatualizado")
	}
}

// TestAnalyzeDecay_TimestampImplausivel: UKI-20261026-x não é um unix de 1970.
func TestAnalyzeDecay_TimestampImplausivel(t *testing.T) {
	dir := t.TempDir()
	escrever(t, filepath.Join(dir, "UKI-20261026-x.md"), "# X\n")
	infos, err := AnalyzeDecayWithRunner(dir, dir, &MockRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 1 || !infos[0].CreatedAt.IsZero() || infos[0].IsStale || infos[0].DaysSinceActivity != 0 {
		t.Errorf("timestamp inválido do nome deveria ser ignorado: %+v", infos)
	}
}

// TestAnalyzeDecay_UsaAtualizadoEm: sem histórico git, o updated_at do frontmatter vale.
func TestAnalyzeDecay_UsaAtualizadoEm(t *testing.T) {
	dir := t.TempDir()
	antigo := time.Now().AddDate(0, 0, -200).Format(time.DateOnly)
	escrever(t, filepath.Join(dir, "regra.md"), "---\nid: a.b\ntipo: regra\nstatus: ativo\nupdated_at: "+antigo+"\n---\n\n# R\n")
	infos, err := AnalyzeDecayWithRunner(dir, dir, &MockRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 1 || !infos[0].IsStale || infos[0].DaysSinceActivity < 190 {
		t.Errorf("updated_at de 200 dias deveria sair desatualizado: %+v", infos)
	}
}
