package portforwarder

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

type KubernetesInfo struct {
	Config    string `json:"config,omitempty"    yaml:"config,omitempty"`
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"  yaml:"endpoint,omitempty"`
	Context   string `json:"context,omitempty"   yaml:"context,omitempty"`
	Port      string `json:"port,omitempty"      yaml:"port,omitempty"`
}

func (k KubernetesInfo) IsZeroValue() bool {
	return k.Config == "" || k.Namespace == "" || k.Context == "" || k.Endpoint == ""
}

type PortForwarder struct {
	portAllocationMx sync.Mutex
	stopCh           chan struct{}
	Port             int
}

func (p *PortForwarder) findAvailableLocalPort() (int, error) {
	p.portAllocationMx.Lock()
	defer p.portAllocationMx.Unlock()

	listener, err := net.Listen("tcp", ":0") //nolint:gosec
	if err != nil {
		return 0, fmt.Errorf("error dialing: %w", err)
	}

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("address it not type net.TCPAddr: %#v", listener.Addr())
	}

	_ = listener.Close()

	return tcpAddr.Port, nil
}

type portForwardAPodRequest struct {
	// RestConfig is the kubernetes userconfig
	RestConfig *rest.Config
	// Pod is the selected pod for this port forwarding
	Pod corev1.Pod
	// LocalPort is the local port that will be selected to expose the PodPort
	LocalPort int
	// PodPort is the target port for the pod
	PodPort int
	// Steams configures where to write or read input from
	Streams genericclioptions.IOStreams
	// StopCh is the channel used to manage the port forward lifecycle
	StopCh <-chan struct{}
	// ReadyCh communicates when the tunnel is ready to receive traffic
	ReadyCh chan struct{}
}

// resolveClientConfig will retrieve a restconfig, but considering files with
// multiple contexts also.
func resolveClientConfig(configFile string, context string) (*rest.Config, error) {
	//nolint:exhaustivestruct
	configLoadingRules := &clientcmd.ClientConfigLoadingRules{ //nolint:exhaustruct
		ExplicitPath: configFile,
	}
	//nolint:exhaustivestruct
	configOverrides := &clientcmd.ConfigOverrides{CurrentContext: context} //nolint:exhaustruct

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		configLoadingRules, configOverrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("error loading kubeconfig: %w", err)
	}

	slog.Info(fmt.Sprintf("Impersonating: %s", config.Impersonate.UserName))

	return config, nil
}

func (p *PortForwarder) Close() error {
	close(p.stopCh)

	return nil
}

const PFWaitTimeout = time.Second * 5

//nolint:funlen
func (p *PortForwarder) Start(ctx context.Context, kinfo KubernetesInfo) error {
	stopCh := make(chan struct{}, 1)
	p.stopCh = stopCh

	// use the current context in kubeconfig
	config, err := resolveClientConfig(kinfo.Config, kinfo.Context)
	if err != nil {
		return fmt.Errorf("error resolving client userconfig: %w", err)
	}

	lport, err := p.findAvailableLocalPort()
	if err != nil {
		return fmt.Errorf("error finding available local port: %w", err)
	}

	rport, err := strconv.Atoi(kinfo.Port)
	if err != nil {
		return fmt.Errorf("error converting port to int: %w", err)
	}

	pfreq := &portForwardAPodRequest{
		RestConfig: config,
		Pod: corev1.Pod{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      kinfo.Endpoint,
				Namespace: kinfo.Namespace,
			},
		},
		LocalPort: lport,
		PodPort:   rport,
		Streams:   genericclioptions.IOStreams{},
		StopCh:    nil,
		ReadyCh:   nil,
	}

	pfreq.RestConfig = config

	// stopCh control the port forwarding lifecycle. When it gets closed the
	// port forward will terminate

	// readyCh communicate when the port forward is ready to get traffic
	readyCh := make(chan struct{})
	errCh := make(chan error)
	// stream is used to tell the port forwarder where to place its output or
	// where to expect input if needed. For the port forwarding we just need
	// the output eventually
	stream := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	pfreq.Streams = stream
	pfreq.ReadyCh = readyCh
	pfreq.StopCh = stopCh
	err = checkPodOrService(ctx, pfreq)

	if err != nil {
		return fmt.Errorf("error checking pod or service: %w", err)
	}

	go func() {
		err := portForwardAPod(ctx, pfreq)
		if err != nil {
			slog.Warn("error while port forwarding", "err", err)
			errCh <- err
		}
	}()

	select {
	case err = <-errCh:
		if err != nil {
			return fmt.Errorf("error port forwarding: %w", err)
		}
	case <-time.After(PFWaitTimeout):
	case <-readyCh:
	}
	slog.Info("Port forwarding is ready to get traffic. have fun!")

	p.Port = lport

	return nil
}

