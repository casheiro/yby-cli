//go:build !windows

package cmd

import "os"

// agendarRemocao apaga caminho. Fora do Windows, o arquivo de um executável em uso pode
// ser apagado na hora: o processo continua com a cópia que já abriu.
func agendarRemocao(caminho string) error {
	return os.Remove(caminho)
}
