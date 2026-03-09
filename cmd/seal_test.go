package cmd

import "testing"

func TestValidateSecretKey(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
		errMsg  string
	}{
		{"chave válida simples", "good-key", false, ""},
		{"chave com pontos", "my.key", false, ""},
		{"chave com underscore", "my_key", false, ""},
		{"chave alfanumérica", "key123", false, ""},
		{"chave com maiúsculas", "MyKey", false, ""},
		{"chave com todos os caracteres válidos", "a-z.A-Z_0-9", false, ""},
		{"chave com igual", "key=value", true, "não pode conter '='"},
		{"chave com espaço", "key value", true, "caractere inválido"},
		{"chave com arroba", "key@value", true, "caractere inválido"},
		{"chave com barra", "key/value", true, "caractere inválido"},
		{"chave com dois pontos", "key:value", true, "caractere inválido"},
		{"chave com exclamação", "key!value", true, "caractere inválido"},
		{"chave com hashtag", "key#value", true, "caractere inválido"},
		{"chave com asterisco", "key*value", true, "caractere inválido"},
		{"chave com acento", "chave-açúcar", true, "caractere inválido"},
		{"chave vazia (string)", "", true, "obrigatória"},
		{"tipo inválido (não string)", 123, true, "obrigatória"},
		{"tipo nil", nil, true, "obrigatória"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSecretKey(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSecretKey(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("validateSecretKey(%v) erro = %q, esperado conter %q", tt.input, err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestValidateSecretKey_ChaveLonga(t *testing.T) {
	// Chave longa mas válida (253 caracteres - limite do K8s)
	longKey := ""
	for i := 0; i < 253; i++ {
		longKey += "a"
	}
	err := validateSecretKey(longKey)
	if err != nil {
		t.Errorf("Chave longa válida não deveria retornar erro, obtido: %v", err)
	}
}

func TestValidateSecretKey_ApenasNumeros(t *testing.T) {
	err := validateSecretKey("12345")
	if err != nil {
		t.Errorf("Chave apenas com números deveria ser válida, obtido: %v", err)
	}
}

func TestValidateSecretKey_ApenasCaracteresEspeciaisValidos(t *testing.T) {
	err := validateSecretKey(".-_")
	if err != nil {
		t.Errorf("Chave com .-_ deveria ser válida, obtido: %v", err)
	}
}

func TestSealCmd_Estrutura(t *testing.T) {
	if sealCmd.Use != "seal" {
		t.Errorf("sealCmd.Use = %q, esperado 'seal'", sealCmd.Use)
	}

	if sealCmd.Short == "" {
		t.Error("sealCmd.Short não deveria ser vazio")
	}

	if sealCmd.Long == "" {
		t.Error("sealCmd.Long não deveria ser vazio")
	}

	if sealCmd.Run == nil {
		t.Error("sealCmd.Run não deveria ser nil")
	}
}

func TestSealCmd_EhSubcomandoDeSecrets(t *testing.T) {
	found := false
	for _, sub := range secretsCmd.Commands() {
		if sub.Name() == "seal" {
			found = true
			break
		}
	}
	if !found {
		t.Error("seal deveria ser subcomando de secrets")
	}
}
