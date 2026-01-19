package mods

import (
	"fmt"
	"go/token"

	"github.com/casheiro/yby-cli/plugins/forge/internal/engine"
	"github.com/dave/dst"
)

// LogMod injects a log statement into the main function
type LogMod struct{}

func (m *LogMod) Name() string {
	return "AddLogCheckMod"
}

func (m *LogMod) Check(ctx *engine.Context) (bool, error) {
	f, err := engine.LoadGoFile(ctx, "main.go")
	if err != nil {
		return false, nil
	}

	mainFn := findFunction(f, "main")
	if mainFn == nil {
		return false, nil
	}

	// Check if already has the log
	if len(mainFn.Body.List) > 0 {
		if exprStmt, ok := mainFn.Body.List[0].(*dst.ExprStmt); ok {
			if call, ok := exprStmt.X.(*dst.CallExpr); ok {
				if sel, ok := call.Fun.(*dst.SelectorExpr); ok {
					if sel.Sel.Name == "Println" {
						if len(call.Args) > 0 {
							if lit, ok := call.Args[0].(*dst.BasicLit); ok {
								if lit.Value == "\"Forge Active 🔨\"" {
									return false, nil
								}
							}
						}
					}
				}
			}
		}
	}

	return true, nil
}

func (m *LogMod) Apply(ctx *engine.Context) error {
	f, err := engine.LoadGoFile(ctx, "main.go")
	if err != nil {
		return err
	}

	mainFn := findFunction(f, "main")
	if mainFn == nil {
		return fmt.Errorf("main function not found")
	}

	// Create the AST node for: fmt.Println("Forge Active 🔨")
	logStmt := &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent("fmt"),
				Sel: dst.NewIdent("Println"),
			},
			Args: []dst.Expr{
				&dst.BasicLit{
					Kind:  token.STRING,
					Value: "\"Forge Active 🔨\"",
				},
			},
		},
	}

	// Prepend to body
	mainFn.Body.List = append([]dst.Stmt{logStmt}, mainFn.Body.List...)

	// Ensure fmt is imported
	// Simple heuristic: check inputs. Real implementation would use dstutil.AddImport
	// For now, assuming fmt is there or let the user fix imports (Forge v1)

	return engine.SaveGoFile(ctx, "main.go", f)
}

func findFunction(f *dst.File, name string) *dst.FuncDecl {
	for _, decl := range f.Decls {
		if fn, ok := decl.(*dst.FuncDecl); ok {
			if fn.Name.Name == name {
				return fn
			}
		}
	}
	return nil
}

// Ensure interface
var _ engine.Codemod = &LogMod{}
