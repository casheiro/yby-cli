package cmd

import (
	"context"
	"os"
	"testing"
)

func TestUpCmd_ClusterStartFailure(t *testing.T) {
	// Setup mocks
	teardown := mockExecCommand()
	defer teardown()

	// No osExit mock needed anymore, runLocalUp returns an error directly.

	// We utilize the fact that runLocalUp calls:
	// 1. lookPath("k3d") -> success (mocked)
	// 2. execCommand("k3d", "cluster", "list", ...) -> success?
	//    Wait, our TestHelperProcess implementation for "k3d" "cluster" "start" checks "fail-cluster".
	//    For "list", it falls through to default success (exit 0).
	//    So checkCmd.Run() succeeds.
	// 3. Then it prints "✅ Cluster já existe. Garantindo start..."
	// 4. Then execCommand("k3d", "cluster", "start", clusterName).Run()
	//    If clusterName is "fail-cluster", HelperProcess exits 1.
	//    Then runLocalUp returns an error (no osExit(1) anymore).

	// We need to set env var YBY_CLUSTER_NAME to "fail-cluster"
	os.Setenv("YBY_CLUSTER_NAME", "fail-cluster")
	defer os.Unsetenv("YBY_CLUSTER_NAME")

	// Call the function under test
	// Since runLocalUp is not exported, we can't call it directly if we are in "cmd_test" package.
	// But the file declares "package cmd". So we CAN call it.
	// It requires context and root string.
	// But it prints to stdout/stderr.
	err := runLocalUp(context.Background(), ".")

	if err == nil {
		t.Error("Expected error, got nil")
	}
}
