package webserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"go.digitalcircle.com.br/dc/netmux/app/nx-daemon/webserver/api"

	"go.digitalcircle.com.br/dc/netmux/app/nx-daemon/daemon"
	"go.digitalcircle.com.br/dc/netmux/business/caroot"
)

type WebServer struct {
	Service *daemon.Daemon
	server  http.Server
	api     api.API
}

const ReaderTimeout = time.Second * 5

//nolint:funlen
func (w *WebServer) Run(ctx context.Context, name string, addr string, port string, certAuth *caroot.CA) error {
	root := mux.NewRouter()

	root.Handle("/metrics", promhttp.Handler()).Name("prometheus-metrics")

	err := w.api.Plugin(ctx, root, certAuth)
	if err != nil {
		return fmt.Errorf("error adding api routes: %w", err)
	}

	cert, err := certAuth.GetOrGenFromRoot(name)
	if err != nil {
		return fmt.Errorf("could not retrieve certificate for %s: %w", name, err)
	}

	server := http.Server{
		Addr:    addr + ":" + port,
		Handler: root,
		TLSConfig: &tls.Config{
			MinVersion:   tls.VersionTLS13,
			NextProtos:   []string{"http/1.1"},
			Certificates: []tls.Certificate{cert},
		},
		ReadHeaderTimeout: ReaderTimeout,
	}

	go func() {
		<-ctx.Done()
		_ = server.Shutdown(ctx)
	}()

	err = root.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		tmpl, err := route.GetPathTemplate()
		if err != nil {
			return fmt.Errorf("error getting path template: %w", err)
		}
		methods, _ := route.GetMethods()

		methodsStr := strings.Join(methods, ", ")
		if len(methods) == 0 {
			methodsStr = "ALL"
		}

		name := route.GetName()

		slog.Debug(fmt.Sprintf("* %s: [%s] %s", name, methodsStr, tmpl))

		return nil
	})

	if err != nil {
		return fmt.Errorf("error walking routes: %w", err)
	}

	slog.Info(fmt.Sprintf("Webserver running at: %s:%s", addr, port))

	err = server.ListenAndServeTLS("", "")
	if err != nil {
		return fmt.Errorf("error running server: %w", err)
	}

	return nil
}

const ReadHeaderTimeout = time.Second * 5

func New(svc *daemon.Daemon) *WebServer {
	ret := &WebServer{
		Service: svc,
		server: http.Server{
			ReadHeaderTimeout: ReadHeaderTimeout,
		},
		api: api.API{Service: svc},
	}

	return ret
}
