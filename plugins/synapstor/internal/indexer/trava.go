package indexer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

// arquivoDaTrava fica solto na raiz da pasta do índice: o git ignora a pasta e o chromem
// ignora os arquivos soltos nela.
const arquivoDaTrava = "index.lock"

// ErrIndexEmAndamento é devolvido quando outro index já está gravando o índice do projeto.
var ErrIndexEmAndamento = errors.New("outro index em andamento neste projeto")

// travar pega a trava do índice em storePath e devolve a função que a solta. A trava é do
// sistema operacional (flock no Unix, LockFileEx no Windows) e sai sozinha quando o
// processo termina, até num ctrl+c ou numa queda: não existe trava velha a limpar. O
// arquivo fica na pasta; o que vale é a trava sobre ele, não a existência.
func travar(storePath string) (func(), error) {
	if err := os.MkdirAll(storePath, 0o755); err != nil {
		return nil, fmt.Errorf("não deu para criar a pasta do índice: %w", err)
	}
	trava := flock.New(filepath.Join(storePath, arquivoDaTrava))
	ok, err := trava.TryLock()
	if err != nil {
		return nil, fmt.Errorf("não deu para travar o índice: %w", err)
	}
	if !ok {
		return nil, ErrIndexEmAndamento
	}
	return func() { _ = trava.Unlock() }, nil
}

// esvaziarIndice apaga tudo o que há na pasta do índice, menos a trava, que está presa por
// este processo (no Windows, nem daria para apagá-la).
func esvaziarIndice(storePath string) error {
	entradas, err := os.ReadDir(storePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, e := range entradas {
		if e.Name() == arquivoDaTrava {
			continue
		}
		if err := os.RemoveAll(filepath.Join(storePath, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
