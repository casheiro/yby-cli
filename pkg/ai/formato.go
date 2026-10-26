package ai

import (
	"encoding/json"
	"errors"
	"log/slog"
)

// Causas, em PT-BR, de uma resposta da IA que não é o JSON pedido.
var (
	// ErrRespostaNaoEJSON: a IA respondeu texto (ou JSON quebrado) em vez do JSON pedido.
	ErrRespostaNaoEJSON = errors.New("a IA respondeu fora do formato esperado: veio texto em vez do JSON pedido")
	// ErrRespostaComTipoErrado: o JSON veio, mas um campo dele tem o tipo errado.
	ErrRespostaComTipoErrado = errors.New("a IA respondeu fora do formato esperado: um campo do JSON veio com o tipo errado")
)

// CausaForaDoFormato troca o erro do encoding/json ao ler a resposta da IA, que vem em
// inglês ("invalid character 'D' looking for beginning of value"), por uma causa em PT-BR
// para a tela, e manda o detalhe técnico para o log de depuração. Outros erros (e nil)
// voltam como vieram.
func CausaForaDoFormato(err error) error {
	var sintaxe *json.SyntaxError
	var tipo *json.UnmarshalTypeError
	switch {
	case errors.As(err, &sintaxe):
		slog.Debug("a resposta da IA não é um JSON", "erro", err, "posicao", sintaxe.Offset)
		return ErrRespostaNaoEJSON
	case errors.As(err, &tipo):
		slog.Debug("a resposta da IA tem um campo com o tipo errado", "erro", err, "campo", tipo.Field)
		return ErrRespostaComTipoErrado
	}
	return err
}
