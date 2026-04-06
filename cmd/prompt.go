package cmd

import (
	"github.com/charmbracelet/huh"
)

// Prompter abstrai operações de prompt interativo para facilitar testes.
type Prompter interface {
	Input(title string, defaultVal string) (string, error)
	Password(title string) (string, error)
	Confirm(title string, defaultVal bool) (bool, error)
	Select(title string, options []string, defaultVal string) (string, error)
	MultiSelect(title string, options []string, defaults []string) ([]string, error)
}

// HuhPrompter implementa Prompter usando charmbracelet/huh.
type HuhPrompter struct{}

func (p *HuhPrompter) Input(title string, defaultVal string) (string, error) {
	var val string
	if defaultVal != "" {
		val = defaultVal
	}
	err := huh.NewInput().Title(title).Value(&val).Run()
	return val, err
}

func (p *HuhPrompter) Password(title string) (string, error) {
	var val string
	err := huh.NewInput().Title(title).EchoMode(huh.EchoModePassword).Value(&val).Run()
	return val, err
}

func (p *HuhPrompter) Confirm(title string, defaultVal bool) (bool, error) {
	val := defaultVal
	err := huh.NewConfirm().Title(title).Value(&val).Run()
	return val, err
}

func (p *HuhPrompter) Select(title string, options []string, defaultVal string) (string, error) {
	var val string
	if defaultVal != "" {
		val = defaultVal
	}
	opts := make([]huh.Option[string], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(o, o)
	}
	err := huh.NewSelect[string]().Title(title).Options(opts...).Value(&val).Run()
	return val, err
}

func (p *HuhPrompter) MultiSelect(title string, options []string, defaults []string) ([]string, error) {
	var val []string
	if len(defaults) > 0 {
		val = defaults
	}
	opts := make([]huh.Option[string], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(o, o)
	}
	err := huh.NewMultiSelect[string]().Title(title).Options(opts...).Value(&val).Run()
	return val, err
}

// prompter é a instância global usada pelos comandos. Pode ser substituída em testes.
var prompter Prompter = &HuhPrompter{}
