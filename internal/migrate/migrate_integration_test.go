package migrate_test

import (
	"context"
	"testing"
	"testing/fstest"

	"factory-sample-services/internal/migrate"
	"factory-sample-services/internal/testsupport"
	"factory-sample-services/migrations"
)

func TestIntegrationUpAppliesEachMigrationOnce(t *testing.T) {
	pool := testsupport.Postgres(t)
	ctx := context.Background()

	first, err := migrate.Up(ctx, pool, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 {
		t.Fatal("first run applied no migrations")
	}
	second, err := migrate.Up(ctx, pool, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 0 {
		t.Fatalf("second run applied %v, want nothing", second)
	}

	var recorded int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM schema_migrations").Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	if recorded != len(first) {
		t.Fatalf("schema_migrations has %d rows, want %d", recorded, len(first))
	}
	if _, err := pool.Exec(ctx, "INSERT INTO links (code, url) VALUES ('abc', 'https://example.com')"); err != nil {
		t.Fatalf("links table missing after migrations: %v", err)
	}
}

func TestIntegrationUpRollsBackAFailedMigration(t *testing.T) {
	pool := testsupport.Postgres(t)
	ctx := context.Background()
	fsys := fstest.MapFS{
		"001_ok.sql":     {Data: []byte("CREATE TABLE ok (id int)")},
		"002_broken.sql": {Data: []byte("CREATE TABLE half (id int); SELECT no_such_function()")},
	}

	applied, err := migrate.Up(ctx, pool, fsys)
	if err == nil {
		t.Fatal("Up succeeded, want an error from 002_broken.sql")
	}
	if len(applied) != 1 || applied[0] != 1 {
		t.Fatalf("applied = %v, want [1]", applied)
	}
	var exists bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('half') IS NOT NULL").Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("table from the failed migration was kept")
	}
}
