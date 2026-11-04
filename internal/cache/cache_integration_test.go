package cache_test

import (
	"context"
	"testing"

	"factory-sample-services/internal/cache"
	"factory-sample-services/internal/testsupport"
)

func TestIntegrationVisits(t *testing.T) {
	client, prefix := testsupport.Redis(t)
	visits := cache.NewVisits(client, prefix)
	ctx := context.Background()

	if n, err := visits.Count(ctx, "never"); err != nil || n != 0 {
		t.Fatalf("Count(never) = %d, %v; want 0, nil", n, err)
	}
	for want := int64(1); want <= 3; want++ {
		got, err := visits.Incr(ctx, "abc")
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("Incr = %d, want %d", got, want)
		}
	}
	if n, err := visits.Count(ctx, "abc"); err != nil || n != 3 {
		t.Fatalf("Count(abc) = %d, %v; want 3, nil", n, err)
	}
	if err := client.Get(ctx, prefix+"visits:abc").Err(); err != nil {
		t.Fatalf("counter not stored under the prefix: %v", err)
	}
}
