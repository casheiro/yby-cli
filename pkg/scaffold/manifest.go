package scaffold

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// ProjectManifest persiste as decisões de configuração do yby init.
type ProjectManifest struct {
	APIVersion string          `yaml:"apiVersion"`
	Kind       string          `yaml:"kind"`
	Metadata   ProjectMetadata `yaml:"metadata"`
	Spec       ProjectSpec     `yaml:"spec"`
}

// ProjectMetadata contém informações de identificação do projeto.
type ProjectMetadata struct {
	Name      string `yaml:"name"`
	CreatedAt string `yaml:"createdAt"`
}

// ProjectSpec contém as decisões de configuração do projeto.
type ProjectSpec struct {
	Topology        string            `yaml:"topology"`
	Workflow        string            `yaml:"workflow"`
	SecretsStrategy string            `yaml:"secretsStrategy"`
	Features        FeatureFlags      `yaml:"features"`
	Git             GitSpec           `yaml:"git"`
	Domain          string            `yaml:"domain"`
	Email           string            `yaml:"email"`
	FileHashes      map[string]string `yaml:"fileHashes,omitempty"`
}

// FeatureFlags representa as flags de features habilitadas no projeto.
type FeatureFlags struct {
	Kepler        bool `yaml:"kepler"`
	Minio         bool `yaml:"minio"`
	KEDA          bool `yaml:"keda"`
	MetricsServer bool `yaml:"metricsServer"`
	DevContainer  bool `yaml:"devContainer"`
	CI            bool `yaml:"ci"`
	Discovery     bool `yaml:"discovery"`
}

// GitSpec contém a configuração de repositório git.
type GitSpec struct {
	Repo   string `yaml:"repo"`
	Branch string `yaml:"branch"`
}

// SaveProjectManifest serializa o BlueprintContext para .yby/project.yaml.
// fileHashes é opcional — se nil, o manifest é salvo sem hashes de arquivos.
func SaveProjectManifest(targetDir string, ctx *BlueprintContext, fileHashes ...map[string]string) error {
	manifest := ProjectManifest{
		APIVersion: "yby/v1",
		Kind:       "ProjectManifest",
		Metadata: ProjectMetadata{
			Name:      ctx.ProjectName,
			CreatedAt: time.Now().Format(time.RFC3339),
		},
		Spec: ProjectSpec{
			Topology:        ctx.Topology,
			Workflow:        ctx.WorkflowPattern,
			SecretsStrategy: ctx.SecretsStrategy,
			Features: FeatureFlags{
				Kepler:        ctx.EnableKepler,
				Minio:         ctx.EnableMinio,
				KEDA:          ctx.EnableKEDA,
				MetricsServer: ctx.EnableMetricsServer,
				DevContainer:  ctx.EnableDevContainer,
				CI:            ctx.EnableCI,
				Discovery:     ctx.EnableDiscovery,
			},
			Git: GitSpec{
				Repo:   ctx.GitRepo,
				Branch: ctx.GitBranch,
			},
			Domain: ctx.Domain,
			Email:  ctx.Email,
		},
	}

	if len(fileHashes) > 0 && fileHashes[0] != nil {
		manifest.Spec.FileHashes = fileHashes[0]
	}

	data, err := yaml.Marshal(&manifest)
	if err != nil {
		return fmt.Errorf("erro ao serializar project manifest: %w", err)
	}

	path := filepath.Join(targetDir, ".yby", "project.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório .yby: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// ComputeFileHash calcula o hash SHA-256 de um arquivo.
func ComputeFileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

// ComputeDirHashes calcula o hash SHA-256 de todos os arquivos regulares em um diretório.
// Retorna um mapa de caminho relativo -> hash.
func ComputeDirHashes(dir string) (map[string]string, error) {
	hashes := make(map[string]string)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		hash, err := ComputeFileHash(path)
		if err != nil {
			return err
		}

		hashes[relPath] = hash
		return nil
	})
	if err != nil {
		return nil, err
	}
	return hashes, nil
}

// LoadProjectManifest carrega o project manifest de .yby/project.yaml.
func LoadProjectManifest(targetDir string) (*ProjectManifest, error) {
	path := filepath.Join(targetDir, ".yby", "project.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest ProjectManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("erro ao ler project manifest: %w", err)
	}

	return &manifest, nil
}

// ManifestToContext converte um ProjectManifest de volta para BlueprintContext.
func ManifestToContext(m *ProjectManifest) *BlueprintContext {
	return &BlueprintContext{
		ProjectName:         m.Metadata.Name,
		Topology:            m.Spec.Topology,
		WorkflowPattern:     m.Spec.Workflow,
		SecretsStrategy:     m.Spec.SecretsStrategy,
		EnableKepler:        m.Spec.Features.Kepler,
		EnableMinio:         m.Spec.Features.Minio,
		EnableKEDA:          m.Spec.Features.KEDA,
		EnableMetricsServer: m.Spec.Features.MetricsServer,
		EnableDevContainer:  m.Spec.Features.DevContainer,
		EnableCI:            m.Spec.Features.CI,
		EnableDiscovery:     m.Spec.Features.Discovery,
		GitRepo:             m.Spec.Git.Repo,
		GitBranch:           m.Spec.Git.Branch,
		Domain:              m.Spec.Domain,
		Email:               m.Spec.Email,
	}
}

// MergeContextDefaults preenche campos vazios de target com valores de defaults.
func MergeContextDefaults(target, defaults *BlueprintContext) {
	if target.ProjectName == "" {
		target.ProjectName = defaults.ProjectName
	}
	if target.Topology == "" {
		target.Topology = defaults.Topology
	}
	if target.WorkflowPattern == "" {
		target.WorkflowPattern = defaults.WorkflowPattern
	}
	if target.SecretsStrategy == "" {
		target.SecretsStrategy = defaults.SecretsStrategy
	}
	if target.GitRepo == "" {
		target.GitRepo = defaults.GitRepo
	}
	if target.GitBranch == "" {
		target.GitBranch = defaults.GitBranch
	}
	if target.Domain == "" {
		target.Domain = defaults.Domain
	}
	if target.Email == "" {
		target.Email = defaults.Email
	}
	// Features: só aplica defaults se target não setou explicitamente
	// Como bools são zero-value false, usamos os defaults quando target é false
	// mas defaults é true — ou seja, preservamos features habilitadas do manifest anterior
	if !target.EnableKepler && defaults.EnableKepler {
		target.EnableKepler = true
	}
	if !target.EnableMinio && defaults.EnableMinio {
		target.EnableMinio = true
	}
	if !target.EnableKEDA && defaults.EnableKEDA {
		target.EnableKEDA = true
	}
	if !target.EnableMetricsServer && defaults.EnableMetricsServer {
		target.EnableMetricsServer = true
	}
	if !target.EnableDevContainer && defaults.EnableDevContainer {
		target.EnableDevContainer = true
	}
	if !target.EnableCI && defaults.EnableCI {
		target.EnableCI = true
	}
	if !target.EnableDiscovery && defaults.EnableDiscovery {
		target.EnableDiscovery = true
	}
}
