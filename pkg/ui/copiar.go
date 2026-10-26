package ui

import (
	"io"
	"os"
	"strings"

	"github.com/aymanbagabas/go-osc52/v2"
)

// CopiarOSC52 manda o texto para a área de transferência do terminal pela sequência
// OSC 52, que atravessa SSH. Dentro do tmux e do screen a sequência vai embrulhada
// para chegar ao terminal de fora. O OSC 52 não tem resposta: o terminal que não
// aceita a sequência a descarta calado, por isso quem chama diz que o texto foi
// enviado, não que foi copiado.
func CopiarOSC52(texto string) error {
	return EscreverOSC52(os.Stdout, os.Getenv, texto)
}

// EscreverOSC52 escreve em w a sequência OSC 52 com o texto, embrulhada para o tmux
// ou o screen conforme as variáveis de ambiente lidas por getenv.
func EscreverOSC52(w io.Writer, getenv func(string) string, texto string) error {
	seq := osc52.New(texto)
	switch {
	case getenv("TMUX") != "":
		seq = seq.Tmux()
	case strings.HasPrefix(getenv("TERM"), "screen"):
		seq = seq.Screen()
	}
	_, err := seq.WriteTo(w)
	return err
}
