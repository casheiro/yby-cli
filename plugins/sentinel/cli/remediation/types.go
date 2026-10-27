//go:build k8s

// Package remediation monta e aplica as correções do --fix do Sentinel. O spec de um pod
// em execução não muda nesses campos: a correção vai para o template do dono dele
// (Deployment, StatefulSet, DaemonSet ou CronJob), e o rollout recria os pods.
package remediation

// RemediationPatch é a correção de um recurso: um strategic merge patch no template dos
// pods dele, com o que muda em cada contêiner.
type RemediationPatch struct {
	ResourceKind string `json:"resource_kind"` // "Deployment", "StatefulSet", "DaemonSet" ou "CronJob"
	ResourceName string `json:"resource_name"`
	Namespace    string `json:"namespace"`
	PatchType    string `json:"patch_type"` // "strategic-merge"
	Patch        string `json:"patch"`      // JSON do patch
	Description  string `json:"description"`
	// Mudancas é o diff do template, uma linha por campo de um contêiner, em ordem:
	// "contêiner app: allowPrivilegeEscalation (vazio) → false".
	Mudancas []string `json:"mudancas"`
}

// Recurso devolve o recurso do patch como "Kind/nome".
func (p RemediationPatch) Recurso() string { return p.ResourceKind + "/" + p.ResourceName }

// Recusa é um recurso que o --fix não corrige, com o motivo em PT-BR: pod sem dono, Job
// solto, recurso do Argo CD, contêiner que não aceita a correção.
type Recusa struct {
	Recurso string `json:"recurso"`
	Motivo  string `json:"motivo"`
}

// Plano é o que o --fix faria: os patches (um por recurso, em ordem), os recursos
// recusados e quantos achados são de checks sem correção automática.
type Plano struct {
	Patches     []RemediationPatch
	Recusas     []Recusa
	SemCorrecao int
}

// Falha é um patch que o servidor recusou, no dry-run ou de verdade.
type Falha struct {
	Patch RemediationPatch
	Err   error
}

func (f Falha) Error() string { return f.Patch.Recurso() + ": " + f.Err.Error() }

// Unwrap devolve o erro do servidor.
func (f Falha) Unwrap() error { return f.Err }
