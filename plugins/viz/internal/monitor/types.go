package monitor

import "time"

// Pod representa um pod Kubernetes
type Pod struct {
	Name      string
	Namespace string
	// Status é o estado como o kubectl mostra: o motivo do contêiner quando há
	// um (CrashLoopBackOff, Completed, OOMKilled), senão a fase do pod.
	Status     string
	CPU        string
	Memory     string
	Prontos    int32 // contêineres prontos
	Containers int32 // contêineres no pod
	Reinicios  int32 // soma dos reinícios dos contêineres
	// UltimoReinicio é quando o contêiner reiniciado mais recente terminou
	// pela última vez (zero se nunca reiniciou).
	UltimoReinicio time.Time
	No             string // nó onde o pod roda
	IP             string
	CriadoEm       time.Time
	Labels         map[string]string // labels do objeto (o filtro L é aplicado na TUI)
}

// Deployment representa um deployment Kubernetes
type Deployment struct {
	Name        string
	Namespace   string
	Replicas    int32
	Ready       int32
	Available   int32
	Atualizados int32 // réplicas já na versão atual do template
	CriadoEm    time.Time
	Labels      map[string]string // labels do objeto (o filtro L é aplicado na TUI)
}

// Service representa um serviço Kubernetes
type Service struct {
	Name      string
	Namespace string
	Type      string
	ClusterIP string
	Ports     string
	CriadoEm  time.Time
	Labels    map[string]string // labels do objeto (o filtro L é aplicado na TUI)
}

// Node representa um nó do cluster Kubernetes
type Node struct {
	Name           string
	Status         string
	CPUCapacity    string
	MemoryCapacity string
	Version        string
	CriadoEm       time.Time
	Labels         map[string]string // labels do objeto (o filtro L é aplicado na TUI)
}

// StatefulSet representa um statefulset Kubernetes
type StatefulSet struct {
	Name        string
	Namespace   string
	Replicas    int32
	Ready       int32
	Atualizados int32 // réplicas já na revisão atual
	CriadoEm    time.Time
	Labels      map[string]string // labels do objeto (o filtro L é aplicado na TUI)
}

// Job representa um job Kubernetes
type Job struct {
	Name        string
	Namespace   string
	Completions int32
	Active      int32
	Succeeded   int32
	Failed      int32
	// Concluido e Falhou são as condições Complete e Failed do Job: o Job de
	// fila de trabalho (sem spec.completions) só termina por elas.
	Concluido bool
	Falhou    bool
	CriadoEm  time.Time
	Labels    map[string]string // labels do objeto (o filtro L é aplicado na TUI)
}

// Ingress representa um ingress Kubernetes
type Ingress struct {
	Name      string
	Namespace string
	Class     string
	Hosts     string
	Paths     string
	CriadoEm  time.Time
	Labels    map[string]string // labels do objeto (o filtro L é aplicado na TUI)
}

// ConfigMap representa um configmap Kubernetes
type ConfigMap struct {
	Name      string
	Namespace string
	Keys      int
	DataSize  string
	CriadoEm  time.Time
	Labels    map[string]string // labels do objeto (o filtro L é aplicado na TUI)
}

// Event representa um evento Kubernetes
type Event struct {
	// ID é o metadata.name do objeto Event, único no namespace; Name é o
	// objeto envolvido, que se repete em vários eventos.
	ID        string
	Name      string
	Namespace string
	Type      string
	Reason    string
	Message   string
	Age       string
	Labels    map[string]string // labels do objeto (o filtro L é aplicado na TUI)
}
