package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"

	"go.digitalcircle.com.br/dc/netmux/app/nx-daemon/config"
	"go.digitalcircle.com.br/dc/netmux/business/netmux"
	"go.digitalcircle.com.br/dc/netmux/business/networkallocator"
	"go.digitalcircle.com.br/dc/netmux/business/networkallocator/dnsallocator"
	"go.digitalcircle.com.br/dc/netmux/business/portforwarder"
	"go.digitalcircle.com.br/dc/netmux/foundation/memstore"
	"go.digitalcircle.com.br/dc/netmux/foundation/metrics"
)

var (
	ErrEndpointAlreadyConnected = fmt.Errorf("endpoint already connect")
	ErrEndpointNotConnected     = fmt.Errorf("endpoint not connect")
	ErrBridgeNotFound           = fmt.Errorf("bridge not found")
	ErrBridgeAlreadyConnected   = fmt.Errorf("bridge already connected")
	ErrBridgeDirectionInvalid   = fmt.Errorf("bridge direction invalid")
)

type OperationalBridge struct {
	netmux.Bridge
	cancel func(err error)
}

type OperationalEndPoint struct {
	agent              *netmux.Agent
	config             config.Endpoint
	cancel             func(err error)
	availableBridges   *memstore.Map[netmux.Bridge]
	operationalBridges *memstore.Map[*OperationalBridge]
}

func NewOperationalEndPoint() *OperationalEndPoint {
	return &OperationalEndPoint{
		agent:              &netmux.Agent{},
		config:             config.Endpoint{},
		cancel:             func(err error) {},
		availableBridges:   memstore.New[netmux.Bridge](),
		operationalBridges: memstore.New[*OperationalBridge](),
	}
}

type StatusBridges struct {
	netmux.Bridge
	Status string `json:"status"`
}

type StatusEndPoints struct {
	config.Endpoint
	Status  string          `json:"status"`
	Bridges []StatusBridges `json:"bridges"`
}

type Status struct {
	Endpoints []StatusEndPoints `json:"endpoints"`
}

type Daemon struct {
	cfg                  *config.Config
	networkAllocator     *networkallocator.NetworkAllocator
	operationalEndpoints *memstore.Map[*OperationalEndPoint]
	metricsFactroy       metrics.Factory
}

type Opts func(d *Daemon)

func WithMetrics(m metrics.Factory) Opts {
	return func(d *Daemon) {
		d.metricsFactroy = m
	}
}

func New(cfg *config.Config, nw *networkallocator.NetworkAllocator, opts ...Opts) *Daemon {
	ret := &Daemon{
		cfg:                  cfg,
		networkAllocator:     nw,
		operationalEndpoints: memstore.New[*OperationalEndPoint](),
	}
	for _, opt := range opts {
		opt(ret)
	}

	return ret
}

func (d *Daemon) GetConfig() *config.Config {
	return d.cfg
}

