// Package testsupport connects integration tests to the real PostgreSQL and
// Redis named by DATABASE_URL and REDIS_URL.
//
// Each test gets its own PostgreSQL schema and its own Redis key prefix, both
// removed when the test ends, so the suite can run repeatedly and in parallel.
// A missing variable fails the test instead of skipping it, so a missing
// service is visible.
package testsupport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const setupHint = "Integration tests need real services: run `docker compose up -d`, then export " +
	"DATABASE_URL=postgres://factory:factory@localhost:5432/app?sslmode=disable " +
	"REDIS_URL=redis://localhost:6379/0"

// Postgres returns a pool whose search_path is a fresh, empty schema.
func Postgres(t testing.TB) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Fatal("DATABASE_URL is not set. " + setupHint)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect to PostgreSQL from DATABASE_URL: %v. %s", err, setupHint)
	}
	defer admin.Close(ctx)

	schema := "test_" + suffix(t)
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema %s: %v", schema, err)
	}

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("parse DATABASE_URL: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		conn, err := pgx.Connect(ctx, url)
		if err != nil {
			t.Errorf("drop schema %s: %v", schema, err)
			return
		}
		defer conn.Close(ctx)
		if _, err := conn.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("drop schema %s: %v", schema, err)
		}
	})
	return pool
}

// Redis returns a client and a key prefix that belongs to this test alone.
func Redis(t testing.TB) (*redis.Client, string) {
	t.Helper()
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Fatal("REDIS_URL is not set. " + setupHint)
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse REDIS_URL: %v", err)
	}
	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		t.Fatalf("connect to Redis from REDIS_URL: %v. %s", err, setupHint)
	}

	prefix := "test:" + suffix(t) + ":"
	t.Cleanup(func() {
		defer client.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		iter := client.Scan(ctx, 0, prefix+"*", 100).Iterator()
		for iter.Next(ctx) {
			client.Del(ctx, iter.Val())
		}
		if err := iter.Err(); err != nil {
			t.Errorf("clean Redis keys %s*: %v", prefix, err)
		}
	})
	return client, prefix
}

func suffix(t testing.TB) string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}
