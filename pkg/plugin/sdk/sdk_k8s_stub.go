//go:build !k8s

package sdk

import "fmt"

func GetKubeClient() (interface{}, error) {
	return nil, fmt.Errorf("kubernetes client not available: build without 'k8s' tag")
}

// GetRestConfig não está disponível sem a build tag 'k8s'.
func GetRestConfig() (interface{}, error) {
	return nil, fmt.Errorf("kubernetes client not available: build without 'k8s' tag")
}
