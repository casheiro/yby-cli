// Package decay implementa análise de decay (obsolescência) de documentos UKI.
package decay

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DecayInfo contém informações de obsolescência de um UKI.
type DecayInfo struct {
	Path              string    `json:"path"`
	Title             string    `json:"title"`
	CreatedAt         time.Time `json:"created_at"`
	LastGitActivity   time.Time `json:"last_git_activity"`
	DaysSinceActivity int       `json:"days_since_activity"`
	IsStale           bool      `json:"is_stale"`
}

// StaleThresholdDays define o limite de dias sem atividade para considerar stale.
const StaleThresholdDays = 90

// reTimestamp extrai o timestamp do nome do arquivo UKI (UKI-TIMESTAMP-slug.md).
var reTimestamp = regexp.MustCompile(`UKI-(\d+)-`)

// reTitle extrai o título do conteúdo markdown.
var reTitle = regexp.MustCompile(`(?m)^#\s+(.+)$`)

// CommandRunner abstrai a execução de comandos para facilitar testes.
type CommandRunner interface {
	Run(name string, args ...string) (string, error)
}

// RealRunner executa comandos reais no sistema.
type RealRunner struct{}

// Run executa um comando e retorna o output.
func (r *RealRunner) Run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// AnalyzeDecay analisa a obsolescência dos UKIs em relação à atividade git.
func AnalyzeDecay(ukiDir, projectDir string) ([]DecayInfo, error) {
	return AnalyzeDecayWithRunner(ukiDir, projectDir, &RealRunner{})
}

// AnalyzeDecayWithRunner analisa a obsolescência usando um runner customizável.
func AnalyzeDecayWithRunner(ukiDir, projectDir string, runner CommandRunner) ([]DecayInfo, error) {
	entries, err := os.ReadDir(ukiDir)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler diretório de UKIs: %w", err)
	}

	now := time.Now()
	var infos []DecayInfo

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(ukiDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		info := DecayInfo{
			Path: filePath,
		}

		// Extrair título
		content := string(data)
		if matches := reTitle.FindStringSubmatch(content); len(matches) > 1 {
			info.Title = strings.TrimSpace(matches[1])
		} else {
			info.Title = entry.Name()
		}

		// Extrair data de criação do filename
		if matches := reTimestamp.FindStringSubmatch(entry.Name()); len(matches) > 1 {
			ts, err := strconv.ParseInt(matches[1], 10, 64)
			if err == nil {
				info.CreatedAt = time.Unix(ts, 0)
			}
		}

		// Verificar última atividade git no diretório referenciado
		info.LastGitActivity = getLastGitActivity(runner, projectDir, filePath)

		// Calcular dias desde última atividade
		refTime := info.LastGitActivity
		if refTime.IsZero() {
			refTime = info.CreatedAt
		}
		if !refTime.IsZero() {
			info.DaysSinceActivity = int(now.Sub(refTime).Hours() / 24)
		}

		info.IsStale = info.DaysSinceActivity > StaleThresholdDays

		infos = append(infos, info)
	}

	return infos, nil
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
