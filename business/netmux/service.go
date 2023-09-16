// service implements the core service of netmux. Both Service and Agent are defined here. Also the protocol
// established for comunicating.
package netmux

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"runtime"

	"go.digitalcircle.com.br/dc/netmux/foundation/memstore"
	"go.digitalcircle.com.br/dc/netmux/foundation/metrics"
	"go.digitalcircle.com.br/dc/netmux/foundation/pipe"
	"go.digitalcircle.com.br/dc/netmux/foundation/wire"
)

func helperIoClose(closer io.Closer) {
	if err := closer.Close(); err != nil {
		_, file, line, ok := runtime.Caller(1)
		if ok {
			slog.Warn("error closing", "err", err, "file", file, "line", line)

			return
		}

		slog.Warn("error closing - could not find caller", "err", err)
	}
}

// ---------------------------------------------------------------------------------------------------------------------

// EventSource is an external entity that will provide our service with relevant events that will either adjust its
// own behavior, and be propagated to connected agents.
type EventSource interface {
	Events() <-chan Event
}

// ---------------------------------------------------------------------------------------------------------------------

type CmdHandler func(ctx context.Context, req []byte) ([]byte, error)

// Service defines the core Netmux service. This is the software component running from inside infrastructure
// that will accept and process requests from agents running on remote machines.
type Service struct {
	cmdConns            *memstore.Map[net.Conn]
	revProxyConns       *memstore.Map[net.Conn]
	bridges             *memstore.Map[Bridge]
	wire                *wire.Wire
	cmdHandler          map[uint16]CmdHandler
	eventsLogger        func(e Event)
	reportMetricFactory metrics.Factory
}

// SendEvent allows publishing of events. Each event will be broadcast to all connected agents.
func (s *Service) SendEvent(e Event) error {
	payload, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("error marshalling event: %w", err)
	}

	_ = s.cmdConns.ForEach(func(k string, v net.Conn) error {
		if err := s.wire.Write(v, CmdEvents, payload); err != nil {
			slog.Warn("error broadcasting", "conn", v, "err", err)
		}

		return nil
	})

	return nil
}

// Serve will connect this service instance with the provided listener, so that agents can connect and have their
// requests fulfilled. Once the context is done, the listener will be closed from inside this method.
func (s *Service) Serve(ctx context.Context, listener net.Listener) error {
	if ctx.Err() != nil {
		return fmt.Errorf("context canceled when serving: %w", ctx.Err())
	}

	go func() {
		<-ctx.Done()
		helperIoClose(listener)
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("error accepting connection: %w", err)
		}

		slog.Debug("netmux: got new conn", "r-addr", conn.RemoteAddr().String())

		go func() {
			if err = s.handleConn(ctx, conn); err != nil {
				slog.Warn("error handling conn", "err", err, "raddr", conn.RemoteAddr().String())
			}

			helperIoClose(conn)
		}()
	}
}

// handleConn is the first step for handling connections. Some connections may be event oriented, others may be
// stream oriented, thus initial assessment is required here.
//
//nolint:cyclop
func (s *Service) handleConn(ctx context.Context, conn net.Conn) error {
	if ctx.Err() != nil {
		return fmt.Errorf("context canclled when handling connection: %w", ctx.Err())
	}

	command, payload, err := s.wire.Read(conn)
	if err != nil {
		slog.Error("error reading proto", "err", err.Error())
	}

	slog.Debug(
		"server.handleConn: got msg",
		"command", CmdToString(command),
		"payload", string(payload))

	switch command {
	case CmdControl:
		id := s.cmdConns.Add(conn)
		defer s.cmdConns.Del(id)

		if err = s.handleCmdConn(ctx, conn); err != nil {
			return fmt.Errorf("error handling command conn: %w", err)
		}
	case CmdProxy:
		req := ProxyRequest{}

		if err = json.Unmarshal(payload, &req); err != nil {
			return fmt.Errorf("error handling proxy conn: %w", err)
		}

		if err = s.handleProxyConn(ctx, conn, req); err != nil {
			return fmt.Errorf("error handling proxy conn: %w", err)
		}
	case CmdRevProxyListen:
		req := RevProxyListenRequest{}

		if err = json.Unmarshal(payload, &req); err != nil {
			return fmt.Errorf("error handling rev proxy listening conn: %w", err)
		}

		if err = s.handleRevProxyListen(ctx, conn, req); err != nil {
			return fmt.Errorf("error handling rev proxy listening conn: %w", err)
		}
	case CmdRevProxyWork:
		req := RevProxyWorkRequest{}

		if err = json.Unmarshal(payload, &req); err != nil {
			return fmt.Errorf("error handling rev proxy work conn: %w", err)
		}

		if err = s.handleRevProxyWork(ctx, conn, req); err != nil {
			return fmt.Errorf("error handling rev proxy work conn: %w", err)
		}
	default:
		slog.Warn(fmt.Sprintf("received unknown command: %s", CmdToString(command)))
	}

	return nil
}

