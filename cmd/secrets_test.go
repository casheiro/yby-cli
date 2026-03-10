package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWebhookSecretCmd_Structure(t *testing.T) {
	assert.Equal(t, "webhook [provider] [secret]", webhookSecretCmd.Use)
	assert.NotEmpty(t, webhookSecretCmd.Short)
}

func TestMinioSecretCmd_Structure(t *testing.T) {
	assert.Equal(t, "minio", minioSecretCmd.Use)
	assert.NotEmpty(t, minioSecretCmd.Short)
}

func TestGithubTokenSecretCmd_Structure(t *testing.T) {
	assert.Equal(t, "github-token [token]", githubTokenSecretCmd.Use)
}

func TestBackupKeysCmd_Structure(t *testing.T) {
	assert.Equal(t, "backup [file]", backupKeysCmd.Use)
	assert.NotEmpty(t, backupKeysCmd.Short)
}

func TestRestoreKeysCmd_Structure(t *testing.T) {
	assert.Equal(t, "restore [file]", restoreKeysCmd.Use)
	assert.NotEmpty(t, restoreKeysCmd.Short)
}

func TestWebhookSecretCmd_RunE_Exists(t *testing.T) {
	assert.NotNil(t, webhookSecretCmd.RunE, "webhookSecretCmd deve usar RunE")
}

func TestMinioSecretCmd_RunE_Exists(t *testing.T) {
	assert.NotNil(t, minioSecretCmd.RunE, "minioSecretCmd deve usar RunE")
}

func TestGithubTokenSecretCmd_RunE_Exists(t *testing.T) {
	assert.NotNil(t, githubTokenSecretCmd.RunE, "githubTokenSecretCmd deve usar RunE")
}

func TestBackupKeysCmd_RunE_Exists(t *testing.T) {
	assert.NotNil(t, backupKeysCmd.RunE, "backupKeysCmd deve usar RunE")
}

func TestRestoreKeysCmd_RunE_Exists(t *testing.T) {
	assert.NotNil(t, restoreKeysCmd.RunE, "restoreKeysCmd deve usar RunE")
}
