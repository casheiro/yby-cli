package cmd

import (
	stderrors "errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// O cloud refresh e o cloud connect gravam no audit log o que fizeram, como o painel.
func TestCloudRefresh_GravaNoAuditLog(t *testing.T) {
	gcpFalho := &fakeCloudProvider{nome: "gcp", errRefresh: stderrors.New("You do not currently have an active account selected.")}
	usarProvidersFalsos(t, awsAutenticado(), gcpFalho)
	executar(t, cloudRefreshCmd, "")

	eventos, err := auditoriaCloud().ReadEvents(time.Time{})
	require.NoError(t, err)
	require.Len(t, eventos, 2)
	assert.Equal(t, "refresh", eventos[0].Action)
	assert.Equal(t, "aws", eventos[0].Provider)
	assert.True(t, eventos[0].Success)
	assert.Equal(t, "gcp", eventos[1].Provider)
	assert.False(t, eventos[1].Success)
	assert.Equal(t, "You do not currently have an active account selected.", eventos[1].Error)
}

func TestCloudConnect_GravaAAutenticacaoNoAuditLog(t *testing.T) {
	gcpSemLogin := &fakeCloudProvider{nome: "gcp", errValidar: stderrors.New("gcloud auth login necessário")}
	usarProvidersFalsos(t, gcpSemLogin)
	require.NoError(t, cloudConnectCmd.Flags().Set("provider", "gcp"))
	t.Cleanup(func() { _ = cloudConnectCmd.Flags().Set("provider", "") })
	executar(t, cloudConnectCmd, "")

	eventos, err := auditoriaCloud().ReadEvents(time.Time{})
	require.NoError(t, err)
	require.Len(t, eventos, 1)
	assert.Equal(t, "authenticate", eventos[0].Action)
	assert.Equal(t, "gcp", eventos[0].Provider)
	assert.False(t, eventos[0].Success)
	assert.Equal(t, "gcloud auth login necessário", eventos[0].Error)
}
