package mirror

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// PodLister abstrai a listagem de pods para permitir injeção em testes
type PodLister interface {
	ListPods(ctx context.Context, namespace string, labelSelector string) (*corev1.PodList, error)
}

// TunnelDialer abstrai a criação do túnel port-forward para permitir injeção em testes
type TunnelDialer interface {
	CreateTunnel(podName, namespace string, ports []string, stopCh, readyCh chan struct{}, out, errOut io.Writer) (PortForwardSession, error)
}

// PortForwardSession abstrai uma sessão de port-forward ativa
type PortForwardSession interface {
	ForwardPorts() error
	GetPorts() ([]portforward.ForwardedPort, error)
}

// k8sPodLister implementa PodLister usando client-go real
type k8sPodLister struct {
	clientset *kubernetes.Clientset
}

func (l *k8sPodLister) ListPods(ctx context.Context, namespace string, labelSelector string) (*corev1.PodList, error) {
	return l.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
}

// k8sTunnelDialer implementa TunnelDialer usando client-go real
type k8sTunnelDialer struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
}

func (d *k8sTunnelDialer) CreateTunnel(podName, namespace string, ports []string, stopCh, readyCh chan struct{}, out, errOut io.Writer) (PortForwardSession, error) {
	req := d.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(d.config)
	if err != nil {
		return nil, err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())

	fw, err := portforward.New(dialer, ports, stopCh, readyCh, out, errOut)
	if err != nil {
		return nil, fmt.Errorf("failed to create portforwarder: %w", err)
	}

	return fw, nil
}

// PortForwarder manages a port-forwarding session to a pod
type PortForwarder struct {
	Namespace  string
	Service    string
	TargetPort int // The container port (9418)
	LocalPort  int // The local port (0 for random)

	stopCh    chan struct{}
	readyCh   chan struct{}
	clientset *kubernetes.Clientset
	config    *rest.Config
	out       io.Writer

	// Interfaces injetáveis para testes
	podLister     PodLister
	tunnelDialer  TunnelDialer
	healthChecker HealthChecker
}

// NewPortForwarder creates a new PortForwarder instance
func NewPortForwarder(namespace, service string, targetPort int) (*PortForwarder, error) {
	// Load kubeconfig
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &PortForwarder{
		Namespace:     namespace,
		Service:       service,
		TargetPort:    targetPort,
		stopCh:        make(chan struct{}),
		readyCh:       make(chan struct{}),
		clientset:     clientset,
		config:        config,
		out:           io.Discard,
		podLister:     &k8sPodLister{clientset: clientset},
		tunnelDialer:  &k8sTunnelDialer{clientset: clientset, config: config},
		healthChecker: NewTCPHealthChecker(3 * time.Second),
	}, nil
}

// Start establishes the port forwarding tunnel asynchronously.
// It returns the assigned local port once the tunnel is ready.
func (pf *PortForwarder) Start(ctx context.Context) (int, error) {
	// 1. Find the Pod
	pods, err := pf.podLister.ListPods(ctx, pf.Namespace, fmt.Sprintf("app.kubernetes.io/name=%s", pf.Service))
	if err != nil || len(pods.Items) == 0 {
		// Fallback to simpler label
		pods, err = pf.podLister.ListPods(ctx, pf.Namespace, fmt.Sprintf("app=%s", pf.Service))
		if err != nil {
			return 0, fmt.Errorf("failed to list pods for service %s: %w", pf.Service, err)
		}
		if len(pods.Items) == 0 {
			return 0, fmt.Errorf("no pods found for service %s in namespace %s", pf.Service, pf.Namespace)
		}
	}

	// Pick the first running pod
	var targetPod *corev1.Pod
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			targetPod = &pod
			break
		}
	}
	if targetPod == nil {
		return 0, fmt.Errorf("no running pods found for service %s", pf.Service)
	}

	// 2. Configure and create tunnel
	ports := []string{fmt.Sprintf("%d:%d", pf.LocalPort, pf.TargetPort)}

	fw, err := pf.tunnelDialer.CreateTunnel(targetPod.Name, pf.Namespace, ports, pf.stopCh, pf.readyCh, pf.out, os.Stderr)
	if err != nil {
		return 0, err
	}

	// 3. Run in goroutine
	go func() {
		if err := fw.ForwardPorts(); err != nil {
			// This will happen when Stop() is called, so we can ignore it or log it
		}
	}()

	// 4. Wait for Ready
	select {
	case <-pf.readyCh:
		// Get the assigned port
		forwardedPorts, err := fw.GetPorts()
		if err != nil {
			return 0, fmt.Errorf("failed to get forwarded ports: %w", err)
		}
		if len(forwardedPorts) > 0 {
			pf.LocalPort = int(forwardedPorts[0].Local)
			return pf.LocalPort, nil
		}
		return 0, fmt.Errorf("no ports forwarded")
	case <-ctx.Done():
		return 0, fmt.Errorf("timeout waiting for portforward")
	}
}

// Stop terminates the port forwarding session
func (pf *PortForwarder) Stop() {
	close(pf.stopCh)
}

// HealthCheck verifica se o port-forward está ativo
func (pf *PortForwarder) HealthCheck(ctx context.Context) error {
	if pf.healthChecker == nil {
		return fmt.Errorf("health checker não configurado")
	}
	return pf.healthChecker.Check(ctx, pf.LocalPort)
}
