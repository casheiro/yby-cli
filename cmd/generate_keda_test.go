package cmd

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestGenerateKedaCmd(t *testing.T) {
	// Backup original stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Set flags purely
	kedaOpts.Name = "test-scaler"
	kedaOpts.Deployment = "test-deploy"
	kedaOpts.Namespace = "test-ns"
	kedaOpts.Schedule = "0 18 * * *"
	kedaOpts.Replicas = "5"
	kedaOpts.Timezone = "UTC"

	// Run command directly (mocking cobra execution context is hard, just testing the logic block if possible or run Run())
	// Since Run uses survey which requires TTY, we might fail if we don't provide all flags.
	// We provided all flags so it should skip prompts.

	kedaCmd.Run(kedaCmd, []string{})

	// Restore
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	expected := `apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: test-scaler
  namespace: test-ns
spec:
  scaleTargetRef:
    name: test-deploy
  minReplicaCount: 0
  maxReplicaCount: 5
  triggers:
  - type: cron
    metadata:
      timezone: UTC
      start: 0 18 * * *
      end: 0 8 * * *
      desiredReplicas: "0"
`

	if output != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, output)
	}
}
