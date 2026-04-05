package cmd

import "fmt"

// mockLookPath configura a variável lookPath para testes.
// Retorna uma função de teardown que restaura o valor original.
func mockLookPath() func() {
	originalLookPath := lookPath
	lookPath = func(file string) (string, error) {
		if file == "fail-tool" || file == "missing-tool" {
			return "", fmt.Errorf("tool not found: %s", file)
		}
		return "/usr/bin/" + file, nil
	}

	return func() {
		lookPath = originalLookPath
	}
}