//nolint:funlen,cyclop
func (d *Daemon) Connect(ctx context.Context, endpointName string) error {
	if ctx.Err() != nil {
		return fmt.Errorf("context cancelled when connecting: %w", ctx.Err())
	}

	endpoint := d.operationalEndpoints.Get(endpointName)
	if endpoint != nil {
		return ErrEndpointAlreadyConnected
	}

	ctx, cancel := context.WithCancelCause(ctx)

	epCfg, found := d.cfg.Endpoints.FindByName(endpointName)
	if !found {
		return fmt.Errorf("config not found for endpoint %s", endpointName)
	}

	var localAgent *netmux.Agent

	var err error

	if epCfg.Kubernetes != (portforwarder.KubernetesInfo{}) {
		portForwarder := portforwarder.New()
		if err := portForwarder.Start(ctx, epCfg.Kubernetes); err != nil {
			cancel(fmt.Errorf("error starting portforwad: %w", err))

			return fmt.Errorf("error connecting port forward: %w", err)
		}

		agentEndPointPortForward := net.JoinHostPort("localhost", strconv.Itoa(portForwarder.Port))

		localAgent, err = netmux.NewAgent(ctx,
			agentEndPointPortForward, d.networkAllocator, netmux.AgentWithMetrics(d.metricsFactroy))
		if err != nil {
			cancel(fmt.Errorf("error creating agent: %w", err))

			return fmt.Errorf("could not connect to endpoint")
		}
	} else {
		localAgent, err = netmux.NewAgent(ctx, epCfg.Endpoint, d.networkAllocator, netmux.AgentWithMetrics(d.metricsFactroy))
		if err != nil {
			cancel(fmt.Errorf("error creating agent: %w", err))

			return fmt.Errorf("could not connect to endpoint")
		}
	}

	operationalEndPoint := NewOperationalEndPoint()

	operationalEndPoint.agent = localAgent
	operationalEndPoint.cancel = cancel
	operationalEndPoint.config = epCfg
	operationalEndPoint.availableBridges = memstore.New[netmux.Bridge]()

	go func(operationalEndPoint *OperationalEndPoint) {
		for {
			select {
			case <-ctx.Done():
				_ = operationalEndPoint.operationalBridges.ForEach(func(k string, v *OperationalBridge) error {
					if v == nil || v.cancel == nil {
						return nil
					}

					v.cancel(fmt.Errorf("connection closing: %w", ctx.Err()))

					return nil
				})

				return
			case evt := <-localAgent.Events():
				slog.Info(fmt.Sprintf("Event: %v: %s", evt.EvtName, evt.Bridge.String()))

				switch evt.EvtName {
				case netmux.EventBridgeUp:
					operationalEndPoint.availableBridges.Set(evt.Bridge.Name, evt.Bridge)
				case netmux.EventBridgeDel:
					operationalEndPoint.availableBridges.Del(evt.Bridge.Name)
				case netmux.EventBridgeAdd:
					operationalEndPoint.availableBridges.Set(evt.Bridge.Name, evt.Bridge)
				}
			}
		}
	}(operationalEndPoint)

	d.operationalEndpoints.Set(endpointName, operationalEndPoint)

	return nil
}

func (d *Daemon) Disconnect(endpoint string) error {
	managedEndpoint := d.operationalEndpoints.Get(endpoint)
	if managedEndpoint == nil {
		return ErrEndpointNotConnected
	}

	managedEndpoint.cancel(fmt.Errorf("Disconnect called"))
	d.operationalEndpoints.Del(endpoint)

	return nil
}

func (d *Daemon) Exit() {
	os.Exit(0)
}

func (d *Daemon) CleanUp() error {
	if err := d.networkAllocator.CleanUp("nx"); err != nil {
		return fmt.Errorf("error during cleanup: %w", err)
	}

	return nil
}

func (d *Daemon) GetStatus() Status {
	ret := Status{}

	for _, endpoint := range d.cfg.Endpoints {
		epStatus := StatusEndPoints{
			Endpoint: endpoint,
			Status:   "off",
		}

		opEndpoint := d.operationalEndpoints.Get(endpoint.Name)
		if opEndpoint != nil {
			epStatus.Status = "on"
			_ = opEndpoint.availableBridges.ForEach(func(k string, v netmux.Bridge) error {
				bridge := StatusBridges{Bridge: v, Status: "off"}
				opBridge := opEndpoint.operationalBridges.Get(v.Name)
				if opBridge != nil {
					bridge.Status = "on"
				}
				epStatus.Bridges = append(epStatus.Bridges, bridge)

				return nil
			})
		}

		ret.Endpoints = append(ret.Endpoints, epStatus)
	}

	return ret
}

