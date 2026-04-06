package monitor

// Pod representa um pod Kubernetes
type Pod struct {
	Name      string
	Namespace string
	Status    string
	CPU       string
	Memory    string
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

// StatefulSet representa um statefulset Kubernetes
type StatefulSet struct {
	Name      string
	Namespace string
	Replicas  int32
	Ready     int32
}

// Job representa um job Kubernetes
type Job struct {
	Name        string
	Namespace   string
	Completions int32
	Active      int32
	Succeeded   int32
	Failed      int32
}

// Ingress representa um ingress Kubernetes
type Ingress struct {
	Name      string
	Namespace string
	Class     string
	Hosts     string
	Paths     string
}

// ConfigMap representa um configmap Kubernetes
type ConfigMap struct {
	Name      string
	Namespace string
	Keys      int
	DataSize  string
}

// Event representa um evento Kubernetes
type Event struct {
	Name      string
	Namespace string
	Type      string
	Reason    string
	Message   string
	Age       string
}
