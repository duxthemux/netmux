package netmux

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"runtime"

	"github.com/duxthemux/netmux/foundation/memstore"
	"github.com/duxthemux/netmux/foundation/metrics"
	"github.com/duxthemux/netmux/foundation/pipe"
	"github.com/duxthemux/netmux/foundation/wire"
)

const (
	MaxEventsBacklog = 24
)

func helperError(err error) {
	if err != nil {
		_, file, line, ok := runtime.Caller(1)
		if ok {
			slog.Warn("error", "err", err, "file", file, "line", line)

			return
		}

		slog.Warn("error - could not find caller", "err", err)
	}
}

//----------------------------------------------------------------------------------------------------------------------

// IPAllocator will provide the capability of allocating an IP address in the local machine, while associating it with
// the provided name.
// Release will do the opposite and make that address available for a later call
// Implementations are expected to do something like, adding an entry to the hosts file and associating a new
// ip address to a local interface.
type IPAllocator interface {
	GetIP(name ...string) (string, error)
	ReleaseIP(ip string) error
}

// PerfReporter allows reporting the amount of data copied from A to B and vice versa.
type PerfReporter interface {
	AtoB(total int64)
	BotA(total int64)
}

//----------------------------------------------------------------------------------------------------------------------

type Agent struct {
	endpoint string
	cmdConn  net.Conn
	events   chan Event
	wire     wire.Wire

	ipAllocator IPAllocator

	bridges *memstore.Map[Bridge]
	closers *memstore.Map[io.Closer]

	response            chan CmdRawResponse
	reportMetricFactory metrics.Factory
}

//nolint:funlen,cyclop
func (c *Agent) ServeProxy(ctx context.Context, bridge Bridge) error {
	if ctx.Err() != nil {
		return fmt.Errorf("context cancelled when serving prody: %w", ctx.Err())
	}

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(fmt.Errorf("deferred serveproxy ended"))

	lname := bridge.LocalAddr
	if lname == "" {
		lname = bridge.Name
	}

	ipAddr, err := c.ipAllocator.GetIP(lname)
	if err != nil {
		return fmt.Errorf("error allocating ip for bridge %s: %w", bridge.Name, err)
	}

	defer func() {
		if err := c.ipAllocator.ReleaseIP(ipAddr); err != nil {
			slog.Warn("error releasing ip addr", "bridge", bridge, "err", err)
		}
	}()

	bridge.LocalAddr = ipAddr

	listener, err := net.Listen(bridge.Family, bridge.FullLocalAddr())
	if err != nil {
		return fmt.Errorf("error listening at %s while serving proxy: %w", bridge.FullLocalAddr(), err)
	}

	go func() {
		<-ctx.Done()
		helperIoClose(listener)
	}()

	for {
		cli, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("error acceptin conn: %w", err)
		}

		lcon, err := c.Proxy(ProxyRequest{
			Name:     bridge.Name,
			Family:   bridge.Family,
			Endpoint: bridge.FullContainerAddr(),
		})
		if err != nil {
			return fmt.Errorf("error establishing proxy connection for %s: %w", bridge.Name, err)
		}

		piper := pipe.New(lcon, cli)

		if c.reportMetricFactory != nil {
			obsB := c.reportMetricFactory.New("proxy", "name", "from", "to").
				Counter(map[string]string{
					"name": bridge.Name,
					"from": bridge.FullLocalAddr(),
					"to":   bridge.FullContainerAddr(),
				})

			obsA := c.reportMetricFactory.New("proxy", "name", "from", "to").
				Counter(map[string]string{
					"name": bridge.Name,
					"from": bridge.FullContainerAddr(),
					"to":   bridge.FullLocalAddr(),
				})

			piper.BMetric = obsB.Add

			piper.AMetric = obsA.Add
		}

		go func() {
			if err := piper.Run(ctx); err != nil {
				slog.Warn("error while piping", "bridge", bridge.Name, "err", err)
			}
		}()
	}
}

