package ui

// Seta indica de que lado da janela está o conteúdo que não aparece.
type Seta int

const (
	// SetaAcima marca conteúdo acima da janela ("↑ 2 mensagens anteriores").
	SetaAcima Seta = iota
	// SetaAbaixo marca conteúdo abaixo da janela ("↓ 3 mensagens novas").
	SetaAbaixo
)

// AvisoRolagem devolve a nota de conteúdo fora da janela, em TintaSuave e sem recuo:
// "↑ 2 mensagens anteriores" ou "↓ 3 mensagens novas". No modo ASCII as setas viram
// ^ e v; a seta ocupa sempre 1 célula.
func AvisoRolagem(seta Seta, texto string) string {
	acima, abaixo := "↑", "↓"
	if Atual().ASCII {
		acima, abaixo = "^", "v"
	}
	simbolo := abaixo
	if seta == SetaAcima {
		simbolo = acima
	}
	return S().TintaSuave.Render(simbolo + " " + texto)
}
