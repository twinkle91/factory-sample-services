package main

import (
	"strings"
	"testing"
)

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestLoadConfigRequiresServices(t *testing.T) {
	_, err := loadConfig(env(nil))
	if err == nil {
		t.Fatal("want an error without DATABASE_URL and REDIS_URL")
	}
	for _, name := range []string{"DATABASE_URL", "REDIS_URL"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s", err, name)
		}
	}
	_, err = loadConfig(env(map[string]string{"DATABASE_URL": "postgres://x"}))
	if err == nil || strings.Contains(err.Error(), "DATABASE_URL") || !strings.Contains(err.Error(), "REDIS_URL") {
		t.Fatalf("only REDIS_URL missing: err = %v", err)
	}
}

func TestLoadConfigPort(t *testing.T) {
	base := map[string]string{"DATABASE_URL": "postgres://x", "REDIS_URL": "redis://x"}
	c, err := loadConfig(env(base))
	if err != nil {
		t.Fatal(err)
	}
	if c.addr() != "0.0.0.0:8080" {
		t.Fatalf("default addr = %s", c.addr())
	}

	base["PORT"] = "9090"
	if c, _ := loadConfig(env(base)); c.addr() != "0.0.0.0:9090" {
		t.Fatalf("PORT=9090 addr = %s", c.addr())
	}
	base["PORT"] = "http"
	if _, err := loadConfig(env(base)); err == nil {
		t.Fatal("want an error for PORT=http")
	}
}
