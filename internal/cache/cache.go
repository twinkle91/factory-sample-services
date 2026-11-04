// Package cache keeps per-link visit counters in Redis.
package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// DefaultPrefix namespaces the server's keys, such as shortlinks:visits:abc123.
const DefaultPrefix = "shortlinks:"

// Connect opens a client for a URL such as redis://redis:6379/0 and checks it responds.
func Connect(ctx context.Context, url string) (*redis.Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_URL: %w", err)
	}
	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("ping Redis: %w", err)
	}
	return client, nil
}

// Visits counts how often each short code was opened.
type Visits struct {
	client *redis.Client
	prefix string
}

func NewVisits(client *redis.Client, prefix string) *Visits {
	return &Visits{client: client, prefix: prefix}
}

func (v *Visits) key(code string) string {
	return v.prefix + "visits:" + code
}

// Incr adds one visit and returns the new total.
func (v *Visits) Incr(ctx context.Context, code string) (int64, error) {
	return v.client.Incr(ctx, v.key(code)).Result()
}

// Count returns the visits so far; a code never visited has zero.
func (v *Visits) Count(ctx context.Context, code string) (int64, error) {
	n, err := v.client.Get(ctx, v.key(code)).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	return n, err
}

func (v *Visits) Ping(ctx context.Context) error {
	return v.client.Ping(ctx).Err()
}
