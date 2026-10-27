// Package decay implementa análise de decay (obsolescência) de documentos UKI.
package decay

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/plugins/synapstor/internal/uki"
)

// DecayInfo contém informações de obsolescência de um UKI.
type DecayInfo struct {
	Path              string    `json:"path"`
	Title             string    `json:"title"`
	CreatedAt         time.Time `json:"created_at"`
	LastGitActivity   time.Time `json:"last_git_activity"`
	DaysSinceActivity int       `json:"days_since_activity"`
	IsStale           bool      `json:"is_stale"`
	// CodeCommits conta os commits, depois da nota, nos arquivos que ela cita.
	CodeCommits int `json:"code_commits"`
	// CodePaths são os arquivos citados na nota que existem no projeto.
	CodePaths []string `json:"code_paths,omitempty"`
}

// StaleThresholdDays define o limite de dias sem atividade para considerar stale.
const StaleThresholdDays = 90

// reTimestamp extrai o timestamp do nome do arquivo UKI (UKI-TIMESTAMP-slug.md).
var reTimestamp = regexp.MustCompile(`UKI-(\d+)-`)

// reCaminho acha os caminhos de arquivo citados no texto (pasta/arquivo.ext), sem as URLs.
var reCaminho = regexp.MustCompile(`(?:^|[\s\x60"'(\[])((?:[\w.\-]+/)+[\w.\-]+\.[A-Za-z0-9]+)`)

// reHash reconhece o hash de um commit.
var reHash = regexp.MustCompile(`^[0-9a-f]{40}$`)

// maxCaminhos limita quantos arquivos citados por UKI são consultados no git.
const maxCaminhos = 20

// prazoGit é o prazo de cada comando git.
const prazoGit = 15 * time.Second

// CommandRunner abstrai a execução de comandos para facilitar testes.
type CommandRunner interface {
	Run(name string, args ...string) (string, error)
}

// RealRunner executa comandos reais no sistema.
type RealRunner struct{}

// Run executa um comando e retorna o output.
func (r *RealRunner) Run(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), prazoGit)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// AnalyzeDecay analisa a obsolescência dos UKIs em relação à atividade git.
func AnalyzeDecay(ukiDir, projectDir string) ([]DecayInfo, error) {
	return AnalyzeDecayWithRunner(ukiDir, projectDir, &RealRunner{})
}

// AnalyzeDecayWithRunner analisa a obsolescência usando um runner customizável.
func AnalyzeDecayWithRunner(ukiDir, projectDir string, runner CommandRunner) ([]DecayInfo, error) {
	arquivos, err := uki.Listar(ukiDir)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var infos []DecayInfo

	for _, a := range arquivos {
		filePath := a.Caminho
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		info := DecayInfo{
			Path: filePath,
		}

		// Título do corpo; data de criação do frontmatter ou, sem ela, do nome do arquivo
		meta, _ := uki.Ler(string(data))
		info.Title = meta.Titulo
		if info.Title == "" {
			info.Title = filepath.Base(filePath)
		}
		info.CreatedAt = meta.CriadoEm

		if matches := reTimestamp.FindStringSubmatch(filepath.Base(filePath)); info.CreatedAt.IsZero() && len(matches) > 1 {
			ts, err := strconv.ParseInt(matches[1], 10, 64)
			if err == nil && timestampPlausivel(ts, now) {
				info.CreatedAt = time.Unix(ts, 0)
			}
		}

		// Verificar última atividade git no diretório referenciado
		info.LastGitActivity = getLastGitActivity(runner, projectDir, filePath)

		// Calcular dias desde última atividade
		refTime := info.LastGitActivity
		if meta.AtualizadoEm.After(refTime) {
			refTime = meta.AtualizadoEm
		}
		if refTime.IsZero() {
			refTime = info.CreatedAt
		}
		if !refTime.IsZero() {
			info.DaysSinceActivity = int(now.Sub(refTime).Hours() / 24)
		}

		// O código que a nota descreve mudou depois dela
		info.CodePaths = caminhosCitados(string(data), projectDir)
		info.CodeCommits = commitsNoCodigo(runner, projectDir, filePath, refTime, info.CodePaths)

		info.IsStale = info.DaysSinceActivity > StaleThresholdDays || info.CodeCommits > 0

		infos = append(infos, info)
	}

	return infos, nil
}

// timestampPlausivel confere se o número do nome do arquivo é um unix de verdade (entre
// 2000 e amanhã); a IA escolhe o nome, e UKI-20261026-x não é uma data de 1970.
func timestampPlausivel(ts int64, agora time.Time) bool {
	return ts >= 946684800 && ts <= agora.Add(24*time.Hour).Unix()
}

// caminhosCitados devolve os arquivos citados no texto que existem no projeto, sem
// repetição e sem os próprios UKIs.
func caminhosCitados(texto, projectDir string) []string {
	var caminhos []string
	vistos := map[string]bool{}
	for _, m := range reCaminho.FindAllStringSubmatch(texto, -1) {
		c := filepath.ToSlash(filepath.Clean(strings.TrimPrefix(m[1], "./")))
		if vistos[c] || strings.HasPrefix(c, "../") || strings.HasPrefix(c, ".synapstor/") {
			continue
		}
		vistos[c] = true
		if st, err := os.Stat(filepath.Join(projectDir, filepath.FromSlash(c))); err != nil || st.IsDir() {
			continue
		}
		caminhos = append(caminhos, c)
		if len(caminhos) == maxCaminhos {
			break
		}
	}
	return caminhos
}

// commitsNoCodigo conta os commits distintos, nos arquivos citados, depois do último
// commit do UKI; sem commit do UKI, os depois da data de referência. Sem nenhum dos dois,
// não há como medir.
func commitsNoCodigo(runner CommandRunner, projectDir, ukiPath string, ref time.Time, caminhos []string) int {
	if len(caminhos) == 0 {
		return 0
	}
	var filtro string
	if out, err := runner.Run("git", "-C", projectDir, "log", "-1", "--format=%H", "--", ukiPath); err == nil && reHash.MatchString(out) {
		filtro = out + "..HEAD"
	} else if !ref.IsZero() {
		filtro = "--since=" + ref.Format(time.RFC3339)
	} else {
		return 0
	}
	commits := map[string]bool{}
	for _, c := range caminhos {
		out, err := runner.Run("git", "-C", projectDir, "log", "--format=%H", filtro, "--", c)
		if err != nil {
			continue
		}
		for _, h := range strings.Fields(out) {
			commits[h] = true
		}
	}
	return len(commits)
}

// getLastGitActivity consulta o git para encontrar a última atividade em um arquivo.
func getLastGitActivity(runner CommandRunner, projectDir, filePath string) time.Time {
	output, err := runner.Run("git", "-C", projectDir, "log", "-1", "--format=%ci", "--", filePath)
	if err != nil || output == "" {
		return time.Time{}
	}

	t, err := time.Parse("2006-01-02 15:04:05 -0700", output)
	if err != nil {
		return time.Time{}
	}

	return t
}

// FindStale filtra apenas os UKIs considerados stale.
func FindStale(infos []DecayInfo) []DecayInfo {
	var stale []DecayInfo
	for _, info := range infos {
		if info.IsStale {
			stale = append(stale, info)
		}
	}
	return stale
}
