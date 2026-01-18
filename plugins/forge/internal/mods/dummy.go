package mods

import (
	"github.com/casheiro/yby-cli/plugins/forge/internal/engine"
)

// DummyMod é um exemplo de refatoração: adiciona um comentário "Refactored by Forge" no topo do arquivo main.go
type DummyMod struct{}

func (m *DummyMod) Name() string {
	return "DummyMod"
}

func (m *DummyMod) Check(ctx *engine.Context) (bool, error) {
	// Verifica se existe main.go e se já tem o comentário
	f, err := engine.LoadGoFile(ctx, "main.go")
	if err != nil {
		// Se não achar main.go, não roda
		return false, nil
	}

	for _, c := range f.Decs.Start.All() {
		if c == "// Refactored by Forge" {
			return false, nil // Já aplicado
		}
	}

	return true, nil
}

func (m *DummyMod) Apply(ctx *engine.Context) error {
	f, err := engine.LoadGoFile(ctx, "main.go")
	if err != nil {
		return err
	}

	// Adiciona comentário no topo
	f.Decs.Start.Prepend("// Refactored by Forge")

	// Salva
	return engine.SaveGoFile(ctx, "main.go", f)
}

// Ensure interface
var _ engine.Codemod = &DummyMod{}
