package netmux_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.digitalcircle.com.br/dc/netmux/business/netmux"
)

const MaxWaitTime = time.Second * 5

func doClose(c io.Closer) {
	err := c.Close()
	if err != nil {
		slog.Warn("error closing", "err", err)
	}
}

type ZeroIPAllocator struct{}

func (z *ZeroIPAllocator) GetIP(_ ...string) (string, error) {
	return "0.0.0.0", nil
}

func (z *ZeroIPAllocator) ReleaseIP(_ string) error {
	return nil
}

//nolint:funlen,paralleltest,cyclop
func TestProxy(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource:   true,
		Level:       slog.LevelDebug,
		ReplaceAttr: nil,
	}))

	slog.SetDefault(logger)

	netmuxServiceListener, err := net.Listen("tcp", "")
	assert.NoError(t, err)

	defer doClose(netmuxServiceListener)

	srv := netmux.NewService()

	chErr := make(chan error)

	proxiedUserServiceListener, err := net.Listen("tcp", "")
	assert.NoError(t, err)

	defer doClose(proxiedUserServiceListener)

	go func() {
		for {
			pconn, err := proxiedUserServiceListener.Accept()
			if err != nil {
				chErr <- err

				return
			}

			buff := make([]byte, 128)

			numBytes, err := pconn.Read(buff)
			if err != nil {
				chErr <- err

				return
			}

			assert.Equal(t, "ok", string(buff[:numBytes]))

			if _, err = pconn.Write([]byte("ok")); err != nil {
				chErr <- err

				return
			}
		}
	}()

	go func() {
		err = srv.Serve(ctx, netmuxServiceListener)
		if err != nil {
			chErr <- err
		}
	}()

	go func() {
		cli, err := netmux.NewAgent(ctx, netmuxServiceListener.Addr().String(), &ZeroIPAllocator{})
		assert.NoError(t, err)

		cliRwd, err := cli.Proxy(netmux.ProxyRequest{
			Message:  netmux.Message{},
			Family:   "tcp",
			Endpoint: proxiedUserServiceListener.Addr().String(),
		})
		if err != nil {
			chErr <- err

			return
		}

		if _, err = cliRwd.Write([]byte("ok")); err != nil {
			chErr <- err

			return
		}

		buf := make([]byte, 128)

		n, err := cliRwd.Read(buf)
		if err != nil {
			chErr <- err
		}

		assert.Equal(t, string(buf[:n]), "ok")
		chErr <- nil
	}()

	select {
	case err = <-chErr:
		assert.NoError(t, err)
	case <-time.After(MaxWaitTime):
		t.Fatalf("test timed out")
	}
}

//nolint:funlen,paralleltest,cyclop
func TestProxyCancelConnFromProxy(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource:   true,
		Level:       slog.LevelDebug,
		ReplaceAttr: nil,
	}))

	slog.SetDefault(logger)

	netmuxServiceListener, err := net.Listen("tcp", "")

	assert.NoError(t, err)

	defer doClose(netmuxServiceListener)

	srv := netmux.NewService()

	chErr := make(chan error)

	proxiedUserServiceListener, err := net.Listen("tcp", "")
	assert.NoError(t, err)

	defer doClose(proxiedUserServiceListener)

	go func() {
		for {
			pconn, err := proxiedUserServiceListener.Accept()
			if err != nil {
				chErr <- err

				return
			}

			buff := make([]byte, 128)

			numBytes, err := pconn.Read(buff)
			if err != nil {
				chErr <- err

				return
			}

			assert.Equal(t, "ok", string(buff[:numBytes]))

			if _, err = pconn.Write([]byte("ok")); err != nil {
				chErr <- err

				return
			}
		}
	}()

	go func() {
		err = srv.Serve(ctx, netmuxServiceListener)
		if err != nil {
			chErr <- err
		}
	}()

	go func() {
		cli, err := netmux.NewAgent(ctx, netmuxServiceListener.Addr().String(), &ZeroIPAllocator{})
		assert.NoError(t, err)

		cliRwd, err := cli.Proxy(netmux.ProxyRequest{
			Message:  netmux.Message{},
			Family:   "tcp",
			Endpoint: proxiedUserServiceListener.Addr().String(),
		})
		if err != nil {
			chErr <- err

			return
		}

		if _, err = cliRwd.Write([]byte("ok")); err != nil {
			chErr <- err

			return
		}

		err = cliRwd.Close()
		assert.NoError(t, err)

		buf := make([]byte, 128)

		_, err = cliRwd.Read(buf)
		if err != nil {
			chErr <- err
		}

		assert.Error(t, err, "using closed conn")

		chErr <- nil
	}()

	select {
	case err = <-chErr:
		assert.Errorf(t, err, "closed too soon")
	case <-time.After(MaxWaitTime):
		t.Fatalf("test timed out")
	}
}

