// Command server runs the short-links API.
//
//	DATABASE_URL=postgres://factory:factory@localhost:5432/app?sslmode=disable \
//	REDIS_URL=redis://localhost:6379/0 \
//	go run ./cmd/server
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"factory-sample-services/internal/cache"
	"factory-sample-services/internal/links"
	"factory-sample-services/internal/migrate"
	"factory-sample-services/migrations"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func run() error {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(startCtx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(startCtx); err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	applied, err := migrate.Up(startCtx, pool, migrations.FS)
	if err != nil {
		return err
	}
	log.Printf("migrations: applied %v", applied)

	redisClient, err := cache.Connect(startCtx, cfg.RedisURL)
	if err != nil {
		return err
	}
	defer redisClient.Close()

	api := links.NewAPI(links.NewPostgresStore(pool), cache.NewVisits(redisClient, cache.DefaultPrefix))
	srv := &http.Server{
		Addr:              cfg.addr(),
		Handler:           api.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", srv.Addr)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Print("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
