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
	// Motivo explica o estado: por que o último contêiner terminou ("Error ·
	// código de saída 1"), por que espera ou por que não foi agendado.
	Motivo   string
	Imagens  []string // imagem de cada contêiner
	Dono     string   // quem controla o pod ("Deployment worker", "Job migrate")
	Recursos Recursos // soma dos pedidos e limites dos contêineres
	// Conteiner é o contêiner cujo log explica o pod: o que está com problema,
	// senão o padrão (a API recusa o log sem contêiner em pod com mais de um).
	Conteiner string
}

// Recursos são os pedidos (requests) e limites de CPU e memória, somados nos
// contêineres; vazio quando nenhum contêiner declara.
type Recursos struct {
	CPUPedido, CPULimite string
	MemPedido, MemLimite string
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
	Estrategia  string            // "RollingUpdate · surge 25% · indisponível 25%" ou "Recreate"
	Imagens     []string          // imagem de cada contêiner do template
	Seletor     string            // seletor dos pods ("app=worker")
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
	Externos  string            // IPs externos e do balanceador
	Seletor   string            // seletor dos pods ("app=api")
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
	Papeis         string            // papéis do node-role.kubernetes.io ("control-plane")
	IPInterno      string
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
	Servico     string            // service headless que dá nome aos pods
	Imagens     []string          // imagem de cada contêiner do template
	Seletor     string            // seletor dos pods ("app=redis")
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
	Endereco  string            // IP ou nome do balanceador
}

// ConfigMap representa um configmap Kubernetes
type ConfigMap struct {
	Name      string
	Namespace string
	Keys      int
	DataSize  string
	CriadoEm  time.Time
	Labels    map[string]string // labels do objeto (o filtro L é aplicado na TUI)
	Chaves    []string          // nomes das chaves, em ordem
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
	Kind      string            // tipo do objeto envolvido ("Pod")
	Quantas   int32             // quantas vezes o evento se repetiu
	Quando    time.Time         // a última ocorrência (zero se a API não diz)
}
