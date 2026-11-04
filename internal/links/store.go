// Package links serves the short-link API and stores links in PostgreSQL.
package links

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound  = errors.New("link not found")
	ErrCodeTaken = errors.New("short code already taken")
)

type Link struct {
	Code      string    `json:"code"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

// PostgresStore keeps links in the links table (see migrations/).
type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) Create(ctx context.Context, code, url string) (Link, error) {
	var l Link
	err := s.pool.QueryRow(ctx,
		`INSERT INTO links (code, url) VALUES ($1, $2) RETURNING code, url, created_at`,
		code, url,
	).Scan(&l.Code, &l.URL, &l.CreatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return Link{}, ErrCodeTaken
	}
	return l, err
}

func (s *PostgresStore) Get(ctx context.Context, code string) (Link, error) {
	var l Link
	err := s.pool.QueryRow(ctx,
		`SELECT code, url, created_at FROM links WHERE code = $1`, code,
	).Scan(&l.Code, &l.URL, &l.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Link{}, ErrNotFound
	}
	return l, err
}

// List returns up to limit links, newest first.
func (s *PostgresStore) List(ctx context.Context, limit int) ([]Link, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT code, url, created_at FROM links ORDER BY created_at DESC, id DESC LIMIT $1`, limit,
	)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Link, error) {
		var l Link
		err := row.Scan(&l.Code, &l.URL, &l.CreatedAt)
		return l, err
	})
}

func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}
