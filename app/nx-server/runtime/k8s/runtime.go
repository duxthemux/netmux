package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/duxthemux/netmux/business/netmux"
)

type Opts struct {
	Kubefile   string
	Namespaces []string
	All        bool
}

func MyNamespace() (string, error) {
	bs, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return "", fmt.Errorf("error reading namespace: %w", err)
	}

	return string(bs), nil
}

type Runtime struct {
	opts     Opts
	cancel   func(err error)
	chEvents chan netmux.Event
}

func (k *Runtime) Events() <-chan netmux.Event {
	return k.chEvents
}

func NewRuntime(opts Opts) *Runtime {
	ret := &Runtime{
		opts:     opts,
		chEvents: make(chan netmux.Event),
	}

	return ret
}

func (k *Runtime) loadFromAnnotation(s string) ([]netmux.Bridge, error) {
	ret := make([]netmux.Bridge, 0)

	err := yaml.Unmarshal([]byte(s), &ret)
	if err != nil {
		return nil, fmt.Errorf("error parsing annotation: %w", err)
	}

	return ret, nil
}

func (k *Runtime) resolveConfig(fname string) (*rest.Config, error) {
	if fname != "" {
		ret, err := clientcmd.BuildConfigFromFlags("", fname)
		if err != nil {
			return nil, fmt.Errorf("errmr building config from file %s: %w", fname, err)
		}

		return ret, nil
	}

	slog.Info("Using InClusterConfig")

	ret, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("error building incluster config: %w", err)
	}

	return ret, nil
}

//nolint:funlen,cyclop
func (k *Runtime) handleServiceWithAnnotations(evt watch.EventType, dep *corev1.Service) {
	bridges, err := k.loadFromAnnotation(dep.Annotations["nx"])
	if err != nil {
		slog.Warn(fmt.Sprintf("error reading annotation for %s.%s: %s", dep.Name, dep.Namespace, err.Error()))

		return
	}

	for i := range bridges {
		nxa := bridges[i]
		if nxa.Name == "" {
			nxa.Name = dep.Name
			slog.Debug(fmt.Sprintf("Using name from service: %s.%s", dep.Namespace, dep.Name))
		}

		if nxa.ContainerAddr == "" {
			nxa.ContainerAddr = dep.Spec.ClusterIP
			slog.Debug(fmt.Sprintf("Fixing bridge w/o remote addr: %s.%s => %s", dep.Namespace, dep.Name, nxa.ContainerAddr))
		}

		if nxa.LocalAddr == "" {
			nxa.LocalAddr = dep.Name
			slog.Debug(fmt.Sprintf("Fixing bridge w/o local addr: %s.%s => %s", dep.Namespace, dep.Name, nxa.LocalAddr))
		}

		if nxa.ContainerPort == "" {
			nxa.ContainerPort = fmt.Sprintf("%v", dep.Spec.Ports[0].Port)
			slog.Debug(fmt.Sprintf("Fixing bridge w/o remote port: %s.%s => %s", dep.Namespace, dep.Name, nxa.ContainerPort))
		}

		if nxa.LocalPort == "" {
			nxa.LocalPort = fmt.Sprintf("%v", dep.Spec.Ports[0].Port)
			slog.Debug(fmt.Sprintf("Fixing bridge w/o local port: %s.%s => %s", dep.Namespace, dep.Name, nxa.LocalPort))
		}

		if nxa.Direction == "" {
			nxa.Direction = "L2C"

			slog.Debug(fmt.Sprintf("Fixing bridge w/o direction: %s.%s => %s", dep.Namespace, dep.Name, "L2C"))
		}

		if nxa.Family == "" {
			nxa.Family = "tcp"

			slog.Debug(fmt.Sprintf("Fixing bridge w/o proto: %s.%s => %s", dep.Namespace, dep.Name, "tcp"))
		}

		nxa.Namespace = dep.Namespace

		slog.Info(fmt.Sprintf("K8S Event %v for %s.%s", evt, dep.Name, dep.Namespace))

		switch evt {
		case watch.Added:
			slog.Info(fmt.Sprintf("Added service: %s", nxa.Name))
			k.chEvents <- netmux.Event{
				EvtName: netmux.EventBridgeAdd,
				Bridge:  nxa,
			}

		case watch.Deleted:
			slog.Info(fmt.Sprintf("Deleted service: %s", nxa.Name))
			k.chEvents <- netmux.Event{
				EvtName: netmux.EventBridgeDel,
				Bridge:  nxa,
			}

		case watch.Modified:
			slog.Info(fmt.Sprintf("Modified service: %s", nxa.Name))
			k.chEvents <- netmux.Event{
				EvtName: netmux.EventBridgeUp,
				Bridge:  nxa,
			}
		case watch.Bookmark:
		case watch.Error:
			slog.Warn(fmt.Sprintf("error during event collection: %v", evt))

		default:
			slog.Warn(fmt.Sprintf("unknown state while processing k8s events: %v", evt))
		}
	}
}

