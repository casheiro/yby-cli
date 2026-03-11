package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/AlecAivazis/survey/v2"
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

func TestSealCmd_TemRunEDefinido(t *testing.T) {
	// RunE deve estar definido
	assert.NotNil(t, sealCmd.RunE, "sealCmd.RunE não deveria ser nil")
}

func TestSealCmd_LongDescription(t *testing.T) {
	assert.Contains(t, sealCmd.Long, "secrets encriptados",
		"Descrição longa deveria mencionar secrets encriptados")
	assert.Contains(t, sealCmd.Long, "sealed-secrets",
		"Descrição longa deveria mencionar sealed-secrets")
}

// ========================================================
// sealCmd.Run — testes de fluxo com mocks
// ========================================================

// captureOutput captura a saída padrão durante a execução de uma função
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestSealCmd_KubectlNotFound(t *testing.T) {
	origLookPath := lookPath
	defer func() { lookPath = origLookPath }()

	lookPath = func(file string) (string, error) {
		if file == "kubectl" {
			return "", fmt.Errorf("not found")
		}
		return "/usr/bin/" + file, nil
	}

	err := sealCmd.RunE(sealCmd, []string{})

	assert.Error(t, err, "Deveria retornar erro quando kubectl não encontrado")
	assert.Contains(t, err.Error(), "kubectl",
		"Deveria mencionar kubectl na mensagem de erro")
	assert.Contains(t, err.Error(), "não encontrado",
		"Deveria informar que não foi encontrado")
}

func TestSealCmd_KubesealNotFound(t *testing.T) {
	origLookPath := lookPath
	defer func() { lookPath = origLookPath }()

	lookPath = func(file string) (string, error) {
		if file == "kubeseal" {
			return "", fmt.Errorf("not found")
		}
		return "/usr/bin/" + file, nil
	}

	err := sealCmd.RunE(sealCmd, []string{})

	assert.Error(t, err, "Deveria retornar erro quando kubeseal não encontrado")
	assert.Contains(t, err.Error(), "kubeseal",
		"Deveria mencionar kubeseal na mensagem de erro")
	assert.Contains(t, err.Error(), "não encontrado",
		"Deveria informar que não foi encontrado")
}

func TestSealCmd_SurveyError(t *testing.T) {
	origLookPath := lookPath
	origSurveyAsk := surveyAsk
	defer func() {
		lookPath = origLookPath
		surveyAsk = origSurveyAsk
	}()

	lookPath = func(file string) (string, error) {
		return "/usr/bin/" + file, nil
	}

	surveyAsk = func(qs []*survey.Question, response interface{}, opts ...survey.AskOpt) error {
		return fmt.Errorf("prompt interrompido pelo usuário")
	}

	err := sealCmd.RunE(sealCmd, []string{})

	assert.Error(t, err, "Deveria retornar erro quando survey falha")
	assert.Contains(t, err.Error(), "prompt interrompido pelo usuário",
		"Deveria conter a mensagem de erro do survey")
}

func TestSealCmd_KubectlCreateFails(t *testing.T) {
	teardown := mockExecCommand()
	defer teardown()

	origSurveyAsk := surveyAsk
	defer func() { surveyAsk = origSurveyAsk }()

	surveyAsk = func(qs []*survey.Question, response interface{}, opts ...survey.AskOpt) error {
		v := reflect.ValueOf(response).Elem()
		v.FieldByName("Name").SetString("fail-secret")
		v.FieldByName("Namespace").SetString("default")
		v.FieldByName("Key").SetString("password")
		v.FieldByName("Value").SetString("secret123")
		return nil
	}

	err := sealCmd.RunE(sealCmd, []string{})

	assert.Error(t, err, "Deveria retornar erro ao gerar secret via kubectl")
	assert.Contains(t, err.Error(), "falha ao gerar secret",
		"Deveria informar erro ao gerar secret via kubectl")
}

func TestSealCmd_KubesealFails(t *testing.T) {
	// Configurar execCommand com mock customizado para kubeseal falhar
	originalExecCommand := execCommand
	originalLookPath := lookPath
	origSurveyAsk := surveyAsk
	defer func() {
		execCommand = originalExecCommand
		lookPath = originalLookPath
		surveyAsk = origSurveyAsk
	}()

	lookPath = func(file string) (string, error) {
		return "/usr/bin/" + file, nil
	}

	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		env := []string{"GO_WANT_HELPER_PROCESS=1"}
		if name == "kubeseal" {
			env = append(env, "GO_KUBESEAL_FAIL=1")
		}
		cmd.Env = env
		return cmd
	}

	surveyAsk = func(qs []*survey.Question, response interface{}, opts ...survey.AskOpt) error {
		v := reflect.ValueOf(response).Elem()
		v.FieldByName("Name").SetString("test-secret")
		v.FieldByName("Namespace").SetString("default")
		v.FieldByName("Key").SetString("password")
		v.FieldByName("Value").SetString("secret123")
		return nil
	}

	err := sealCmd.RunE(sealCmd, []string{})

	assert.Error(t, err, "Deveria retornar erro ao selar com kubeseal")
	assert.Contains(t, err.Error(), "falha ao selar secret",
		"Deveria informar erro ao selar com kubeseal")
}