func (d *Daemon) startIndividualService(ctx context.Context, endpoint string, svc string) error {
	managedEndpoint := d.operationalEndpoints.Get(endpoint)
	if managedEndpoint == nil {
		return ErrEndpointNotConnected
	}

	if op := managedEndpoint.operationalBridges.Get(svc); op != nil {
		return ErrBridgeAlreadyConnected
	}

	bridge := managedEndpoint.availableBridges.Get(svc)
	if bridge == (netmux.Bridge{}) {
		return ErrBridgeNotFound
	}

	ctx, cancel := context.WithCancelCause(ctx)

	operationalBridge := &OperationalBridge{
		Bridge: bridge,
		cancel: cancel,
	}

	switch bridge.Direction {
	case netmux.DirectionL2C:
		go func() {
			if err := managedEndpoint.agent.ServeProxy(ctx, bridge); err != nil {
				slog.Warn("error serving proxy", "err", err)
			}
		}()
		managedEndpoint.operationalBridges.Set(svc, operationalBridge)

		return nil
	case netmux.DirectionC2L:
		go func() {
			closer, err := managedEndpoint.agent.ServeReverse(ctx, bridge)
			if err != nil {
				slog.Warn("error serving proxy", "err", err)
			}

			operationalBridge.cancel = closer
		}()
		managedEndpoint.operationalBridges.Set(svc, operationalBridge)

		return nil
	default:
		return ErrBridgeDirectionInvalid
	}
}

//nolint:nestif
func (d *Daemon) StartService(ctx context.Context, endpoint string, svc string) error {
	if strings.Contains(svc, "+") {
		svc := strings.ReplaceAll(svc, "+", ".*")

		svcRegex, err := regexp.Compile(svc)
		if err != nil {
			return fmt.Errorf("error compiling regex: %w", err)
		}

		managedEndpoint := d.operationalEndpoints.Get(endpoint)
		if managedEndpoint == nil {
			return ErrEndpointNotConnected
		}

		errs := make([]string, 0)

		_ = managedEndpoint.availableBridges.ForEach(func(k string, v netmux.Bridge) error {
			if svcRegex.MatchString(k) {
				if err = d.startIndividualService(ctx, endpoint, k); err != nil {
					errs = append(errs, fmt.Sprintf("error closing %s: %s", k, err.Error()))
				}
			}

			return nil
		})

		if len(errs) > 0 {
			return fmt.Errorf(strings.Join(errs, "\n"))
		}

		return nil
	}

	return d.startIndividualService(ctx, endpoint, svc)
}

//nolint:nestif
func (d *Daemon) StopService(endpoint string, svc string) error {
	if strings.Contains(svc, "+") {
		svc := strings.ReplaceAll(svc, "+", ".*")

		svcRegex, err := regexp.Compile(svc)
		if err != nil {
			return fmt.Errorf("error compiling regex: %w", err)
		}

		managedEndpoint := d.operationalEndpoints.Get(endpoint)
		if managedEndpoint == nil {
			return ErrEndpointNotConnected
		}

		errs := make([]string, 0)
		bridges := make([]string, 0)

		_ = managedEndpoint.operationalBridges.ForEach(func(k string, v *OperationalBridge) error {
			if svcRegex.MatchString(k) {
				bridges = append(bridges, k)
			}

			return nil
		})

		for _, k := range bridges {
			if err = d.stopIndividualService(endpoint, k); err != nil {
				errs = append(errs, fmt.Sprintf("error closing %s: %s", k, err.Error()))
			}
		}

		if len(errs) > 0 {
			return fmt.Errorf(strings.Join(errs, "\n"))
		}

		return nil
	}

	return d.stopIndividualService(endpoint, svc)
}

func (d *Daemon) stopIndividualService(endpoint string, svc string) error {
	managedEndpoint := d.operationalEndpoints.Get(endpoint)
	if managedEndpoint == nil {
		return ErrEndpointNotConnected
	}

	operationalBridge := managedEndpoint.operationalBridges.Get(svc)
	if operationalBridge == nil {
		return ErrBridgeNotFound
	}

	operationalBridge.cancel(fmt.Errorf("StopService called for %s", endpoint))
	managedEndpoint.operationalBridges.Del(svc)

	return nil
}

func (d *Daemon) DNSEntries() []dnsallocator.DNSEntry {
	return d.networkAllocator.DNSEntries()
}
