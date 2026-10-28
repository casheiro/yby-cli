package ai

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// limiteStderrCLI é o tanto do stderr de um CLI que entra na mensagem de erro.
const limiteStderrCLI = 2048

// esperaEncerrarCLI é o prazo, depois do cancelamento, para o CLI soltar os pipes. Num
// shim do npm no Windows o cancelamento mata só o cmd.exe e o node segue segurando a saída.
const esperaEncerrarCLI = 2 * time.Second

// executarCLI roda o CLI com o prompt pelo stdin (argumento tem limite de tamanho, o stdin
// não) e mata o processo quando o ctx é cancelado. O stderr volta limitado.
func executarCLI(ctx context.Context, comando string, args []string, prompt string, stdout io.Writer) (string, error) {
	cmd := exec.CommandContext(ctx, comando, args...)
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Stdout = stdout
	cmd.WaitDelay = esperaEncerrarCLI

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	return limitarStderr(stderr.String()), err
}

// limitarStderr guarda o fim do stderr, que é onde o CLI diz o motivo da falha.
func limitarStderr(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= limiteStderrCLI {
		return s
	}
	return "..." + s[len(s)-limiteStderrCLI:]
}

// erroCLI monta o erro de uma execução que falhou.
func erroCLI(nome, etapa string, err error, stderr string) error {
	if stderr == "" {
		return fmt.Errorf("%s%s falhou: %w", nome, etapa, err)
	}
	return fmt.Errorf("%s%s falhou: %w (stderr: %s)", nome, etapa, err, stderr)
}