func (c *Agent) ServeReverse(ctx context.Context, b Bridge) (func(err error), error) {
	return c.RevProxyListen(ctx, RevProxyListenRequest{
		Name:       b.Name,
		Family:     b.Family,
		RemoteAddr: b.FullContainerAddr(),
		LocalAddr:  b.FullLocalAddr(),
	})
}

func (c *Agent) Proxy(req ProxyRequest) (io.ReadWriteCloser, error) {
	if req.Family == "" {
		return nil, fmt.Errorf("no family provided")
	}

	con, err := net.Dial(req.Family, c.endpoint)
	if err != nil {
		return nil, fmt.Errorf("error dialing: %w", err)
	}

	if err = c.wire.WriteJSON(con, CmdProxy, req); err != nil {
		return nil, fmt.Errorf("client.Proxy: error sending command: %w", err)
	}

	return con, nil
}

//nolint:funlen
func (c *Agent) handleRevProxyWork(ctx context.Context, rpe RevProxyWorkRequest, rplreq RevProxyListenRequest) { //nolint:lll
	rconn, err := net.Dial("tcp", c.endpoint)
	if err != nil {
		slog.Warn("Agent.handleRevProxyWork:error dialing remote proxy", "err", err)

		return
	}

	defer helperIoClose(rconn)

	if err = c.wire.WriteJSON(rconn, CmdRevProxyWork, rpe); err != nil {
		slog.Warn("Agent.handleRevProxyWork: error writing to wire", "err", err)

		return
	}

	rperes := RevProxyWorkResponse{}
	if err = c.wire.ReadJSON(rconn, CmdRevProxyWork, &rperes); err != nil {
		slog.Warn("Agent.handleRevProxyWork: error receiving work confirmation", "err", err)

		return
	}

	lconn, err := net.Dial("tcp", rplreq.LocalAddr)
	if err != nil {
		slog.Warn("error opening local port", "err", err)

		return
	}

	defer helperIoClose(lconn)

	slog.Debug(
		"client.handleRevProxyWork: proxying:",
		"rconn-addr",
		rconn.RemoteAddr().String(),
		"lconn-addr",
		lconn.RemoteAddr().String())

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(fmt.Errorf("handleRevProxyWork ended"))

	piper := pipe.New(lconn, rconn)

	if c.reportMetricFactory != nil {
		obsB := c.reportMetricFactory.New("rev-proxy", "name", "from", "to").
			Counter(map[string]string{
				"name": rplreq.Name,
				"from": rplreq.RemoteAddr,
				"to":   rplreq.LocalAddr,
			})

		obsA := c.reportMetricFactory.New("rev-proxy", "name", "from", "to").
			Counter(map[string]string{
				"name": rplreq.Name,
				"from": rplreq.LocalAddr,
				"to":   rplreq.RemoteAddr,
			})

		piper.BMetric = obsB.Add

		piper.AMetric = obsA.Add
	}

	if err := piper.Run(ctx); err != nil {
		slog.Warn("error piping rev proxy work", "err", err)
	}
}

func (c *Agent) handleRevProxyListen(ctx context.Context, conn net.Conn, req RevProxyListenRequest) {
	for {
		rpe := RevProxyWorkRequest{}

		if err := c.wire.ReadJSON(conn, CmdRevProxyWork, &rpe); err != nil {
			slog.Warn("error receiving payload", "err", err)

			return
		}

		slog.Debug("client.handleRevProxyListen: got new conn", "addr", conn.RemoteAddr().String())

		go c.handleRevProxyWork(ctx, rpe, req)
	}
}

func (c *Agent) RevProxyListen(ctx context.Context, req RevProxyListenRequest) (func(err error), error) {
	con, err := net.Dial("tcp", c.endpoint)
	if err != nil {
		return nil, fmt.Errorf("error dialing: %w", err)
	}

	if err = c.wire.WriteJSON(con, CmdRevProxyListen, req); err != nil {
		return nil, fmt.Errorf("error marshalling request: %w", err)
	}

	rplres := RevProxyListenResponse{}
	if err = c.wire.ReadJSON(con, CmdRevProxyListen, &rplres); err != nil {
		return nil, fmt.Errorf("error reading rev proxy listen confirmation")
	}

	go func() {
		c.handleRevProxyListen(ctx, con, req)
	}()

	return func(err error) {
		if err != nil {
			slog.Warn("error closing rev. proxy", "err", err)
		}

		helperIoClose(con)
	}, nil
}

