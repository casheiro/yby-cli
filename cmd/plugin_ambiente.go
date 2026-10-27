package cmd

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/casheiro/yby-cli/pkg/errors"
)

// linhaDoYby devolve os argumentos do yby, sem o nome do binário. Variável para os testes.
var linhaDoYby = func() []string { return os.Args[1:] }

// flagsGlobaisComValor são as flags do rootCmd que levam o valor no argumento seguinte.
var flagsGlobaisComValor = map[string]bool{
	"-c": true, "--context": true, "--log-level": true, "--log-format": true,
}

// argsDoPlugin separa a linha do yby em torno do nome do plugin: as flags antes do nome
// são do core (o cobra não as lê, porque o comando do plugin desliga o parse de flags e
// as entrega ao plugin junto com as dele), e o que vem depois é do plugin. args é o que o
// cobra entregou. Devolve o ambiente pedido (o -c/--context antes do nome; sem ele, o
// YBY_ENV) e os args do plugin. Se a linha não bate com args (ex.: o nome não veio depois
// só de flags), os args seguem como estão.
//
// Flag antes do nome que o core não reconhece é erro, e não some: descartada em silêncio,
// uma forma do -c que escapasse daqui faria o plugin rodar no current-context do
// kubeconfig em vez do ambiente pedido.
func argsDoPlugin(linha []string, nome string, args []string) (ambiente string, doPlugin []string, err error) {
	ambiente = os.Getenv("YBY_ENV")
	for i := 0; i < len(linha); i++ {
		a := linha[i]
		if a == nome {
			antes, depois := linha[:i], linha[i+1:]
			if !slices.Equal(args, append(slices.Clone(antes), depois...)) {
				return ambiente, args, nil
			}
			contexto, err := flagsDoCore(antes, nome)
			if err != nil {
				return "", nil, err
			}
			if contexto != "" {
				ambiente = contexto
			}
			return ambiente, depois, nil
		}
		if !strings.HasPrefix(a, "-") {
			break
		}
		if flagsGlobaisComValor[a] {
			i++
		}
	}
	return ambiente, args, nil
}

// flagsDoCore lê as flags antes do nome do plugin e devolve o valor do -c/--context (o
// último vale, como no cobra). Aceita as formas do pflag: "-c x", "-cx", "-c=x",
// "--context x" e "--context=x", e as duas formas do --log-level e do --log-format.
// Qualquer outra flag é erro. (Hoje o cobra já barra a flag desconhecida antes do nome,
// porque toma o nome do plugin como valor dela; o erro aqui não depende disso.)
func flagsDoCore(flags []string, nome string) (contexto string, err error) {
	for i := 0; i < len(flags); i++ {
		a := flags[i]
		switch {
		case flagsGlobaisComValor[a]:
			// o laço de argsDoPlugin já pulou o valor: ele está em flags[i+1]
			if (a == "-c" || a == "--context") && i+1 < len(flags) {
				contexto = flags[i+1]
			}
			i++
		case strings.HasPrefix(a, "--context="):
			contexto = strings.TrimPrefix(a, "--context=")
		case strings.HasPrefix(a, "--log-level="), strings.HasPrefix(a, "--log-format="):
		case strings.HasPrefix(a, "-c="):
			contexto = strings.TrimPrefix(a, "-c=")
		case strings.HasPrefix(a, "-c") && !strings.HasPrefix(a, "--"):
			// valor colado ao -c, como o pflag aceita: yby -cprod sentinel
			contexto = strings.TrimPrefix(a, "-c")
		default:
			return "", errors.New(errors.ErrCodeValidation,
				fmt.Sprintf("a flag %s antes de %s não é do yby", a, nome)).
				WithHint(fmt.Sprintf("Antes de %s, o yby aceita só -c/--context, --log-level e --log-format. Para passar a flag ao plugin, escreva-a depois do nome: yby %s %s", nome, nome, a))
		}
	}
	return contexto, nil
}
