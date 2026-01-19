package engine

import (
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

// Context armazena o estado da execução do Forge
type Context struct {
	RootDir string
	Fset    *token.FileSet
}

// Codemod define a interface para transformações
type Codemod interface {
	Name() string
	Check(ctx *Context) (bool, error)
	Apply(ctx *Context) error
}

// Engine orquestra a aplicação dos codemods
type Engine struct {
	Mods []Codemod
}

func NewEngine() *Engine {
	return &Engine{
		Mods: []Codemod{},
	}
}

func (e *Engine) Register(mod Codemod) {
	e.Mods = append(e.Mods, mod)
}

func (e *Engine) Run(rootDir string) error {
	ctx := &Context{
		RootDir: rootDir,
		Fset:    token.NewFileSet(),
	}

	for _, mod := range e.Mods {
		fmt.Printf("🔍 Verificando %s...\n", mod.Name())
		shouldRun, err := mod.Check(ctx)
		if err != nil {
			return fmt.Errorf("erro ao verificar mod %s: %w", mod.Name(), err)
		}

		if shouldRun {
			fmt.Printf("Reescrita: Aplicando %s...\n", mod.Name())
			if err := mod.Apply(ctx); err != nil {
				return fmt.Errorf("erro ao aplicar mod %s: %w", mod.Name(), err)
			}
			fmt.Printf("✅ %s aplicado\n", mod.Name())
		} else {
			fmt.Printf("⏭️  Pulando %s (não necessário)\n", mod.Name())
		}
	}
	return nil
}

// Helper: Carrega e parseia um arquivo Go preservando estilo
func LoadGoFile(ctx *Context, relPath string) (*dst.File, error) {
	path := filepath.Join(ctx.RootDir, relPath)
	f, err := decorator.ParseFile(ctx.Fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Helper: Salva o arquivo Go modificado
func SaveGoFile(ctx *Context, relPath string, f *dst.File) error {
	path := filepath.Join(ctx.RootDir, relPath)
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return decorator.Fprint(file, f)
}

// Helper: Verifica arquivos Go no projeto
func FindGoFiles(rootDir string) ([]string, error) {
	var files []string
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".go") {
			rel, _ := filepath.Rel(rootDir, path)
			files = append(files, rel)
		}
		return nil
	})
	return files, err
}
