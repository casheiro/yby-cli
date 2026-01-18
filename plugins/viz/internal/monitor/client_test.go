package monitor

import "testing"

func TestMockClient_GetPods(t *testing.T) {
	client := NewMockClient()

	pods, err := client.GetPods()
	if err != nil {
		t.Fatalf("GetPods failed: %v", err)
	}

	if len(pods) == 0 {
		t.Error("Expected mocked pods, got 0")
	}

	found := false
	for _, p := range pods {
		if p.Name == "api-gateway-v1" {
			found = true
			if p.Status != "Running" {
				t.Errorf("Expected api-gateway-v1 to be Running, got %s", p.Status)
			}
		}
	}

	if !found {
		t.Error("Expected to find api-gateway-v1 in mocked data")
	}
}
