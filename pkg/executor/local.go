package executor

import (
	"os"
	"os/exec"

	"github.com/casheiro/yby-cli/pkg/services/shared"
)

// LocalExecutor roda os scripts nesta máquina, via bash -c.
type LocalExecutor struct {
	// Etapas recebe o andamento de cada Run; nil escreve em os.Stdout como ui.Steps.
	Etapas shared.Etapas
	// Executar roda o script e devolve a saída combinada; nil usa bash -c. Substituível
	// em testes.
	Executar func(script string) ([]byte, error)
	// LerArquivo lê um arquivo local; nil usa os.ReadFile. Substituível em testes.
	LerArquivo func(path string) ([]byte, error)
	// LerComSudo lê o arquivo com sudo depois de uma permissão negada; nil usa sudo cat.
	LerComSudo func(path string) ([]byte, error)
}

// pausavel são etapas que saem da tela enquanto outro programa fala com o terminal
// (*ui.Progresso).
type pausavel interface{ Pausar() (retomar func()) }

func NewLocalExecutor() *LocalExecutor {
	return &LocalExecutor{}
}

func (e *LocalExecutor) etapas() shared.Etapas {
	if e.Etapas == nil {
		e.Etapas = etapasPadrao()
	}
	return e.Etapas
}

func (e *LocalExecutor) Run(name, script string) error {
	executar := e.Executar
	if executar == nil {
		// bash -c aceita o script inteiro; a saída fica guardada para mostrar na falha,
		// como no SSHExecutor
		executar = func(script string) ([]byte, error) {
			return exec.Command("bash", "-c", script).CombinedOutput()
		}
	}
	return rodarEtapa(e.etapas(), name, func() ([]byte, error) { return executar(script) })
}

func (e *LocalExecutor) FetchFile(path string) ([]byte, error) {
	ler := e.LerArquivo
	if ler == nil {
		ler = os.ReadFile
	}
	data, err := ler(path)
	if err == nil {
		return data, nil
	}

	// If Permission Denied, try sudo
	if os.IsPermission(err) {
		e.etapas().Detalhe("permissão negada em " + path + " · lendo com sudo")
		// o sudo pede a senha direto no terminal: o redesenho das etapas espera, para
		// não apagar o pedido
		if p, ok := e.etapas().(pausavel); ok {
			retomar := p.Pausar()
			defer retomar()
		}
		sudo := e.LerComSudo
		if sudo == nil {
			sudo = func(path string) ([]byte, error) {
				return exec.Command("sudo", "cat", path).CombinedOutput()
			}
		}
		return sudo(path)
	}

	return nil, err
}

func (e *LocalExecutor) Close() error {
	return nil
}
