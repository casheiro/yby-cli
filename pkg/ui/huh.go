package ui

import (
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// HuhTheme devolve o tema do huh derivado dos tokens da carta do terminal: título em
// caixa-alta e negrito, "› " como prompt, descrição em TintaSuave, erro em Erro e
// opção selecionada marcada com ◆. É resolvido na hora da chamada, então respeita o tema,
// o perfil de cor e o modo ASCII configurados.
func HuhTheme() *huh.Theme {
	s := S()
	g := G()
	t := huh.ThemeBase()

	t.Form.Base = lipgloss.NewStyle()
	t.Group.Base = lipgloss.NewStyle()
	t.Group.Title = s.Negrito.Transform(strings.ToUpper)
	t.Group.Description = s.TintaSuave
	t.FieldSeparator = lipgloss.NewStyle().SetString("\n")

	estilo := func(f *huh.FieldStyles, foco bool) {
		f.Base = lipgloss.NewStyle().PaddingLeft(1)
		f.Title = s.Negrito.Transform(strings.ToUpper)
		f.NoteTitle = f.Title
		f.Description = s.TintaSuave
		f.ErrorIndicator = s.Erro.SetString(" " + g.Falha)
		f.ErrorMessage = s.Erro

		f.SelectSelector = s.Ok.SetString(g.Ativo + " ")
		f.MultiSelectSelector = s.Ok.SetString(g.Ativo + " ")
		f.NextIndicator = s.TintaSuave.SetString(" →")
		f.PrevIndicator = s.TintaSuave.SetString("← ")
		f.Option = s.Tinta
		f.SelectedOption = s.Negrito
		f.SelectedPrefix = s.Ok.SetString("[" + g.Ativo + "] ")
		f.UnselectedPrefix = s.TintaSuave.SetString("[" + g.Pendente + "] ")
		f.UnselectedOption = s.Tinta
		f.Directory = s.Tinta
		f.File = s.Tinta

		f.TextInput.Prompt = s.Ok.SetString("› ")
		f.TextInput.Text = s.Tinta
		f.TextInput.Placeholder = s.TintaSuave
		f.TextInput.Cursor = s.Tinta
		f.TextInput.CursorText = s.Tinta

		f.FocusedButton = s.Selecionado.Padding(0, 2)
		f.BlurredButton = s.TintaSuave.Padding(0, 2)
		f.Card = lipgloss.NewStyle()
		f.Next = s.Selecionado.Padding(0, 2)

		if !foco {
			f.Title = s.TintaSuave.Transform(strings.ToUpper)
			f.NoteTitle = f.Title
			f.SelectSelector = s.TintaSuave.SetString("  ")
			f.MultiSelectSelector = s.TintaSuave.SetString("  ")
			f.TextInput.Prompt = s.TintaSuave.SetString("› ")
			f.SelectedOption = s.Tinta
		}
	}
	estilo(&t.Focused, true)
	estilo(&t.Blurred, false)
	t.Help.ShortKey = s.TintaSuave
	t.Help.ShortDesc = s.TintaSuave
	t.Help.ShortSeparator = s.Fio
	t.Help.FullKey = s.TintaSuave
	t.Help.FullDesc = s.TintaSuave
	t.Help.FullSeparator = s.Fio
	t.Help.Ellipsis = s.Fio
	return t
}
