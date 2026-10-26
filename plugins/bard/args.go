package main

import (
	"strings"

	yerrors "github.com/casheiro/yby-cli/pkg/errors"
)

// opcoesBard são os args do hook command (o core passa as flags do yby bard sem ler).
type opcoesBard struct {
	ajuda bool
	// prompt é a pergunta do -p, que responde e sai; temPrompt diz que o -p veio (sem
	// pergunta é erro).
	prompt    string
	temPrompt bool
	// pergunta abre a TUI com o campo já escrito, sem enviar: a pessoa revisa e aperta ↵
	// (o b do Viz abre o Bard assim, com a pergunta sobre o recurso selecionado).
	pergunta string
	// contexto e namespace são o cluster da sessão no lugar do contexto atual do
	// kubectl: o cabeçalho, o contexto do cluster no prompt e o kubectl das ferramentas.
	contexto  string
	namespace string
	// voltar é o instrumento que abriu o Bard (o Viz passa --voltar Viz): fora da
	// resposta, o esc fecha o Bard e volta a ele.
	voltar string
}

// flagsComValor são as flags que levam um valor, com o campo que ele preenche.
var flagsComValor = map[string]func(o *opcoesBard, v string){
	"--pergunta":  func(o *opcoesBard, v string) { o.pergunta = v },
	"--context":   func(o *opcoesBard, v string) { o.contexto = v },
	"--namespace": func(o *opcoesBard, v string) { o.namespace = v },
	"-n":          func(o *opcoesBard, v string) { o.namespace = v },
	"--voltar":    func(o *opcoesBard, v string) { o.voltar = v },
}

// lerArgs lê os args do yby bard. O -p junta tudo o que vem depois dele numa pergunta,
// como sempre; as outras flags valem antes dele. Args desconhecidos são ignorados.
func lerArgs(args []string) (opcoesBard, error) {
	var o opcoesBard
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--help", "-h", "help":
			o.ajuda = true
			return o, nil
		case "-p", "--prompt":
			o.temPrompt = true
			o.prompt = strings.TrimSpace(strings.Join(args[i+1:], " "))
			return o, nil
		}
		nome, valor, comIgual := strings.Cut(arg, "=")
		definir, ok := flagsComValor[nome]
		if !ok {
			continue
		}
		if !comIgual {
			if i+1 >= len(args) {
				return o, semValor(nome)
			}
			i++
			valor = args[i]
		}
		valor = strings.TrimSpace(valor)
		if valor == "" {
			return o, semValor(nome)
		}
		definir(&o, valor)
	}
	return o, nil
}

// semValor é o erro da flag sem o valor.
func semValor(flag string) error {
	exemplo := map[string]string{
		"--pergunta":  `yby bard --pergunta "por que o pod reinicia?"`,
		"--context":   "yby bard --context k3d-yby-local",
		"--namespace": "yby bard --namespace payments",
		"-n":          "yby bard -n payments",
		"--voltar":    "yby bard --voltar Viz",
	}[flag]
	return yerrors.New(yerrors.ErrCodeValidation, "falta o valor de "+flag).
		WithHint("use " + exemplo)
}

// rascunhoSemTerminal é o erro do --pergunta sem terminal: o rascunho só existe na TUI.
func rascunhoSemTerminal() error {
	return yerrors.New(yerrors.ErrCodeValidation, "o --pergunta abre a conversa do Bard e precisa de um terminal").
		WithHint(`use yby bard -p "pergunta" para responder sem a TUI`)
}
