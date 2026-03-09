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
