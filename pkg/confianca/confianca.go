// Package confianca guarda a confiança do usuário nos arquivos que vêm do projeto e
// mudam o que o yby faz: as ferramentas do Bard (.yby/tools/*.yaml) e os prompts
// (.yby/prompts/*.txt). Um repositório clonado pode trazê-los, então eles só valem depois
// que o usuário os confirma com "yby plugin trust <arquivo>". A confiança é do caminho
// absoluto com o SHA-256 do conteúdo: o arquivo mudou, ela cai.
//
// É um pacote-folha (só a biblioteca padrão), para o pkg/ai/prompts e os plugins o usarem
// sem puxar o resto do core.
package confianca

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// entrada é um arquivo confiável no registro.
type entrada struct {
	SHA256     string    `json:"sha256"`
	ConfiadoEm time.Time `json:"confiado_em"`
}

// registro é o arquivo ~/.yby/trusted-files.json, com os arquivos pelo caminho absoluto.
type registro struct {
	Arquivos map[string]entrada `json:"arquivos"`
}

// caminhoRegistro devolve onde fica o registro; os testes trocam.
var caminhoRegistro = func() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("falha ao obter o diretório home: %w", err)
	}
	return filepath.Join(home, ".yby", "trusted-files.json"), nil
}

var mu sync.Mutex

// chaveDe é como o arquivo aparece no registro: o caminho absoluto limpo (no Windows, em
// minúsculas, porque lá o caminho não diferencia maiúsculas).
func chaveDe(caminho string) (string, error) {
	abs, err := filepath.Abs(caminho)
	if err != nil {
		return "", fmt.Errorf("falha ao resolver o caminho %s: %w", caminho, err)
	}
	abs = filepath.Clean(abs)
	if runtime.GOOS == "windows" {
		abs = strings.ToLower(abs)
	}
	return abs, nil
}

// Confiar registra o arquivo como confiável, com o SHA-256 do conteúdo de agora.
func Confiar(caminho string) error {
	dados, err := os.ReadFile(caminho)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("o arquivo %s não existe", caminho)
	}
	if err != nil {
		return fmt.Errorf("falha ao ler %s: %w", caminho, err)
	}
	chave, err := chaveDe(caminho)
	if err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()
	reg, err := carregar()
	if err != nil {
		return err
	}
	reg.Arquivos[chave] = entrada{SHA256: soma(dados), ConfiadoEm: time.Now().UTC()}
	return salvar(reg)
}

// Remover tira o arquivo do registro; erro se ele não estava lá.
func Remover(caminho string) error {
	chave, err := chaveDe(caminho)
	if err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()
	reg, err := carregar()
	if err != nil {
		return err
	}
	if _, ok := reg.Arquivos[chave]; !ok {
		return fmt.Errorf("o arquivo %s não está no registro de confiança", caminho)
	}
	delete(reg.Arquivos, chave)
	return salvar(reg)
}

// Confiavel diz se o conteúdo dado é o que o usuário confirmou para esse caminho. Quem
// chama passa o que leu do arquivo e usa só isso, para o arquivo não trocar entre a
// conferência e o uso. Registro ilegível conta como nenhuma confiança.
func Confiavel(caminho string, conteudo []byte) bool {
	chave, err := chaveDe(caminho)
	if err != nil {
		return false
	}

	mu.Lock()
	defer mu.Unlock()
	reg, err := carregar()
	if err != nil {
		return false
	}
	e, ok := reg.Arquivos[chave]
	return ok && e.SHA256 == soma(conteudo)
}

// soma é o SHA-256 do conteúdo em hexadecimal.
func soma(dados []byte) string {
	h := sha256.Sum256(dados)
	return hex.EncodeToString(h[:])
}

// carregar lê o registro; sem o arquivo, o registro está vazio.
func carregar() (*registro, error) {
	caminho, err := caminhoRegistro()
	if err != nil {
		return nil, err
	}
	reg := &registro{}
	dados, err := os.ReadFile(caminho)
	switch {
	case os.IsNotExist(err):
	case err != nil:
		return nil, fmt.Errorf("falha ao ler o registro de confiança: %w", err)
	default:
		if err := json.Unmarshal(dados, reg); err != nil {
			return nil, fmt.Errorf("falha ao analisar o registro de confiança %s: %w", caminho, err)
		}
	}
	if reg.Arquivos == nil {
		reg.Arquivos = map[string]entrada{}
	}
	return reg, nil
}

// salvar grava o registro, só para o usuário.
func salvar(reg *registro) error {
	caminho, err := caminhoRegistro()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
		return fmt.Errorf("falha ao criar %s: %w", filepath.Dir(caminho), err)
	}
	dados, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("falha ao serializar o registro de confiança: %w", err)
	}
	if err := os.WriteFile(caminho, dados, 0o600); err != nil {
		return fmt.Errorf("falha ao salvar o registro de confiança: %w", err)
	}
	return nil
}
