package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"

	"golang.org/x/sync/errgroup"

	"github.com/duxthemux/netmux/app/nx-server/runtime/k8s"
	"github.com/duxthemux/netmux/business/netmux"
	"github.com/duxthemux/netmux/foundation/buildinfo"
	"github.com/duxthemux/netmux/foundation/metrics"
)

const (
	EnvLogLevel = "LOGLEVEL"
	EnvLogSrc   = "LOGSRC"
)

func logInit() {
	logLevel := os.Getenv(EnvLogLevel)

	slogLevel := slog.LevelInfo

	err := slogLevel.UnmarshalText([]byte(logLevel))
	if err != nil {
		slogLevel = slog.LevelDebug
	}

	slogAddSource, _ := strconv.ParseBool(os.Getenv(EnvLogSrc))

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

	k8sRuntime := k8s.NewRuntime(k8s.Opts{})

	defer func() {
		if err := k8sRuntime.Close(); err != nil {
			slog.Warn("error closing k8s runtime", "err", err)
		}
	}()

	probe := k8s.NewProbe(":8083")

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
