package ui

import (
	"os"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// homesDoUsuario devolve as pastas que valem como a home para AbreviarHome: a do
// os.UserHomeDir e as de HOME e USERPROFILE (no Windows as duas podem existir e diferir,
// como no Git Bash). Variável para os testes.
var homesDoUsuario = func() []string {
	var homes []string
	if h, err := os.UserHomeDir(); err == nil {
		homes = append(homes, h)
	}
	return append(homes, os.Getenv("HOME"), os.Getenv("USERPROFILE"))
}

// separadorDeCaminho diz se r separa as pastas de um caminho (/ em todo sistema, \ no
// Windows e em caminhos vindos dele).
func separadorDeCaminho(r byte) bool { return r == '/' || r == '\\' }

// AbreviarHome troca a home do usuário no começo de caminho por ~, no Windows também
// (C:\Users\ana\.yby\reports vira ~\.yby\reports); o resto do caminho fica como veio.
// Fora da home, devolve o caminho sem mudança.
func AbreviarHome(caminho string) string {
	for _, home := range homesDoUsuario() {
		home = strings.TrimRight(home, `/\`)
		if home == "" || len(caminho) < len(home) {
			continue
		}
		prefixo := caminho[:len(home)]
		igual := prefixo == home
		if runtime.GOOS == "windows" {
			igual = strings.EqualFold(strings.ReplaceAll(prefixo, `\`, "/"), strings.ReplaceAll(home, `\`, "/"))
		}
		if !igual {
			continue
		}
		if len(caminho) == len(home) {
			return "~"
		}
		if separadorDeCaminho(caminho[len(home)]) {
			return "~" + caminho[len(home):]
		}
	}
	return caminho
}

// QuebrarCaminho divide caminho em linhas de até largura células, quebrando só depois de
// um separador (/ ou \), que fica no fim da linha: um nome de pasta ou de arquivo nunca se
// parte, e um nome mais largo que a linha fica inteiro na sua. Com largura menor que
// minQuebra, ou quando o caminho já cabe, devolve o caminho como está.
func QuebrarCaminho(caminho string, largura int) []string {
	if largura < minQuebra || lipgloss.Width(caminho) <= largura {
		return []string{caminho}
	}
	// pedaços: cada nome com o separador que vem depois dele
	var pedacos []string
	ini := 0
	for i := 0; i < len(caminho); i++ {
		if separadorDeCaminho(caminho[i]) {
			pedacos = append(pedacos, caminho[ini:i+1])
			ini = i + 1
		}
	}
	if ini < len(caminho) {
		pedacos = append(pedacos, caminho[ini:])
	}
	var linhas []string
	atual := ""
	for _, p := range pedacos {
		if atual != "" && lipgloss.Width(atual)+lipgloss.Width(p) > largura {
			linhas = append(linhas, atual)
			atual = ""
		}
		atual += p
	}
	return append(linhas, atual)
}
