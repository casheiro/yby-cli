package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ========================================================
// validateSecretKey — edge cases adicionais
// ========================================================

func TestValidateSecretKey_CaracteresUnicode(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{"emoji", "key-🔑", true},
		{"japones", "キー", true},
		{"cedilha", "chave-ção", true},
		{"til", "chave~nome", true},
		{"circunflexo", "chave^nome", true},
		{"sinal de porcentagem", "key%value", true},
		{"dolar", "key$value", true},
		{"ampersand", "key&value", true},
		{"parenteses", "key(value)", true},
		{"colchetes", "key[value]", true},
		{"chaves", "key{value}", true},
		{"pipe", "key|value", true},
		{"backslash", `key\value`, true},
		{"aspas simples", "key'value", true},
		{"aspas duplas", `key"value`, true},
		{"ponto-e-virgula", "key;value", true},
		{"virgula", "key,value", true},
		{"maior/menor", "key<value>", true},
		{"interrogacao", "key?value", true},
		{"acento agudo", "chavé", true},
		{"acento grave", "chàve", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSecretKey(tt.input)
			if tt.wantErr {
				assert.Error(t, err, "validateSecretKey(%q) deveria retornar erro", tt.input)
			} else {
				assert.NoError(t, err, "validateSecretKey(%q) não deveria retornar erro", tt.input)
			}
		})
	}
}

func TestValidateSecretKey_ChavesValidasComplexas(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"apenas letras minúsculas", "abcdef"},
		{"apenas letras maiúsculas", "ABCDEF"},
		{"mistura de maiúsculas e minúsculas", "AbCdEf"},
		{"números no início", "123key"},
		{"ponto no início", ".hidden-key"},
		{"underscore no início", "_private"},
		{"hifen no início", "-flag"},
		{"combinação completa", "my-App_Config.v2"},
		{"um único caractere", "a"},
		{"um número", "1"},
		{"um ponto", "."},
		{"um underscore", "_"},
		{"um hifen", "-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSecretKey(tt.input)
			assert.NoError(t, err, "validateSecretKey(%q) não deveria retornar erro", tt.input)
		})
	}
}

func TestValidateSecretKey_MensagensDeErroEspecificas(t *testing.T) {
	// Verifica que o sinal de igual tem mensagem específica
	err := validateSecretKey("chave=valor")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "não pode conter '='",
		"Erro para '=' deveria ter mensagem específica")

	// Verifica que caractere inválido mostra o caractere no erro
	err = validateSecretKey("chave@valor")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "@",
		"Erro deveria mostrar o caractere inválido")

	// Verifica que string vazia indica obrigatoriedade
	err = validateSecretKey("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "obrigatória",
		"Erro para string vazia deveria mencionar obrigatoriedade")
}

func TestValidateSecretKey_TiposInvalidos(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
	}{
		{"inteiro", 42},
		{"float", 3.14},
		{"boolean true", true},
		{"boolean false", false},
		{"slice", []string{"a", "b"}},
		{"nil", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSecretKey(tt.input)
			assert.Error(t, err, "validateSecretKey(%v) deveria retornar erro para tipo %T", tt.input, tt.input)
			assert.Contains(t, err.Error(), "obrigatória")
		})
	}
}

func TestValidateSecretKey_ChaveComEspacosInternos(t *testing.T) {
	// Espaço no meio da chave
	err := validateSecretKey("my key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "caractere inválido")

	// Tab na chave
	err = validateSecretKey("my\tkey")
	assert.Error(t, err)

	// Newline na chave
	err = validateSecretKey("my\nkey")
	assert.Error(t, err)
}

func TestValidateSecretKey_PrimeiroCaractereInvalido(t *testing.T) {
	// Garante que a validação pega o primeiro caractere inválido
	err := validateSecretKey(" leadingspace")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), " ",
		"Deveria reportar o espaço como caractere inválido")
}

func TestValidateSecretKey_ChaveMuitoLonga(t *testing.T) {
	// Kubernetes tem limite de 253 caracteres para nomes, mas a função
	// não valida tamanho - apenas caracteres. Chave de 1000 chars deveria passar.
	longKey := strings.Repeat("a", 1000)
	err := validateSecretKey(longKey)
	assert.NoError(t, err, "Chave longa com caracteres válidos deveria ser aceita")
}

// ========================================================
// sealCmd — verificação de estrutura
// ========================================================

func TestSealCmd_TemRunDefinido(t *testing.T) {
	// Run (não RunE) deve estar definido
	assert.NotNil(t, sealCmd.Run, "sealCmd.Run não deveria ser nil")
	assert.Nil(t, sealCmd.RunE, "sealCmd.RunE deveria ser nil (usa Run)")
}

func TestSealCmd_LongDescription(t *testing.T) {
	assert.Contains(t, sealCmd.Long, "SealedSecrets",
		"Descrição longa deveria mencionar SealedSecrets")
	assert.Contains(t, sealCmd.Long, "kubeseal",
		"Descrição longa deveria mencionar kubeseal")
}
