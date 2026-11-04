package links

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"factory-sample-services/internal/cache"
	"factory-sample-services/internal/migrate"
	"factory-sample-services/internal/testsupport"
	"factory-sample-services/migrations"
)

// newIntegrationStore returns a PostgresStore on a fresh, migrated schema.
func newIntegrationStore(t *testing.T) *PostgresStore {
	t.Helper()
	pool := testsupport.Postgres(t)
	if _, err := migrate.Up(context.Background(), pool, migrations.FS); err != nil {
		t.Fatal(err)
	}
	return NewPostgresStore(pool)
}

func TestIntegrationStore(t *testing.T) {
	store := newIntegrationStore(t)
	ctx := context.Background()

	created, err := store.Create(ctx, "abc", "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if created.CreatedAt.IsZero() {
		t.Fatal("created_at not set")
	}
	if _, err := store.Create(ctx, "abc", "https://other.example"); !errors.Is(err, ErrCodeTaken) {
		t.Fatalf("duplicate code: err = %v, want ErrCodeTaken", err)
	}
	got, err := store.Get(ctx, "abc")
	if err != nil || got.URL != "https://example.com" {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if _, err := store.Get(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(missing) err = %v, want ErrNotFound", err)
	}
}

func TestIntegrationStoreListsNewestFirst(t *testing.T) {
	store := newIntegrationStore(t)
	ctx := context.Background()
	for i := range 5 {
		if _, err := store.Create(ctx, fmt.Sprintf("c%d", i), "https://example.com"); err != nil {
			t.Fatal(err)
		}
	}
	got, err := store.List(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	var codes []string
	for _, l := range got {
		codes = append(codes, l.Code)
	}
	if fmt.Sprint(codes) != "[c4 c3 c2]" {
		t.Fatalf("List(3) = %v, want [c4 c3 c2]", codes)
	}
}

// TestIntegrationAPI runs the HTTP API against the real PostgreSQL and Redis.
func TestIntegrationAPI(t *testing.T) {
	store := newIntegrationStore(t)
	client, prefix := testsupport.Redis(t)
	h := NewAPI(store, cache.NewVisits(client, prefix)).Routes()

	if code, body := do(t, h, "GET", "/health", ""); code != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("health: %d %v", code, body)
	}
	code, body := do(t, h, "POST", "/links", `{"url":"https://go.dev"}`)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %v", code, body)
	}
	short := body["code"].(string)

	for want := 1.0; want <= 3; want++ {
		if code, body := do(t, h, "GET", "/links/"+short, ""); code != 200 || body["visits"] != want {
			t.Fatalf("visit %v: %d %v", want, code, body)
		}
	}
	if code, body := do(t, h, "GET", "/links/"+short+"/stats", ""); code != 200 || body["visits"] != 3.0 {
		t.Fatalf("stats: %d %v", code, body)
	}

	do(t, h, "POST", "/links", `{"url":"https://pkg.go.dev"}`)
	_, body = do(t, h, "GET", "/links?limit=1", "")
	list := body["links"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["url"] != "https://pkg.go.dev" {
		t.Fatalf("list newest first: %v", list)
	}
}
