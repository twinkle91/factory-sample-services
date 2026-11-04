// Package migrate applies plain SQL migrations to PostgreSQL and records the
// applied versions in the schema_migrations table.
package migrate

import (
	"context"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Migration is one SQL file, such as 001_create_links.sql.
type Migration struct {
	Version int
	Name    string
	SQL     string
}

var fileName = regexp.MustCompile(`^(\d+)_([a-z0-9_]+)\.sql$`)

const createTable = `CREATE TABLE IF NOT EXISTS schema_migrations (
    version    integer     PRIMARY KEY,
    name       text        NOT NULL,
    applied_at timestamptz NOT NULL DEFAULT now()
)`

// Load reads every .sql file at the root of fsys, sorted by version.
func Load(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	seen := map[int]string{}
	var out []Migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		m := fileName.FindStringSubmatch(e.Name())
		if m == nil {
			return nil, fmt.Errorf("migration %q: name must look like 001_description.sql", e.Name())
		}
		version, err := strconv.Atoi(m[1])
		if err != nil || version == 0 {
			return nil, fmt.Errorf("migration %q: version must be a positive number", e.Name())
		}
		if other, ok := seen[version]; ok {
			return nil, fmt.Errorf("migrations %q and %q share version %d", other, e.Name(), version)
		}
		seen[version] = e.Name()
		body, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", e.Name(), err)
		}
		out = append(out, Migration{Version: version, Name: m[2], SQL: string(body)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// Up applies the migrations in fsys that are not yet recorded, each in its own
// transaction, and returns the versions it applied. An advisory lock keeps two
// servers starting at once from applying the same migration.
func Up(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS) ([]int, error) {
	migrations, err := Load(fsys)
	if err != nil {
		return nil, err
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock(hashtext(current_schema()))"); err != nil {
		return nil, fmt.Errorf("lock migrations: %w", err)
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock(hashtext(current_schema()))")

	if _, err := conn.Exec(ctx, createTable); err != nil {
		return nil, fmt.Errorf("create schema_migrations: %w", err)
	}
	rows, err := conn.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("read applied migrations: %w", err)
	}
	versions, err := pgx.CollectRows(rows, pgx.RowTo[int])
	if err != nil {
		return nil, fmt.Errorf("read applied migrations: %w", err)
	}
	applied := map[int]bool{}
	for _, v := range versions {
		applied[v] = true
	}

	var done []int
	for _, m := range migrations {
		if applied[m.Version] {
			continue
		}
		err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, m.SQL); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version, name) VALUES ($1, $2)", m.Version, m.Name)
			return err
		})
		if err != nil {
			return done, fmt.Errorf("apply migration %03d_%s: %w", m.Version, m.Name, err)
		}
		done = append(done, m.Version)
	}
	return done, nil
}
