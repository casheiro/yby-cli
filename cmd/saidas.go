package cmd

import (
	stderrors "errors"
	"fmt"
	"io"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/charmbracelet/x/ansi"
)

// Peças das saídas de comando curtas (secret, env, plugin, cloud…), montadas sobre
// pkg/ui no mesmo formato do doctor e do status.

// linhaEmAndamento escreve "  ◈ texto": o que o comando está fazendo agora, antes de uma
// etapa que pode demorar (reiniciar o controller, falar com a cloud).
func linhaEmAndamento(w io.Writer, texto string) {
	fmt.Fprintln(w, "  "+ui.Etapas([]ui.Etapa{{Rotulo: texto, Estado: ui.EtapaAtual}}, 0))
}

// checksAlinhados escreve linhas de verificação com as colunas do nome e do estado
// alargadas para os mais longos delas, como renderChecks faz numa seção.
func checksAlinhados(w io.Writer, linhas ...linhaCheck) {
	checksCom(w, colunasDe(linhas...), linhas...)
}

// colunasDe calcula as colunas do nome e do estado para as linhas dadas. Um relatório
// escrito em partes (seções, ou linhas antes e depois de uma etapa) passa todas as linhas
// de uma vez para que as partes fiquem alinhadas entre si.
func colunasDe(linhas ...linhaCheck) ui.Colunas {
	var nomes []string
	estado := 0
	for _, l := range linhas {
		// linha só com nome não tem coluna a alinhar
		if l.estado != "" || l.nota != "" {
			nomes = append(nomes, l.nome)
		}
		if l.nota != "" {
			estado = max(estado, ansi.StringWidth(l.estado)+2)
		}
	}
	col := ui.ColunasPara(nomes...)
	col.Versao = max(col.Versao, estado)
	return col
}

// checksCom escreve linhas de verificação com colunas já calculadas por colunasDe.
func checksCom(w io.Writer, col ui.Colunas, linhas ...linhaCheck) {
	for _, l := range linhas {
		ui.CheckCom(w, col, l.tipo, l.nome, l.estado, l.nota)
	}
}

// contagem devolve "nenhum cluster", "1 cluster" ou "3 clusters".
func contagem(n int, singular, plural string) string {
	if n == 0 {
		return "nenhum " + singular
	}
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

// Hints compartilhados pelos comandos de secret e pelos que dependem do kubectl.
const (
	// o mesmo de seal e logs quando o kubectl não está no PATH
	hintKubectl       = "rode yby setup para instalar as dependências"
	hintSealedSecrets = "confira se o Sealed Secrets está no cluster: kubectl get pods -n sealed-secrets"
)

// falhaComHint embrulha err num YbyError com o hint dado. Se a causa já trouxer um hint
// próprio (mais perto do problema, como "use --force para reinstalar"), vale o dela.
func falhaComHint(err error, codigo, mensagem, hint string) *errors.YbyError {
	var causa *errors.YbyError
	if stderrors.As(err, &causa) && causa.Hint != "" {
		hint = causa.Hint
	}
	return errors.Wrap(err, codigo, mensagem).WithHint(hint)
}

// mensagemDoErro devolve a mensagem de err sem o código entre colchetes que um YbyError
// traz em Error(): numa nota de check o código não aparece (ele sai só na linha de erro).
func mensagemDoErro(err error) string {
	var y *errors.YbyError
	if stderrors.As(err, &y) {
		if y.Cause != nil {
			return y.Message + ": " + mensagemDoErro(y.Cause)
		}
		return y.Message
	}
	return err.Error()
}

// tituloComando escreve o título do comando e a linha em branco que o separa do corpo.
func tituloComando(w io.Writer, instrumento, direita string) {
	ui.Title(w, instrumento, direita)
	fmt.Fprintln(w)
}
