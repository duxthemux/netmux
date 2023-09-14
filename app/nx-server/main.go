package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"

	"golang.org/x/sync/errgroup"

	k8s2 "go.digitalcircle.com.br/dc/netmux/app/nx-server/runtime/k8s"
	"go.digitalcircle.com.br/dc/netmux/business/netmux"
	"go.digitalcircle.com.br/dc/netmux/foundation/buildinfo"
	"go.digitalcircle.com.br/dc/netmux/foundation/metrics"
)

const (
	EnvLogLevel = "LOGLEVEL"
	EnvLogSrc   = "LOGSRC"
)

func logInit() {
	logLevel := os.Getenv(EnvLogLevel)

	slogLevel := slog.LevelInfo

	switch logLevel {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	}

	slogAddSource := strings.ToLower(os.Getenv(EnvLogSrc)) == "true"

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource:   slogAddSource,
		Level:       slogLevel,
		ReplaceAttr: nil,
	}))

	slog.SetDefault(logger)
}

//nolint:funlen
func run() error {
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(fmt.Errorf("nx-server run ended"))

	metricsProvider := metrics.NewPromFactory()

	netmuxService := netmux.NewService(
		netmux.WithEventsLogger(func(evt netmux.Event) {
			slog.Info(fmt.Sprintf("Event: %v: %s", evt.EvtName, evt.Bridge.String()))
		}),
		netmux.WithMetrics(metricsProvider),
	)

	logInit()

	slog.Info(buildinfo.StringOneLine("nx-server"))

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":50000"
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("error setting up service listener: %w", err)
	}

	k8sRuntime := k8s2.NewRuntime(k8s2.Opts{})

	defer func() {
		if err := k8sRuntime.Close(); err != nil {
			slog.Warn("error closing k8s runtime", "err", err)
		}
	}()

	probe := k8s2.NewProbe(":8083")

	netmuxService.AddEventSource(ctx, k8sRuntime)

	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		defer cancel(fmt.Errorf("k8sRuntime ended"))

		return k8sRuntime.Run(ctx) //nolint:wrapcheck
	})

	group.Go(func() error {
		defer cancel(fmt.Errorf("netmuxService ended"))

		return netmuxService.Serve(ctx, listener) //nolint:wrapcheck
	})

	group.Go(func() error {
		defer cancel(fmt.Errorf("probe ended"))

		return probe.Run(ctx) //nolint:wrapcheck
	})

	group.Go(func() error {
		defer cancel(fmt.Errorf("metricsServer ended"))

		return metricsProvider.Start(ctx, ":8081") //nolint:wrapcheck
	})

	probe.Ready()

	return group.Wait() //nolint:wrapcheck
}

func main() {
	err := run()
	if err != nil {
		panic(err)
	}
}
