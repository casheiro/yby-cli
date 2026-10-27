package context

import (
	"fmt"
	"os"
	"path/filepath"
)

// CoreContext represents the base context derived from the CLI/Project itself.
type CoreContext struct {
	ProjectName string `json:"project_name"`
	Environment string `json:"environment"`
	Overview    string `json:"overview,omitempty"` // From .synapstor/00_PROJECT_OVERVIEW.md or README.md
	Backlog     string `json:"backlog,omitempty"`  // From .synapstor/02_BACKLOG_AND_DEBT.md
}

// GetCoreContext gathers the foundational context for the current project.
// Priorities:
// 1. Synapstor Artifacts (.synapstor/...)
// 2. README.md (Fallback)
// 3. Project Identity (Folder name / Git)
func GetCoreContext(rootDir string) (*CoreContext, error) {
	ctx := &CoreContext{
		ProjectName: deriveProjectName(rootDir),
	}

	// 1. Load Environment (Best Effort)
	mgr := NewManager(rootDir)
	if envName, _, err := mgr.GetCurrent(); err == nil {
		ctx.Environment = envName
	} else {
		ctx.Environment = "unknown"
	}

	// 2. Load Synapstor Context (Priority)
	synapstorDir := filepath.Join(rootDir, ".synapstor")
	if info, err := os.Stat(synapstorDir); err == nil && info.IsDir() {
		// Load Overview
		overviewPath := filepath.Join(synapstorDir, "00_PROJECT_OVERVIEW.md")
		if content, err := readFileLimited(overviewPath, 5000); err == nil {
			ctx.Overview = content
		}

		// Load Backlog
		backlogPath := filepath.Join(synapstorDir, "02_BACKLOG_AND_DEBT.md")
		if content, err := readFileLimited(backlogPath, 5000); err == nil {
			ctx.Backlog = content
		}
	}

	// 3. Fallback to README if Overview is empty
	if ctx.Overview == "" {
		readmePath := filepath.Join(rootDir, "README.md")
		if content, err := readFileLimited(readmePath, 3000); err == nil {
			ctx.Overview = fmt.Sprintf("Source: README.md\n%s", content)
		} else {
			ctx.Overview = "Nenhum arquivo de contexto (Synapstor ou README) encontrado."
		}
	}

	return ctx, nil
}

func deriveProjectName(path string) string {
	return filepath.Base(path)
}

func readFileLimited(path string, limit int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := string(data)
	if len(s) > limit {
		return s[:limit] + "\n... (truncated)", nil
	}
	return s, nil
}
