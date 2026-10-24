package cmd

import (
	"context"
	"fmt"
	"testing"

	"github.com/casheiro/yby-cli/pkg/services/environment"
	"github.com/stretchr/testify/assert"
)

// mockPrompter implementa Prompter para testes
type mockPrompter struct {
	inputFunc         func(title string, defaultVal string) (string, error)
	confirmFunc       func(title string, defaultVal bool) (bool, error)
	confirmarNomeFunc func(acao, nome string) (bool, error)
}

func (m *mockPrompter) Input(title string, defaultVal string) (string, error) {
	if m.inputFunc != nil {
		return m.inputFunc(title, defaultVal)
	}
	return "", nil
}

func (m *mockPrompter) Password(title string) (string, error) { return "", nil }

func (m *mockPrompter) Confirm(title string, defaultVal bool) (bool, error) {
	if m.confirmFunc != nil {
		return m.confirmFunc(title, defaultVal)
	}
	return true, nil
}

func (m *mockPrompter) Select(title string, options []string, defaultVal string) (string, error) {
	return "", nil
}

func (m *mockPrompter) MultiSelect(title string, options []string, defaults []string) ([]string, error) {
	return nil, nil
}

func (m *mockPrompter) ConfirmarNome(acao, nome string) (bool, error) {
	if m.confirmarNomeFunc != nil {
		return m.confirmarNomeFunc(acao, nome)
	}
	return false, nil
}

// mockDestroyClusterManager substitui a factory do destroy para testes
func mockDestroyClusterManager(deleteErr error) func() {
	original := newDestroyClusterManager
	newDestroyClusterManager = func() environment.ClusterManager {
		return &mockClusterManager{
			deleteFunc: func(ctx context.Context, name string) error {
				return deleteErr
			},
		}
	}
	return func() { newDestroyClusterManager = original }
}

func TestDestroyCmd_LocalEnv_WithMock(t *testing.T) {
	teardown := mockDestroyClusterManager(nil)
	defer teardown()

	// Garantir que o ambiente é "local" ou vazio
	t.Setenv("YBY_ENV", "")
	t.Setenv("YBY_CLUSTER_NAME", "test-cluster")

	// Salvar e restaurar contextFlag
	oldCtx := contextFlag
	contextFlag = ""
	defer func() { contextFlag = oldCtx }()

	err := destroyCmd.RunE(destroyCmd, []string{})
	assert.NoError(t, err)
}

func TestDestroyCmd_NonLocalEnv_WithoutFlag_Rejected(t *testing.T) {
	t.Setenv("YBY_ENV", "prod")

	destroyCmd.Flags().Set("yes-destroy-production", "false")

	err := destroyCmd.RunE(destroyCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requer a flag --yes-destroy-production")
}

func TestDestroyCmd_NonLocalEnv_WithFlag_Cancelado(t *testing.T) {
	original := newDestroyClusterManager
	defer func() { newDestroyClusterManager = original }()
	newDestroyClusterManager = func() environment.ClusterManager {
		t.Fatal("não deveria destruir quando o usuário cancela")
		return nil
	}

	oldPrompter := prompter
	prompter = &mockPrompter{
		confirmarNomeFunc: func(acao, nome string) (bool, error) {
			return false, nil
		},
	}
	defer func() { prompter = oldPrompter }()

	t.Setenv("YBY_ENV", "prod")
	t.Setenv("YBY_CLUSTER_NAME", "prod-cluster")

	destroyCmd.Flags().Set("yes-destroy-production", "true")
	defer destroyCmd.Flags().Set("yes-destroy-production", "false")

	b := saidaDoComando(t, destroyCmd)
	err := destroyCmd.RunE(destroyCmd, []string{})
	assert.NoError(t, err)
	assert.Contains(t, b.String(), "destruição cancelada")
}

func TestDestroyCmd_NonLocalEnv_WithFlag_CorrectName(t *testing.T) {
	teardown := mockDestroyClusterManager(nil)
	defer teardown()

	oldPrompter := prompter
	var acaoPedida, nomePedido string
	prompter = &mockPrompter{
		confirmarNomeFunc: func(acao, nome string) (bool, error) {
			acaoPedida, nomePedido = acao, nome
			return true, nil
		},
	}
	defer func() { prompter = oldPrompter }()

	t.Setenv("YBY_ENV", "prod")
	t.Setenv("YBY_CLUSTER_NAME", "prod-cluster")

	destroyCmd.Flags().Set("yes-destroy-production", "true")
	defer destroyCmd.Flags().Set("yes-destroy-production", "false")

	err := destroyCmd.RunE(destroyCmd, []string{})
	assert.NoError(t, err)
	assert.Equal(t, "destruir cluster", acaoPedida)
	assert.Equal(t, "prod-cluster", nomePedido)
}

func TestDestroyCmd_NonLocalEnv_PromptError(t *testing.T) {
	oldPrompter := prompter
	prompter = &mockPrompter{
		confirmarNomeFunc: func(acao, nome string) (bool, error) {
			return false, fmt.Errorf("terminal não interativo")
		},
	}
	defer func() { prompter = oldPrompter }()

	t.Setenv("YBY_ENV", "staging")
	t.Setenv("YBY_CLUSTER_NAME", "staging-cluster")

	destroyCmd.Flags().Set("yes-destroy-production", "true")
	defer destroyCmd.Flags().Set("yes-destroy-production", "false")

	err := destroyCmd.RunE(destroyCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "falha ao ler confirmação")
}

func TestDestroyCmd_DefaultClusterName(t *testing.T) {
	var capturedName string
	original := newDestroyClusterManager
	newDestroyClusterManager = func() environment.ClusterManager {
		return &mockClusterManager{
			deleteFunc: func(ctx context.Context, name string) error {
				capturedName = name
				return nil
			},
		}
	}
	defer func() { newDestroyClusterManager = original }()

	t.Setenv("YBY_ENV", "")
	t.Setenv("YBY_CLUSTER_NAME", "")

	oldCtx := contextFlag
	contextFlag = ""
	defer func() { contextFlag = oldCtx }()

	err := destroyCmd.RunE(destroyCmd, []string{})
	assert.NoError(t, err)
	assert.Equal(t, "yby-local", capturedName, "deve usar 'yby-local' como nome padrão do cluster")
}

func TestDestroyCmd_YBYEnvStaging_WithoutFlag_Rejected(t *testing.T) {
	t.Setenv("YBY_ENV", "staging")
	destroyCmd.Flags().Set("yes-destroy-production", "false")

	err := destroyCmd.RunE(destroyCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ERR_VALIDATION")
	assert.Contains(t, err.Error(), "staging")
}

func TestDestroyCmd_YBYEnvLocal_Allowed(t *testing.T) {
	teardown := mockDestroyClusterManager(nil)
	defer teardown()

	t.Setenv("YBY_ENV", "local")
	t.Setenv("YBY_CLUSTER_NAME", "test-cluster")

	err := destroyCmd.RunE(destroyCmd, []string{})
	assert.NoError(t, err)
}

func TestDestroyCmd_DeleteError(t *testing.T) {
	teardown := mockDestroyClusterManager(fmt.Errorf("falha ao deletar cluster"))
	defer teardown()

	t.Setenv("YBY_ENV", "local")
	t.Setenv("YBY_CLUSTER_NAME", "test-cluster")

	err := destroyCmd.RunE(destroyCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Erro ao destruir cluster")
}
