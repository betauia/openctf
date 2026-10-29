// Command server is the OpenCTF API server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/betauia/openctf/backend/internal/config"
	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/httpserver"
	"github.com/betauia/openctf/backend/internal/instances"
)

const (
	refreshTokenCleanupInterval = time.Hour
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		return err
	}
	provider, err := instances.NewProvider(cfg.InstanceProvider, cfg.HostDomain)
	if err != nil {
		return err
	}

	store := database.NewStore(pool)
	handler, sb := httpserver.New(store, cfg, provider)
	go sb.Run(ctx)
	go deleteExpiredRefreshTokens(ctx, store)

	srv := httpserver.NewServer(cfg.Addr, handler)
	// this goroutine will wait for the server process is killed (SIGTERM/INT) and will shutdown the server gracefully
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	slog.Info("listening", "addr", cfg.Addr)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// deleteExpiredRefreshTokens prunes expired refresh tokens every hour until ctx is done.
func deleteExpiredRefreshTokens(ctx context.Context, store *database.Store) {
	t := time.NewTicker(refreshTokenCleanupInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := store.DeleteExpiredRefreshTokens(ctx); err != nil {
				slog.Error("delete expired refresh tokens", "err", err)
			}
		}
	}
}