// handleCmdConn handles commands - typically this will handle the persistent connection between agent
// and Service.
//
//nolint:cyclop
func (s *Service) handleCmdConn(ctx context.Context, conn net.Conn) error {
	if ctx.Err() != nil {
		return fmt.Errorf("context cancelled when handling command conn: %w", ctx.Err())
	}

	if err := s.wire.WriteJSON(conn, CmdControl, CmdConnControlResponse{}); err != nil {
		return fmt.Errorf("error writing to %s: %w", conn.RemoteAddr().String(), err)
	}

	if err := s.bridges.ForEach(func(_ string, b Bridge) error {
		evt := Event{
			EvtName: EventBridgeAdd,
			Bridge:  b,
		}

		if err := s.wire.WriteJSON(conn, CmdEvents, evt); err != nil {
			return fmt.Errorf("error writing to %s: %w", conn.RemoteAddr().String(), err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("error propagating initial bridges: %w", err)
	}

	for {
		cmd, payload, err := s.wire.Read(conn)
		if err != nil {
			return fmt.Errorf("error retrieving cmd from conn %s: %w", conn.RemoteAddr().String(), err)
		}

		handler, ok := s.cmdHandler[cmd]
		if !ok {
			if err = s.wire.Write(conn, CmdUnknown, nil); err != nil {
				slog.Warn("error writing package", "raddr", conn.RemoteAddr().String(), "err", err)
			}
		}

		res, err := handler(ctx, payload)
		if err != nil {
			if err = s.wire.WriteJSON(conn, cmd, Message{Err: err.Error()}); err != nil {
				slog.Warn("error writing package", "raddr", conn.RemoteAddr().String(), "err", err)
			}

			continue
		}

		if err = s.wire.Write(conn, cmd, res); err != nil {
			return fmt.Errorf("error writing command to %s: %w", conn.RemoteAddr().String(), err)
		}

		return nil
	}
}

func (s *Service) handleProxyConn(ctx context.Context, conn net.Conn, req ProxyRequest) error {
	if ctx.Err() != nil {
		return fmt.Errorf("context cancelled handling proxy conn: %w", ctx.Err())
	}

	if req.Family == "" {
		req.Family = "tcp"
	}

	proxy, err := net.Dial(req.Family, req.Endpoint)
	if err != nil {
		return fmt.Errorf("error connecting to proxy endopint: %w", err)
	}

	piper := pipe.New(conn, proxy)

	if s.reportMetricFactory != nil {
		obsB := s.reportMetricFactory.New("proxy", "name", "from", "to").
			Counter(map[string]string{
				"name": "",
				"from": conn.LocalAddr().String(),
				"to":   conn.RemoteAddr().String(),
			})

		obsA := s.reportMetricFactory.New("proxy", "name", "from", "to").
			Counter(map[string]string{
				"name": "",
				"from": conn.RemoteAddr().String(),
				"to":   conn.LocalAddr().String(),
			})

		piper.BMetric = obsB.Add

		piper.AMetric = obsA.Add
	}

	if err = piper.Run(ctx); err != nil {
		return fmt.Errorf("error during handleProxyConn - piping: %w", err)
	}

	return nil
}

func (s *Service) handleRevProxyListen(ctx context.Context, srcConn net.Conn, req RevProxyListenRequest) error {
	if ctx.Err() != nil {
		return fmt.Errorf("context cancelled when handling rev proxy listen: %w", ctx.Err())
	}

	_, port, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return fmt.Errorf("could not parse port from addr %s: %w", req.RemoteAddr, err)
	}

	listener, err := net.Listen(req.Family, ":"+port)
	if err != nil {
		return fmt.Errorf("error setting up listener for: %s:%s: %w", req.Family, port, err)
	}

	defer helperIoClose(listener)

	if err = s.wire.WriteJSON(srcConn, CmdRevProxyListen, RevProxyListenResponse{}); err != nil {
		return fmt.Errorf("confirmation response: error sending response, %w", err)
	}

	for {
		rconn, err := listener.Accept()
		if err != nil {
			slog.Warn("error receiving conn", "peer", rconn.RemoteAddr(), "err", err)

			return fmt.Errorf("error handleRevProxyListen: %w", err)
		}

		id := s.revProxyConns.Add(rconn)

		slog.Debug("handleRevProxyListen: got new connection", "id", id, "rconn", rconn.RemoteAddr().String())

		revConnEvent := RevProxyEvent{
			ID: id,
		}

		bytes, err := json.Marshal(revConnEvent)
		if err != nil {
			slog.Warn("error marshalling rev proxy event", "err", err)

			continue
		}

		if err = s.wire.Write(srcConn, CmdRevProxyWork, bytes); err != nil {
			return fmt.Errorf("error forwarding event: %w", err)
		}
	}
}

func (s *Service) handleRevProxyWork(ctx context.Context, conn net.Conn, req RevProxyWorkRequest) error {
	if ctx.Err() != nil {
		return fmt.Errorf("context cancelled when handling rev proxy work: %w", ctx.Err())
	}

	ctx, cancel := context.WithCancelCause(ctx)

	defer cancel(fmt.Errorf("deffered handleRevProxyWork"))

	proxy := s.revProxyConns.Get(req.ID)
	if proxy == nil {
		return fmt.Errorf("error connecting to proxy")
	}

	slog.Debug(
		"handleRevProxyWork",
		"id", req.ID,
		"remote-addr", proxy.RemoteAddr().String(),
		"conn-addr", conn.RemoteAddr().String())

	if err := s.wire.WriteJSON(conn, CmdRevProxyWork, RevProxyWorkResponse{}); err != nil {
		return fmt.Errorf("server.handleRevProxyWork: error sending confirmation: %w", err)
	}

	piper := pipe.New(conn, proxy)

	if s.reportMetricFactory != nil {
		obsB := s.reportMetricFactory.New("proxy", "name", "from", "to").
			Counter(map[string]string{
				"name": "",
				"from": conn.LocalAddr().String(),
				"to":   conn.RemoteAddr().String(),
			})

		obsA := s.reportMetricFactory.New("proxy", "name", "from", "to").
			Counter(map[string]string{
				"name": "",
				"from": conn.RemoteAddr().String(),
				"to":   conn.LocalAddr().String(),
			})

		piper.BMetric = obsB.Add

		piper.AMetric = obsA.Add
	}

	if err := piper.Run(ctx); err != nil {
		return fmt.Errorf("error piping data: %w", err)
	}

	return nil
}

func (s *Service) AddEventSource(ctx context.Context, src EventSource) {
	go func() {
		for {
			select {
			case evt := <-src.Events():
				if evt == (Event{}) {
					return
				}

				s.eventsLogger(evt)

				switch evt.EvtName {
				case EventBridgeAdd, EventBridgeUp:
					s.bridges.Add(evt.Bridge)
				case EventBridgeDel:
					s.bridges.Del(evt.Bridge.Name)
				}

				if err := s.SendEvent(evt); err != nil {
					slog.Warn("error propagating event", "err", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

type Opts func(s *Service)

func WithEventsLogger(el func(e Event)) Opts {
	return func(s *Service) {
		s.eventsLogger = el
	}
}

func WithMetrics(rmf metrics.Factory) Opts {
	return func(s *Service) {
		s.reportMetricFactory = rmf
	}
}

func NewService(opts ...Opts) *Service {
	ret := Service{
		cmdConns:      memstore.New[net.Conn](),
		revProxyConns: memstore.New[net.Conn](),
		cmdHandler:    map[uint16]CmdHandler{},
		bridges:       memstore.New[Bridge](),
		eventsLogger: func(e Event) {
		},
	}

	for _, o := range opts {
		o(&ret)
	}

	return &ret
}