//nolint:funlen,paralleltest
func TestProxyCancelConnectionFromService(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource:   true,
		Level:       slog.LevelDebug,
		ReplaceAttr: nil,
	}))

	slog.SetDefault(logger)

	netmuxServiceListener, err := net.Listen("tcp", "")
	assert.NoError(t, err)

	defer doClose(netmuxServiceListener)

	srv := netmux.NewService()

	chErr := make(chan error)

	proxiedUserServiceListener, err := net.Listen("tcp", "")
	assert.NoError(t, err)

	defer doClose(proxiedUserServiceListener)

	go func() {
		for {
			pconn, err := proxiedUserServiceListener.Accept()
			if err != nil {
				chErr <- err

				return
			}

			time.Sleep(time.Second)

			_ = pconn.Close()
		}
	}()

	go func() {
		err = srv.Serve(ctx, netmuxServiceListener)
		if err != nil {
			chErr <- err
		}
	}()

	go func() {
		cli, err := netmux.NewAgent(ctx, netmuxServiceListener.Addr().String(), &ZeroIPAllocator{})
		assert.NoError(t, err)

		cliRwd, err := cli.Proxy(netmux.ProxyRequest{
			Message:  netmux.Message{},
			Family:   "tcp",
			Endpoint: proxiedUserServiceListener.Addr().String(),
		})
		if err != nil {
			chErr <- err

			return
		}

		if _, err = cliRwd.Write([]byte("ok")); err != nil {
			chErr <- err

			return
		}

		buffer := make([]byte, 128)

		_, err = cliRwd.Read(buffer)
		if err != nil {
			chErr <- err

			return
		}

		assert.Error(t, err, "EOF")

		chErr <- nil
	}()

	select {
	case err = <-chErr:
		assert.Errorf(t, err, "EOF")
	case <-time.After(MaxWaitTime):
		t.Fatalf("test timed out")
	}
}

//nolint:paralleltest
func TestEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource:   true,
		Level:       slog.LevelDebug,
		ReplaceAttr: nil,
	}))

	slog.SetDefault(logger)

	netmuxServiceListener, err := net.Listen("tcp", "")
	assert.NoError(t, err)

	defer doClose(netmuxServiceListener)

	srv := netmux.NewService()

	chErr := make(chan error)
	chListenerReady := make(chan bool)

	go func() {
		err = srv.Serve(ctx, netmuxServiceListener)
		if err != nil {
			chErr <- err
		}
	}()

	go func() {
		cli, err := netmux.NewAgent(ctx, netmuxServiceListener.Addr().String(), &ZeroIPAllocator{})
		assert.NoError(t, err)
		chListenerReady <- true

		evt := <-cli.Events()
		assert.Equal(t, evt.EvtName, "123")

		chErr <- nil
	}()
	<-chListenerReady

	err = srv.SendEvent(netmux.Event{EvtName: "123"})
	assert.NoError(t, err)

	select {
	case err = <-chErr:
		assert.NoError(t, err)
	case <-time.After(MaxWaitTime):
		t.Fatalf("test timed out")
	}
}