func TestSealCmd_SurveySuccess(t *testing.T) {
	teardown := mockExecCommand()
	defer teardown()

	origSurveyAsk := surveyAsk
	origAskOne := askOne
	defer func() {
		surveyAsk = origSurveyAsk
		askOne = origAskOne
	}()

	surveyAsk = func(qs []*survey.Question, response interface{}, opts ...survey.AskOpt) error {
		v := reflect.ValueOf(response).Elem()
		v.FieldByName("Name").SetString("test-secret")
		v.FieldByName("Namespace").SetString("default")
		v.FieldByName("Key").SetString("password")
		v.FieldByName("Value").SetString("secret123")
		return nil
	}

	tmpDir := t.TempDir()
	askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		r := response.(*string)
		*r = filepath.Join(tmpDir, "sealed-secret-test.yaml")
		return nil
	}

	output := captureOutput(func() {
		_ = sealCmd.RunE(sealCmd, []string{})
	})

	assert.Contains(t, output, "Gerando Secret",
		"Deveria mostrar mensagem de geração")
	assert.Contains(t, output, "Selando com Kubeseal",
		"Deveria mostrar mensagem de selagem")
}

func TestSealCmd_SaveFileSuccess(t *testing.T) {
	teardown := mockExecCommand()
	defer teardown()

	origSurveyAsk := surveyAsk
	origAskOne := askOne
	defer func() {
		surveyAsk = origSurveyAsk
		askOne = origAskOne
	}()

	surveyAsk = func(qs []*survey.Question, response interface{}, opts ...survey.AskOpt) error {
		v := reflect.ValueOf(response).Elem()
		v.FieldByName("Name").SetString("test-secret")
		v.FieldByName("Namespace").SetString("default")
		v.FieldByName("Key").SetString("password")
		v.FieldByName("Value").SetString("secret123")
		return nil
	}

	tmpDir := t.TempDir()
	savedPath := filepath.Join(tmpDir, "subdir", "sealed-secret-test.yaml")
	askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		r := response.(*string)
		*r = savedPath
		return nil
	}

	output := captureOutput(func() {
		_ = sealCmd.RunE(sealCmd, []string{})
	})

	assert.Contains(t, output, "SealedSecret salvo em",
		"Deveria confirmar salvamento do arquivo")
	assert.Contains(t, output, savedPath,
		"Deveria mostrar o caminho do arquivo salvo")

	// Verificar que o arquivo foi realmente criado
	_, err := os.Stat(savedPath)
	assert.NoError(t, err, "O arquivo selado deveria existir em disco")

	// Verificar conteúdo do arquivo
	content, err := os.ReadFile(savedPath)
	assert.NoError(t, err, "Deveria ser possível ler o arquivo salvo")
	assert.NotEmpty(t, content, "O arquivo salvo não deveria estar vazio")
}

func TestSealCmd_SaveFileInvalidPath(t *testing.T) {
	teardown := mockExecCommand()
	defer teardown()

	origSurveyAsk := surveyAsk
	origAskOne := askOne
	defer func() {
		surveyAsk = origSurveyAsk
		askOne = origAskOne
	}()

	surveyAsk = func(qs []*survey.Question, response interface{}, opts ...survey.AskOpt) error {
		v := reflect.ValueOf(response).Elem()
		v.FieldByName("Name").SetString("test-secret")
		v.FieldByName("Namespace").SetString("default")
		v.FieldByName("Key").SetString("password")
		v.FieldByName("Value").SetString("secret123")
		return nil
	}

	// Caminho inválido: diretório que não pode ser criado
	askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		r := response.(*string)
		*r = "/dev/null/impossivel/sealed.yaml"
		return nil
	}

	err := sealCmd.RunE(sealCmd, []string{})

	assert.Error(t, err, "Deveria retornar erro ao salvar em caminho inválido")
	assert.Contains(t, err.Error(), "falha ao salvar sealed secret",
		"Deveria informar erro ao salvar em caminho inválido")
}