func checkPodOrService(ctx context.Context, req *portForwardAPodRequest) error {
	if ctx.Err() != nil {
		return fmt.Errorf("error checking pod: %w", ctx.Err())
	}

	clientset, err := kubernetes.NewForConfig(req.RestConfig)
	if err != nil {
		return fmt.Errorf("error creating kubernetes client: %w", err)
	}

	endpointsList, err := clientset.CoreV1().Endpoints(req.Pod.Namespace).List(
		ctx, metav1.ListOptions{ //nolint:exhaustruct,exhaustivestruct
			FieldSelector: "metadata.name=" + req.Pod.Name,
		})
	if err != nil {
		return fmt.Errorf("unable to find service %s: %w", req.Pod.Name, err)
	}

	if len(endpointsList.Items) > 0 && len(endpointsList.Items[0].Subsets) > 0 && len(endpointsList.Items[0].Subsets[0].Addresses) > 0 {
		ipadddr := endpointsList.Items[0].Subsets[0].Addresses[0].IP

		pods, err := clientset.CoreV1().Pods(req.Pod.Namespace).List(
			ctx, metav1.ListOptions{ //nolint:exhaustruct,exhaustivestruct
				FieldSelector: "status.podIP=" + ipadddr,
			})
		if err != nil {
			return fmt.Errorf( //nolint:goerr113
				"unable to find pod for service %s: %s",
				req.Pod.Name, err.Error())
		}

		req.Pod.Name = pods.Items[0].Name
		return nil
	}

	return fmt.Errorf("could not resolve ip for endpoint %s", req.Pod.Name)
}

// portForwardAPod wil effectively do the port forward but to a pod.
// If the PF is expected to be closed w a service, please consider
// PFStart, it will resolve a pod from service and "pretend" PF
// is being oriented to a service.
func portForwardAPod(ctx context.Context, req *portForwardAPodRequest) error {
	if ctx.Err() != nil {
		return fmt.Errorf("context closed when port forwarding: %w", ctx.Err())
	}

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward",
		req.Pod.Namespace, req.Pod.Name)
	hostIP := strings.TrimLeft(req.RestConfig.Host, "htps:/")

	transport, upgrader, err := spdy.RoundTripperFor(req.RestConfig)
	if err != nil {
		return fmt.Errorf("error creating roundtripper: %w", err)
	}

	dialer := spdy.NewDialer(
		upgrader,
		&http.Client{Transport: transport}, //nolint:exhaustruct,exhaustivestruct
		http.MethodPost,
		&url.URL{Scheme: "https", Path: path, Host: hostIP}) //nolint:exhaustruct,exhaustivestruct

	portForwader, err := portforward.New(dialer, []string{
		fmt.Sprintf("%d:%d", req.LocalPort, req.PodPort),
	},
		req.StopCh,
		req.ReadyCh,
		req.Streams.Out,
		req.Streams.ErrOut)
	if err != nil {
		return fmt.Errorf("error creating portforward: %w", err)
	}

	err = portForwader.ForwardPorts()
	if err != nil {
		return fmt.Errorf("error forwarding ports: %w", err)
	}

	go func() {
		<-ctx.Done()
		portForwader.Close()
	}()

	return nil
}

func New() *PortForwarder {
	return &PortForwarder{}
}
