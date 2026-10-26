package ai

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestCausaForaDoFormato(t *testing.T) {
	var alvo struct {
		Confianca int `json:"confidence"`
	}
	errTexto := json.Unmarshal([]byte("Desculpe, não consegui analisar"), &alvo)
	errTipo := json.Unmarshal([]byte(`{"confidence": "alta"}`), &alvo)
	errCortado := json.Unmarshal([]byte(`{"confidence": 8`), &alvo)
	outro := errors.New("timeout")

	casos := []struct {
		nome     string
		entrada  error
		esperado error
	}{
		{"texto em vez de JSON", errTexto, ErrRespostaNaoEJSON},
		{"JSON cortado", errCortado, ErrRespostaNaoEJSON},
		{"tipo errado", errTipo, ErrRespostaComTipoErrado},
		{"outro erro", outro, outro},
		{"nil", nil, nil},
	}
	for _, c := range casos {
		if got := CausaForaDoFormato(c.entrada); got != c.esperado {
			t.Errorf("%s: obtido %v, esperado %v", c.nome, got, c.esperado)
		}
	}
}
