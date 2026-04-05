package monitor

// Pod representa um pod Kubernetes
type Pod struct {
	Name      string
	Namespace string
	Status    string
	CPU       string
}

// Deployment representa um deployment Kubernetes
type Deployment struct {
	Name      string
	Namespace string
	Replicas  int32
	Ready     int32
	Available int32
}

// Service representa um serviço Kubernetes
type Service struct {
	Name      string
	Namespace string
	Type      string
	ClusterIP string
	Ports     string
}

// Node representa um nó do cluster Kubernetes
type Node struct {
	Name           string
	Status         string
	CPUCapacity    string
	MemoryCapacity string
	Version        string
}
