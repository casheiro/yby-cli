//go:build windows

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// criarSemJanela é o CREATE_NO_WINDOW do CreateProcess: o cmd.exe ganha um console sem
// janela, que o ping dentro dele herda. O DETACHED_PROCESS não serve: sem console, o
// ping abriria uma janela própria.
const criarSemJanela = 0x08000000

// variavelRemocao leva o caminho ao cmd.exe pelo ambiente do processo filho. Montado na
// linha, o caminho passaria pela expansão do cmd: um trecho %NOME% de uma variável
// definida viraria outro caminho. O %YBY_REMOVER% da linha se expande uma vez só, sem
// reprocessar o valor, e as aspas em volta seguram espaço, & e parênteses.
const variavelRemocao = "YBY_REMOVER"

// tentativasRemocao é quantas vezes, uma por segundo, o cmd.exe tenta apagar o arquivo
// antes de desistir: o yby que abriu um plugin (yby viz em outro terminal) fica em uso
// enquanto o plugin roda.
const tentativasRemocao = 60

// linhaRemocao monta a linha do cmd.exe que espera o yby sair e apaga o arquivo de
// %YBY_REMOVER%: tenta a cada segundo, por até tentativasRemocao segundos, e para quando o
// arquivo some. Com /s, o cmd tira só a primeira e a última aspas; /v:off desliga a
// expansão com ! mesmo que o registro a ligue.
func linhaRemocao(interpretador string) string {
	return fmt.Sprintf(`"%s" /d /v:off /s /c "for /l %%i in (1,1,%d) do @(ping -n 2 127.0.0.1 >nul & del /f /q "%%%s%%" 2>nul & if not exist "%%%s%%" exit)"`,
		interpretador, tentativasRemocao, variavelRemocao, variavelRemocao)
}

// agendarRemocao apaga caminho depois que o yby sair. O Windows não deixa apagar o
// executável em uso (só renomear): um cmd.exe sem janela, fora do grupo de processos do
// terminal, espera o yby fechar e apaga o arquivo.
func agendarRemocao(caminho string) error {
	interpretador := os.Getenv("ComSpec")
	if interpretador == "" {
		interpretador = "cmd.exe"
	}
	c := exec.Command(interpretador)
	c.Env = append(os.Environ(), variavelRemocao+"="+caminho)
	c.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       linhaRemocao(interpretador),
		HideWindow:    true,
		CreationFlags: criarSemJanela | syscall.CREATE_NEW_PROCESS_GROUP,
	}
	if err := c.Start(); err != nil {
		return err
	}
	return c.Process.Release()
}