func (c *Agent) Events() <-chan Event {
	return c.events
}

//nolint:cyclop,funlen
func (c *Agent) handleControlMessages(ctx context.Context, cmdConn net.Conn) error {
	if ctx.Err() != nil {
		return fmt.Errorf("context closed when handling control messages: %w", ctx.Err())
	}

	go func() {
		<-ctx.Done()
		helperIoClose(cmdConn)
	}()

	for {
		cmd, payload, err := c.wire.Read(cmdConn)
		if err != nil {
			slog.Warn("client: error reading from control conn", "err", err)
			helperIoClose(cmdConn)
			close(c.events)

			return fmt.Errorf("client: error reading from control conn: %w", err)
		}

		slog.Debug("Event received", "evt", string(payload))

		switch cmd {
		case CmdEvents:
			if len(c.events) == MaxEventsBacklog {
				var oldest Event

				slog.Warn("client: events chan is full, will discard oldest one", "evt", oldest)

				<-c.events
			}

			anEvent := Event{}
			if err := json.Unmarshal(payload, &anEvent); err != nil {
				slog.Warn("client: error unmarshalling event", "err", err)
			}

			switch anEvent.EvtName {
			case EventBridgeAdd:
				c.bridges.Add(anEvent.Bridge)
			case EventBridgeDel:
				c.bridges.Del(anEvent.Bridge.Name)

				if closer := c.closers.Get(anEvent.Bridge.Name); closer != nil {
					helperIoClose(closer)
				}
			case EventBridgeUp:
				if closer := c.closers.Get(anEvent.Bridge.Name); closer != nil {
					helperIoClose(closer)
				}

				c.bridges.Del(anEvent.Bridge.Name)
				c.bridges.Add(anEvent.Bridge)
			}

			c.events <- anEvent
		default:
			c.response <- CmdRawResponse{
				Cmd: cmd,
				Pl:  payload,
			}
		}
	}
}

type AgentOpts func(a *Agent)

func AgentWithMetrics(m metrics.Factory) AgentOpts {
	return func(a *Agent) {
		if m != nil {
			a.reportMetricFactory = m
		}
	}
}

func NewAgent(ctx context.Context, endponit string, ipAllocator IPAllocator, opts ...AgentOpts) (*Agent, error) {
	if ctx.Err() != nil {
		return nil, fmt.Errorf("context cancelled when creating agent: %w", ctx.Err())
	}

	localWire := wire.Wire{}

	cmdConn, err := net.Dial("tcp", endponit)
	if err != nil {
		return nil, fmt.Errorf("error dialing endpoint: %w", err)
	}

	go func() {
		<-ctx.Done()
		helperIoClose(cmdConn)
	}()

	cmdConnControlRequest := CmdConnControlRequest{}
	if err = localWire.WriteJSON(cmdConn, CmdControl, cmdConnControlRequest); err != nil {
		return nil, fmt.Errorf("error opening control conn: %w", err)
	}

	cmdConnControlResponse := CmdConnControlResponse{}
	if err = localWire.ReadJSON(cmdConn, CmdControl, &cmdConnControlResponse); err != nil {
		return nil, fmt.Errorf("error opening control conn: %w", err)
	}

	ret := &Agent{
		wire:        localWire,
		endpoint:    endponit,
		cmdConn:     cmdConn,
		events:      make(chan Event, MaxEventsBacklog),
		response:    make(chan CmdRawResponse),
		bridges:     memstore.New[Bridge](),
		closers:     memstore.New[io.Closer](),
		ipAllocator: ipAllocator,
	}

	for _, opt := range opts {
		opt(ret)
	}

	go func(ctx context.Context) {
		helperError(ret.handleControlMessages(ctx, cmdConn))
	}(ctx)

	return ret, nil
}
