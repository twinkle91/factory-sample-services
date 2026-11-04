package links

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// memStore and memCounter stand in for PostgreSQL and Redis in unit tests.
type memStore struct {
	mu    sync.Mutex
	links map[string]Link
	clock time.Time
	down  error
}

func newMemStore() *memStore {
	return &memStore{links: map[string]Link{}, clock: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func (s *memStore) Create(_ context.Context, code, url string) (Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.links[code]; ok {
		return Link{}, ErrCodeTaken
	}
	s.clock = s.clock.Add(time.Second)
	l := Link{Code: code, URL: url, CreatedAt: s.clock}
	s.links[code] = l
	return l, nil
}

func (s *memStore) Get(_ context.Context, code string) (Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.links[code]
	if !ok {
		return Link{}, ErrNotFound
	}
	return l, nil
}

func (s *memStore) List(_ context.Context, limit int) ([]Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Link
	for _, l := range s.links {
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *memStore) Ping(context.Context) error { return s.down }

type memCounter struct {
	mu     sync.Mutex
	counts map[string]int64
	down   error
}

func (c *memCounter) Incr(_ context.Context, code string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counts[code]++
	return c.counts[code], nil
}

func (c *memCounter) Count(_ context.Context, code string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts[code], nil
}

func (c *memCounter) Ping(context.Context) error { return c.down }

func newTestAPI() (*API, *memStore, *memCounter) {
	store := newMemStore()
	counter := &memCounter{counts: map[string]int64{}}
	api := NewAPI(store, counter)
	n := 0
	api.newCode = func() (string, error) {
		n++
		return fmt.Sprintf("code%03d", n), nil
	}
	return api, store, counter
}

func do(t *testing.T, h http.Handler, method, path, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("%s %s: response is not JSON: %q", method, path, rec.Body.String())
	}
	return rec.Code, out
}

func TestHealth(t *testing.T) {
	api, store, counter := newTestAPI()
	h := api.Routes()

	if code, body := do(t, h, "GET", "/health", ""); code != 200 || body["status"] != "ok" {
		t.Fatalf("healthy: %d %v", code, body)
	}
	counter.down = errors.New("connection refused")
	code, body := do(t, h, "GET", "/health", "")
	if code != 503 || body["status"] != "unavailable" {
		t.Fatalf("redis down: %d %v", code, body)
	}
	checks := body["checks"].(map[string]any)
	if checks["redis"] != "connection refused" || checks["postgres"] != "ok" {
		t.Fatalf("redis down checks: %v", checks)
	}
	counter.down, store.down = nil, errors.New("timeout")
	if code, _ := do(t, h, "GET", "/health", ""); code != 503 {
		t.Fatalf("postgres down: %d", code)
	}
}

func TestCreateLink(t *testing.T) {
	api, _, _ := newTestAPI()
	code, body := do(t, api.Routes(), "POST", "/links", `{"url":"  https://example.com/a  "}`)
	if code != 201 {
		t.Fatalf("status %d %v", code, body)
	}
	if body["code"] != "code001" || body["url"] != "https://example.com/a" {
		t.Fatalf("body %v", body)
	}
}

func TestCreateLinkRetriesTakenCodes(t *testing.T) {
	api, store, _ := newTestAPI()
	store.Create(context.Background(), "code001", "https://taken.example")
	code, body := do(t, api.Routes(), "POST", "/links", `{"url":"https://example.com"}`)
	if code != 201 || body["code"] != "code002" {
		t.Fatalf("%d %v", code, body)
	}
}

func TestCreateLinkRejectsBadInput(t *testing.T) {
	api, _, _ := newTestAPI()
	tests := map[string]struct{ body, want string }{
		"not json":     {`url=https://example.com`, "body must be JSON"},
		"missing url":  {`{}`, "url is required"},
		"no scheme":    {`{"url":"example.com"}`, "http:// or https://"},
		"ftp":          {`{"url":"ftp://example.com/file"}`, "http:// or https://"},
		"no host":      {`{"url":"https://"}`, "must include a host"},
		"too long":     {`{"url":"https://example.com/` + strings.Repeat("a", 2048) + `"}`, "at most 2048"},
		"invalid":      {`{"url":"https://exa mple.com/%zz"}`, "not a valid URL"},
		"javascript":   {`{"url":"javascript:alert(1)"}`, "http:// or https://"},
		"empty string": {`{"url":"   "}`, "url is required"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			code, body := do(t, api.Routes(), "POST", "/links", tt.body)
			if code != 400 || !strings.Contains(fmt.Sprint(body["error"]), tt.want) {
				t.Fatalf("%d %v, want 400 mentioning %q", code, body, tt.want)
			}
		})
	}
}

func TestGetLinkCountsVisits(t *testing.T) {
	api, _, _ := newTestAPI()
	h := api.Routes()
	do(t, h, "POST", "/links", `{"url":"https://example.com"}`)

	for want := 1.0; want <= 2; want++ {
		code, body := do(t, h, "GET", "/links/code001", "")
		if code != 200 || body["url"] != "https://example.com" || body["visits"] != want {
			t.Fatalf("visit %v: %d %v", want, code, body)
		}
	}
	code, body := do(t, h, "GET", "/links/code001/stats", "")
	if code != 200 || body["visits"] != 2.0 || body["code"] != "code001" {
		t.Fatalf("stats: %d %v", code, body)
	}
}

func TestUnknownCodeIsNotFound(t *testing.T) {
	api, _, counter := newTestAPI()
	h := api.Routes()
	for _, path := range []string{"/links/nope", "/links/nope/stats"} {
		if code, _ := do(t, h, "GET", path, ""); code != 404 {
			t.Fatalf("%s: status %d, want 404", path, code)
		}
	}
	if len(counter.counts) != 0 {
		t.Fatalf("unknown code was counted: %v", counter.counts)
	}
}

func TestListLinks(t *testing.T) {
	api, _, _ := newTestAPI()
	h := api.Routes()

	if code, body := do(t, h, "GET", "/links", ""); code != 200 || len(body["links"].([]any)) != 0 {
		t.Fatalf("empty list: %d %v", code, body)
	}
	for i := range 25 {
		do(t, h, "POST", "/links", fmt.Sprintf(`{"url":"https://example.com/%d"}`, i))
	}

	_, body := do(t, h, "GET", "/links", "")
	if n := len(body["links"].([]any)); n != DefaultLimit {
		t.Fatalf("default limit returned %d links, want %d", n, DefaultLimit)
	}
	_, body = do(t, h, "GET", "/links?limit=2", "")
	got := body["links"].([]any)
	if len(got) != 2 || got[0].(map[string]any)["code"] != "code025" {
		t.Fatalf("limit=2 should return the two newest: %v", got)
	}
	for _, bad := range []string{"0", "-3", "ten"} {
		if code, _ := do(t, h, "GET", "/links?limit="+bad, ""); code != 400 {
			t.Fatalf("limit=%s: status %d, want 400", bad, code)
		}
	}
}

func TestNewCode(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		c, err := NewCode()
		if err != nil {
			t.Fatal(err)
		}
		if len(c) != codeLength || strings.Trim(c, codeAlphabet) != "" {
			t.Fatalf("bad code %q", c)
		}
		seen[c] = true
	}
	if len(seen) < 99 {
		t.Fatalf("codes repeat too often: %d unique of 100", len(seen))
	}
}
