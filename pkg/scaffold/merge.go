package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileAction representa a ação a ser tomada sobre um arquivo durante o merge.
type FileAction int

const (
	// ActionNone indica que o arquivo não teve alterações.
	ActionNone FileAction = iota
	// ActionUpdate indica que o scaffold mudou mas o usuário não alterou.
	ActionUpdate
	// ActionPreserve indica que o usuário alterou mas o scaffold não mudou.
	ActionPreserve
	// ActionConflict indica que ambos (usuário e scaffold) alteraram o arquivo.
	ActionConflict
	// ActionNew indica um arquivo novo do scaffold.
	ActionNew
)

// String retorna a representação textual da ação.
func (a FileAction) String() string {
	switch a {
	case ActionNone:
		return "sem alterações"
	case ActionUpdate:
		return "atualizar"
	case ActionPreserve:
		return "preservar"
	case ActionConflict:
		return "conflito"
	case ActionNew:
		return "novo"
	default:
		return "desconhecido"
	}
}

// MergeEntry descreve o plano de ação para um arquivo individual.
type MergeEntry struct {
	RelPath  string
	Action   FileAction
	OldHash  string // hash do manifest original
	DiskHash string // hash atual no disco
	NewHash  string // hash do novo scaffold
}

// MergePlan contém o plano completo de merge para todos os arquivos.
type MergePlan struct {
	Entries []MergeEntry
}

// Summary retorna contagens por tipo de ação.
func (p *MergePlan) Summary() map[FileAction]int {
	counts := make(map[FileAction]int)
	for _, e := range p.Entries {
		counts[e.Action]++
	}
	return counts
}

// ConflictResolver define a interface para resolução de conflitos durante o merge.
type ConflictResolver interface {
	Resolve(entry MergeEntry, diskContent, newContent []byte) ([]byte, error)
}

// NonInteractiveResolver resolve conflitos automaticamente usando uma estratégia fixa.
type NonInteractiveResolver struct {
	Strategy string // "keep-user", "keep-scaffold", "conflict-markers"
}

// Resolve aplica a estratégia de resolução não-interativa.
func (r *NonInteractiveResolver) Resolve(entry MergeEntry, diskContent, newContent []byte) ([]byte, error) {
	switch r.Strategy {
	case "keep-user":
		return diskContent, nil
	case "keep-scaffold":
		return newContent, nil
	case "conflict-markers":
		return addConflictMarkers(diskContent, newContent), nil
	default:
		return nil, fmt.Errorf("estratégia de resolução desconhecida: %s", r.Strategy)
	}
}

// addConflictMarkers gera conteúdo com marcadores de conflito estilo Git.
func addConflictMarkers(diskContent, newContent []byte) []byte {
	var b strings.Builder
	b.WriteString("<<<<<<< USUARIO (atual)\n")
	b.Write(diskContent)
	if len(diskContent) > 0 && diskContent[len(diskContent)-1] != '\n' {
		b.WriteByte('\n')
	}
	b.WriteString("=======\n")
	b.Write(newContent)
	if len(newContent) > 0 && newContent[len(newContent)-1] != '\n' {
		b.WriteByte('\n')
	}
	b.WriteString(">>>>>>> SCAFFOLD (novo)\n")
	return []byte(b.String())
}

// ComputeMergePlan calcula o plano de merge comparando hashes do manifest, disco e novo scaffold.
func ComputeMergePlan(manifestHashes map[string]string, diskDir, newDir string) (*MergePlan, error) {
	plan := &MergePlan{}

	// Calcular hashes dos arquivos novos do scaffold
	newHashes, err := ComputeDirHashes(newDir)
	if err != nil {
		return nil, fmt.Errorf("erro ao calcular hashes do novo scaffold: %w", err)
	}

	for relPath, newHash := range newHashes {
		oldHash, existsInManifest := manifestHashes[relPath]

		if !existsInManifest {
			// Arquivo novo no scaffold
			plan.Entries = append(plan.Entries, MergeEntry{
				RelPath: relPath,
				Action:  ActionNew,
				NewHash: newHash,
			})
			continue
		}

		// Calcular hash do arquivo atual no disco
		diskPath := filepath.Join(diskDir, relPath)
		diskHash, err := ComputeFileHash(diskPath)
		if err != nil {
			// Arquivo não existe no disco — tratar como novo
			plan.Entries = append(plan.Entries, MergeEntry{
				RelPath: relPath,
				Action:  ActionNew,
				NewHash: newHash,
			})
			continue
		}

		entry := MergeEntry{
			RelPath:  relPath,
			OldHash:  oldHash,
			DiskHash: diskHash,
			NewHash:  newHash,
		}

		userChanged := diskHash != oldHash
		scaffoldChanged := newHash != oldHash

		switch {
		case !userChanged && !scaffoldChanged:
			entry.Action = ActionNone
		case !userChanged && scaffoldChanged:
			entry.Action = ActionUpdate
		case userChanged && !scaffoldChanged:
			entry.Action = ActionPreserve
		default:
			entry.Action = ActionConflict
		}

		plan.Entries = append(plan.Entries, entry)
	}

	return plan, nil
}

// ApplyMergePlan executa o plano de merge, copiando/resolvendo arquivos conforme a ação.
func ApplyMergePlan(plan *MergePlan, diskDir, newDir string, resolver ConflictResolver) error {
	for _, entry := range plan.Entries {
		switch entry.Action {
		case ActionNone, ActionPreserve:
			// Nada a fazer
			continue

		case ActionUpdate, ActionNew:
			srcPath := filepath.Join(newDir, entry.RelPath)
			destPath := filepath.Join(diskDir, entry.RelPath)

			data, err := os.ReadFile(srcPath)
			if err != nil {
				return fmt.Errorf("erro ao ler arquivo do scaffold %s: %w", entry.RelPath, err)
			}

			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return fmt.Errorf("erro ao criar diretório para %s: %w", entry.RelPath, err)
			}

			if err := os.WriteFile(destPath, data, 0644); err != nil {
				return fmt.Errorf("erro ao escrever arquivo %s: %w", entry.RelPath, err)
			}

		case ActionConflict:
			if resolver == nil {
				return fmt.Errorf("conflito detectado em %s mas nenhum resolver configurado", entry.RelPath)
			}

			diskPath := filepath.Join(diskDir, entry.RelPath)
			newPath := filepath.Join(newDir, entry.RelPath)

			diskContent, err := os.ReadFile(diskPath)
			if err != nil {
				return fmt.Errorf("erro ao ler arquivo do disco %s: %w", entry.RelPath, err)
			}

			newContent, err := os.ReadFile(newPath)
			if err != nil {
				return fmt.Errorf("erro ao ler arquivo do scaffold %s: %w", entry.RelPath, err)
			}

			resolved, err := resolver.Resolve(entry, diskContent, newContent)
			if err != nil {
				return fmt.Errorf("erro ao resolver conflito em %s: %w", entry.RelPath, err)
			}

			if err := os.WriteFile(diskPath, resolved, 0644); err != nil {
				return fmt.Errorf("erro ao escrever resolução de %s: %w", entry.RelPath, err)
			}
		}
	}

	return nil
}
