package k8s

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type Probe struct {
	addr  string
	ready bool
}

const (
	TimeoutGraceful   = time.Second * 5
	ReadHeaderTimeout = time.Second * 5
)

func (k *Probe) Ready() {
	k.ready = true
}

func (k *Probe) Run(ctx context.Context) error {
	if ctx.Err() != nil {
		return fmt.Errorf("context closed calling Run: %w", ctx.Err())
	}

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(fmt.Errorf("deferred"))

	mux := http.NewServeMux()
	mux.HandleFunc("/live", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("ok"))
	})

	mux.HandleFunc("/ready", func(writer http.ResponseWriter, request *http.Request) {
		if !k.ready {
			slog.Warn("k8sprobe: Not ready")
			http.Error(writer, "not ready yet", http.StatusServiceUnavailable)

			return
		}
		_, _ = writer.Write([]byte("ok"))
	})

	slog.Info(fmt.Sprintf("Starting k8sprobes on %s", k.addr))
	server := http.Server{
		Addr:              k.addr,
		Handler:           mux,
		ReadHeaderTimeout: ReadHeaderTimeout,
	}

	go func() {
		<-ctx.Done()

		ctx, cancel := context.WithTimeout(context.Background(), TimeoutGraceful)
		defer cancel()

		err := server.Shutdown(ctx) //nolint:contextcheck
		if err != nil {
			slog.Warn(fmt.Sprintf("Errorf closing probe server: %s", err.Error()))
		}
	}()

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("error serving http: %w", err)
	}

	return nil
}

func NewProbe(addr string) *Probe {
	return &Probe{
		addr: addr,
	}
}
