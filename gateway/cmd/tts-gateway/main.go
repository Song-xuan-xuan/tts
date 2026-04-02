package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/songtf/tts-stack/gateway/internal/config"
	"github.com/songtf/tts-stack/gateway/internal/httpapi"
	"github.com/songtf/tts-stack/gateway/internal/upstream"
)

func main() {
	os.Exit(run())
}

func run() int {
	configPath := flag.String("config", "gateway.yaml", "Path to gateway config file")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	store, err := config.NewStore(*configPath)
	if err != nil {
		logger.Error("load config", "error", err, "path", *configPath)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := config.Watch(ctx, logger, store, *configPath); err != nil {
		logger.Error("watch config", "error", err, "path", *configPath)
		return 1
	}

	current := store.Current()
	if current == nil {
		logger.Error("config unavailable after load")
		return 1
	}

	client := upstream.New(current.Upstream.BaseURL, current.Upstream.TimeoutSeconds)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", current.Server.Port),
		Handler: httpapi.New(store, client),
	}

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	logger.Info("gateway listening", "addr", server.Addr)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("http server failed", "error", err)
		return 1
	}

	return 0
}
