package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"
	"gopkg.in/natefinch/lumberjack.v2"

	configlib "go.digitalcircle.com.br/dc/netmux/app/nx-daemon/config"
	"go.digitalcircle.com.br/dc/netmux/app/nx-daemon/daemon"
	"go.digitalcircle.com.br/dc/netmux/business/caroot"
	"go.digitalcircle.com.br/dc/netmux/business/networkallocator"
	"go.digitalcircle.com.br/dc/netmux/foundation/buildinfo"
	"go.digitalcircle.com.br/dc/netmux/foundation/metrics"

	"go.digitalcircle.com.br/dc/netmux/app/nx-daemon/webserver"
)

const (
	MaxSize    = 1
	MaxBackups = 3
	MaxAge     = 28
)

func setupLog() {
	var logWriter io.Writer = os.Stdout

	if os.Getenv("LOGFILE") != "-" {
		logWriter = &lumberjack.Logger{
			Filename:   configlib.DefaultLogFile,
			MaxSize:    MaxSize, // megabytes
			MaxAge:     MaxAge,  // days
			MaxBackups: MaxBackups,
		}
	}

	logger := slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{
		AddSource:   true,
		Level:       slog.LevelDebug,
		ReplaceAttr: nil,
	}))

	slog.SetDefault(logger)
}

//nolint:funlen
func run() error {
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(fmt.Errorf("nx-server main run ended"))

	ctx, _ = signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGKILL)

	slog.Info(buildinfo.String("nx-daemon"))

	agentConfig, err := configlib.Load()
	if err != nil {
		slog.Warn(fmt.Sprintf("error loading userconfig: %s", err.Error()))
	}

	networkAllocator, err := networkallocator.New(agentConfig.IFace, agentConfig.Network)
	if err != nil {
		return fmt.Errorf("error creating network allocator: %w", err)
	}

	_ = networkAllocator.CleanUp("")

	metricsFactory := metrics.NewPromFactory()

	svc := daemon.New(agentConfig, networkAllocator, daemon.WithMetrics(metricsFactory))

	address, err := networkAllocator.GetIP("nx")
	if err != nil {
		return fmt.Errorf("failed to allocate address: %w", err)
	}

	defer func() {
		if err := networkAllocator.ReleaseIP(address); err != nil {
			slog.Warn("error releasing address", "err", err)
		}
	}()

	aCa := caroot.New()

	if err = aCa.Init(".", nil); err != nil {
		return fmt.Errorf("failed to init CA: %w", err)
	}

	aWebserver := webserver.New(svc)

	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		if err = aWebserver.Run(ctx, "nx", address, "443", aCa); err != nil {
			if errors.Is(http.ErrServerClosed, err) {
				return nil
			}

			return fmt.Errorf("failed to serve: %w", err)
		}

		return nil
	})

	group.Go(func() error {
		if err = metricsFactory.Start(ctx, ":50001"); err != nil {
			return fmt.Errorf("error starting metrics factory: %w", err)
		}

		return nil
	})

	if err = group.Wait(); err != nil {
		return fmt.Errorf("error processing group: %w", err)
	}

	return nil
}

func main() {
	setupLog()

	if err := run(); err != nil {
		panic(err)
	}
}