func (k *Runtime) handleServiceWithoutAnnotations(evt watch.EventType, dep *corev1.Service) {
	nxa := netmux.Bridge{}
	nxa.Name = dep.Name
	nxa.ContainerAddr = dep.Spec.ClusterIP
	nxa.ContainerPort = fmt.Sprintf("%v", dep.Spec.Ports[0].Port)
	nxa.LocalAddr = dep.Name
	nxa.LocalPort = fmt.Sprintf("%v", dep.Spec.Ports[0].Port)
	nxa.Direction = "L2C"
	nxa.Namespace = dep.Namespace
	nxa.Family = "tcp"
	nxa.Name = dep.Name

	slog.Info(fmt.Sprintf("K8S Event %v for %s.%s", evt, dep.Name, dep.Namespace))

	switch evt {
	case watch.Added:
		slog.Info(fmt.Sprintf("Added service: %s", nxa.Name))
		k.chEvents <- netmux.Event{
			EvtName: netmux.EventBridgeAdd,
			Bridge:  nxa,
		}

	case watch.Deleted:
		slog.Info(fmt.Sprintf("Deleted service: %s", nxa.Name))
		k.chEvents <- netmux.Event{
			EvtName: netmux.EventBridgeDel,
			Bridge:  nxa,
		}
	case watch.Modified:
		slog.Info(fmt.Sprintf("Modified service: %s", nxa.Name))
		k.chEvents <- netmux.Event{
			EvtName: netmux.EventBridgeUp,
			Bridge:  nxa,
		}
	case watch.Bookmark:
	case watch.Error:
		slog.Warn(fmt.Sprintf("error during event collection: %v", evt))

	default:
		slog.Warn(fmt.Sprintf("unknown state while processing k8s events: %v", evt))
	}
}

func (k *Runtime) handleService(evt watch.EventType, dep *corev1.Service) {
	if dep.Annotations["nx"] != "" {
		k.handleServiceWithAnnotations(evt, dep)

		return
	}

	k.handleServiceWithoutAnnotations(evt, dep)
}

func (k *Runtime) runOnNS(ctx context.Context, cli *kubernetes.Clientset, ns string) error {
	slog.Info("K8s monitoring", "ns", ns)

	wservices, err := cli.CoreV1().Services(ns).Watch(ctx, v1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error watching services: %w", err)
	}

	go func() {
		for {
			select {
			case x := <-wservices.ResultChan():
				p, ok := x.Object.(*corev1.Service)
				if ok && p != nil {
					k.handleService(x.Type, p)
				}

			case <-ctx.Done():
				return
			}
		}
	}()
	slog.Debug("Namespace monitoring on")

	return nil
}

func (k *Runtime) Close() error {
	k.cancel(fmt.Errorf("k8s Runtime ended"))

	return nil
}

func (k *Runtime) Run(ctx context.Context) error {
	opts := k.opts
	ctx, cancel := context.WithCancelCause(ctx)
	k.cancel = cancel

	kubeConfig, err := k.resolveConfig(opts.Kubefile)
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("error creating k8s client: %w", err)
	}

	ns, err := MyNamespace()
	if err != nil {
		return fmt.Errorf("error getting my namespace: %w", err)
	}

	err = k.runOnNS(ctx, clientset, ns)
	if err != nil {
		return err
	}

	<-ctx.Done()

	return nil
}