//nolint:funlen,paralleltest
func TestRevProxy(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource:   true,
		Level:       slog.LevelDebug,
		ReplaceAttr: nil,
	}))

	slog.SetDefault(logger)

	netmuxServiceListener, err := net.Listen("tcp", "")
	assert.NoError(t, err)

	defer doClose(netmuxServiceListener)

	t.Logf("netmux service will listen at %s", netmuxServiceListener.Addr().String())

	srv := netmux.NewService()

	chErr := make(chan error)

	userServiceListener, err := net.Listen("tcp", "")
	assert.NoError(t, err)
	t.Logf("user service will listen at %s", userServiceListener.Addr().String())

	defer doClose(userServiceListener)

	go func() {
		for {
			pconn, err := userServiceListener.Accept()
			if err != nil {
				chErr <- err

				return
			}

			if _, err = pconn.Write([]byte("ok")); err != nil {
				chErr <- err
			}

			doClose(pconn)
		}
	}()

	go func() {
		err = srv.Serve(ctx, netmuxServiceListener)
		if err != nil {
			chErr <- err
		}
	}()

	tmpListener, err := net.Listen("tcp", "")
	assert.NoError(t, err)

	addr := tmpListener.Addr()

	t.Logf("local proxy service will listen at %s", addr.String())

	doClose(tmpListener)

	go func() {
		cli, err := netmux.NewAgent(ctx, netmuxServiceListener.Addr().String(), &ZeroIPAllocator{})
		assert.NoError(t, err)

		closeListener, err := cli.RevProxyListen(ctx, netmux.RevProxyListenRequest{
			Family:     "tcp",
			RemoteAddr: addr.String(),
			LocalAddr:  userServiceListener.Addr().String(),
		})
		assert.NoError(t, err)

		defer closeListener(nil)

		conn, err := net.Dial("tcp", addr.String())
		if err != nil {
			chErr <- err

			return
		}

		buf := make([]byte, 128)

		n, err := conn.Read(buf)
		if err != nil {
			chErr <- err
		}

		assert.Equal(t, string(buf[:n]), "ok")
		chErr <- nil
	}()

	select {
	case err = <-chErr:
		assert.NoError(t, err)
	case <-time.After(MaxWaitTime):
		t.Fatalf("test timed out")
	}
}

type TestEventSource struct {
	ch chan netmux.Event
}

func (t *TestEventSource) Events() <-chan netmux.Event {
	return t.ch
}

//nolint:funlen,paralleltest
func TestRecvEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource:   true,
		Level:       slog.LevelDebug,
		ReplaceAttr: nil,
	}))

	slog.SetDefault(logger)

	netmuxServiceListener, err := net.Listen("tcp", "")
	assert.NoError(t, err)

	defer doClose(netmuxServiceListener)

	testEventSource := &TestEventSource{
		ch: make(chan netmux.Event),
	}

	srv := netmux.NewService()
	srv.AddEventSource(ctx, testEventSource)

	for i := 0; i < 3; i++ {
		testEventSource.ch <- netmux.Event{
			EvtName: netmux.EventBridgeAdd,
			Bridge:  netmux.Bridge{Name: fmt.Sprintf("B0%d", i)},
		}
		assert.NoError(t, err)
	}

	chErr := make(chan error)

	go func() {
		err = srv.Serve(ctx, netmuxServiceListener)
		if err != nil {
			chErr <- err
		}
	}()

	cli, err := netmux.NewAgent(ctx, netmuxServiceListener.Addr().String(), &ZeroIPAllocator{})
	assert.NoError(t, err)

	go func() {
		for i := 0; i < 6; i++ {
			evt := <-cli.Events()

			log.Printf("%#v", evt)

			if i == 5 {
				chErr <- nil
			}
		}
	}()

	go func() {
		for i := 0; i < 3; i++ {
			testEventSource.ch <- netmux.Event{
				EvtName: netmux.EventBridgeAdd,
				Bridge:  netmux.Bridge{Name: fmt.Sprintf("B1%d", i)},
			}
			assert.NoError(t, err)
		}
	}()

	select {
	case err = <-chErr:
		assert.NoError(t, err)
	case <-time.After(MaxWaitTime):
		t.Fatalf("test timed out")
	}
}
