//go:build k8s

package checks

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
)

// Variáveis de ambiente que dizem ao cosign como confiar na assinatura: uma chave pública
// ou uma identidade de certificado (keyless) com o emissor OIDC dela.
const (
	envCosignChave      = "YBY_SENTINEL_COSIGN_KEY"
	envCosignIdentidade = "YBY_SENTINEL_COSIGN_IDENTITY"
	envCosignEmissor    = "YBY_SENTINEL_COSIGN_ISSUER"
)

// prazoCosign é o tempo máximo de cada chamada ao cosign.
const prazoCosign = 30 * time.Second

// ImageSignatureCheck verifica se imagens possuem assinatura cosign. Sem chave nem
// identidade configuradas não há o que verificar: o `cosign verify` sem elas falha em toda
// imagem, e isso não é uma imagem sem assinatura. Nesse caso o check devolve ErrNaoVerificado
// (sem achado), e o mesmo vale para o cosign ausente.
type ImageSignatureCheck struct {
	// ExecCommand permite injetar mock para testes.
	ExecCommand func(name string, args ...string) *exec.Cmd
	// Chave é a chave pública (ou KMS) das assinaturas; vazia, vale YBY_SENTINEL_COSIGN_KEY.
	Chave string
	// Identidade e Emissor são a identidade do certificado keyless e o emissor OIDC dela;
	// vazios, valem YBY_SENTINEL_COSIGN_IDENTITY e YBY_SENTINEL_COSIGN_ISSUER.
	Identidade string
	Emissor    string
}

// argsDeConfianca devolve as flags do `cosign verify` para a configuração do check (ou do
// ambiente); vazio quando não há chave nem identidade com emissor.
func (c *ImageSignatureCheck) argsDeConfianca() []string {
	pegar := func(valor, env string) string {
		if valor != "" {
			return valor
		}
		return os.Getenv(env)
	}
	if chave := pegar(c.Chave, envCosignChave); chave != "" {
		return []string{"--key", chave}
	}
	identidade, emissor := pegar(c.Identidade, envCosignIdentidade), pegar(c.Emissor, envCosignEmissor)
	if identidade != "" && emissor != "" {
		return []string{"--certificate-identity", identidade, "--certificate-oidc-issuer", emissor}
	}
	return nil
}

// rodarComPrazo roda o comando e devolve a saída; passado o prazo (ou cancelado o ctx), mata o processo.
func rodarComPrazo(ctx context.Context, cmd *exec.Cmd, prazo time.Duration) ([]byte, error) {
	var saida bytes.Buffer
	cmd.Stdout, cmd.Stderr = &saida, &saida
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	fim := make(chan error, 1)
	go func() { fim <- cmd.Wait() }()
	select {
	case err := <-fim:
		return saida.Bytes(), err
	case <-time.After(prazo):
		_ = cmd.Process.Kill()
		<-fim
		return saida.Bytes(), fmt.Errorf("o cosign passou de %s", prazo)
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		<-fim
		return saida.Bytes(), ctx.Err()
	}
}

func init() { Register(&ImageSignatureCheck{ExecCommand: exec.Command}) }

func (c *ImageSignatureCheck) ID() string         { return "IMAGE_SIGNATURE" }
func (c *ImageSignatureCheck) Name() string       { return "Assinatura de Imagem" }
func (c *ImageSignatureCheck) Category() Category { return CategorySupplyChain }
func (c *ImageSignatureCheck) Severity() Severity { return SeverityHigh }
func (c *ImageSignatureCheck) Description() string {
	return "Verifica se imagens de containers possuem assinatura cosign"
}

func (c *ImageSignatureCheck) Run(ctx context.Context, client kubernetes.Interface, namespace string) ([]SecurityFinding, error) {
	pods, err := listarPods(ctx, client, namespace)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar pods: %w", err)
	}

	execCmd := c.ExecCommand
	if execCmd == nil {
		execCmd = exec.Command
	}

	confianca := c.argsDeConfianca()
	if len(confianca) == 0 {
		return nil, &ErrNaoVerificado{Motivo: "assinatura de imagens não verificada: configure " + envCosignChave + " ou " + envCosignIdentidade + " e " + envCosignEmissor}
	}

	// Verificar se cosign está disponível
	if _, err := rodarComPrazo(ctx, execCmd("cosign", "version"), prazoCosign); err != nil {
		return nil, &ErrNaoVerificado{Motivo: "cosign não encontrado: instale em https://docs.sigstore.dev/cosign/system_config/installation/"}
	}

	// Coletar imagens únicas
	imageSet := make(map[string][]string) // imagem -> []pod/container
	for _, pod := range pods.Items {
		allContainers := append(pod.Spec.Containers, pod.Spec.InitContainers...)
		for _, container := range allContainers {
			key := container.Image
			imageSet[key] = append(imageSet[key], fmt.Sprintf("%s/%s", pod.Name, container.Name))
		}
	}

	var findings []SecurityFinding
	for image, refs := range imageSet {
		args := append([]string{"verify", "--output", "text"}, confianca...)
		cmd := execCmd("cosign", append(args, image)...)
		if _, err := rodarComPrazo(ctx, cmd, prazoCosign); err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			for _, ref := range refs {
				parts := strings.SplitN(ref, "/", 2)
				podName := parts[0]
				containerName := ""
				if len(parts) > 1 {
					containerName = parts[1]
				}
				findings = append(findings, SecurityFinding{
					CheckID:        c.ID(),
					Severity:       SeverityHigh,
					Category:       CategorySupplyChain,
					Pod:            podName,
					Container:      containerName,
					Namespace:      namespace,
					Resource:       ref,
					Message:        fmt.Sprintf("Imagem '%s' não possui assinatura cosign verificada", image),
					Recommendation: "Assine imagens com cosign antes do deploy: cosign sign <imagem>",
					Type:           "critical",
					Description:    fmt.Sprintf("Imagem '%s' não possui assinatura cosign verificada", image),
				})
			}
		}
	}

	return findings, nil
}
