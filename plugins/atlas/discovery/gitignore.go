package discovery

import (
	"bytes"
	"context"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// IgnoradosPadrao são as pastas que o Atlas nunca percorre (dependências,
// artefatos de build, caches e pastas de editor).
var IgnoradosPadrao = []string{
	"node_modules", "vendor", ".git", ".idea", ".vscode",
	"dist", "build", "target", ".venv", ".next", ".terraform", "coverage", "__pycache__",
}

// prazoGit limita o `git ls-files`; passado o prazo, a varredura segue sem o .gitignore.
const prazoGit = 10 * time.Second

// filtroVarredura decide, durante o WalkDir, o que a varredura pula.
type filtroVarredura struct {
	root     string
	ignores  []string
	ignorado map[string]bool // caminhos que o git diz ignorados (pastas sem a barra final)
}

// novoFiltro monta o filtro de uma varredura em root. Os ignores valem sobre o
// caminho relativo à raiz, então uma raiz dentro de .../vendor/proj não é pulada.
// Havendo git, pula também o que o .gitignore ignora; o resto (inclusive o
// conteúdo de submódulos e de repositórios aninhados) continua na varredura.
func novoFiltro(root string, ignores []string) *filtroVarredura {
	return &filtroVarredura{root: root, ignores: ignores, ignorado: ignoradosDoGit(root)}
}

// pular diz se o item deve ser pulado (e, sendo pasta, se a pasta toda sai).
func (f *filtroVarredura) pular(path string, d fs.DirEntry) bool {
	rel, err := relSlash(f.root, path)
	if err != nil || rel == "." {
		return false
	}
	return ShouldIgnore(rel, f.ignores) || f.ignorado[rel]
}

// ignoradosDoGit lista, com barra normal e relativos a root, os caminhos que o
// .gitignore ignora (arquivos e pastas inteiras). É uma lista de bloqueio: o que
// o git não conhece, como o conteúdo de submódulos, não entra nela e é varrido.
// Devolve nil quando root não está num repositório git ou o git não está instalado.
func ignoradosDoGit(root string) map[string]bool {
	ctx, cancel := context.WithTimeout(context.Background(), prazoGit)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "-z",
		"--others", "--ignored", "--exclude-standard", "--directory")
	saida, err := cmd.Output()
	if err != nil {
		return nil
	}
	ignorados := make(map[string]bool)
	for _, p := range bytes.Split(saida, []byte{0}) {
		if len(p) > 0 {
			ignorados[strings.TrimSuffix(filepath.ToSlash(string(p)), "/")] = true
		}
	}
	return ignorados
}
