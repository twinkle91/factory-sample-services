package links

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"
)

// Store is where links live; PostgresStore in production.
type Store interface {
	Create(ctx context.Context, code, url string) (Link, error)
	Get(ctx context.Context, code string) (Link, error)
	List(ctx context.Context, limit int) ([]Link, error)
	Ping(ctx context.Context) error
}

// Counter counts visits per code; cache.Visits in production.
type Counter interface {
	Incr(ctx context.Context, code string) (int64, error)
	Count(ctx context.Context, code string) (int64, error)
	Ping(ctx context.Context) error
}

type API struct {
	store   Store
	visits  Counter
	newCode func() (string, error)
}

func NewAPI(store Store, visits Counter) *API {
	return &API{store: store, visits: visits, newCode: NewCode}
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", a.health)
	mux.HandleFunc("POST /links", a.createLink)
	mux.HandleFunc("GET /links", a.listLinks)
	mux.HandleFunc("GET /links/{code}", a.getLink)
	mux.HandleFunc("GET /links/{code}/stats", a.linkStats)
	return mux
}

type visitedLink struct {
	Link
	Visits int64 `json:"visits"`
}

type stats struct {
	Code   string `json:"code"`
	Visits int64  `json:"visits"`
}

// health answers {"status":"ok"} only when PostgreSQL and Redis both respond.
func (a *API) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	checks := map[string]string{"postgres": "ok", "redis": "ok"}
	healthy := true
	if err := a.store.Ping(ctx); err != nil {
		checks["postgres"], healthy = err.Error(), false
	}
	if err := a.visits.Ping(ctx); err != nil {
		checks["redis"], healthy = err.Error(), false
	}
	if !healthy {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "unavailable", "checks": checks})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) createLink(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "body must be JSON like {\"url\": \"https://example.com\"}")
		return
	}
	target, err := validateURL(body.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	for range 5 {
		code, err := a.newCode()
		if err != nil {
			a.serverError(w, err)
			return
		}
		link, err := a.store.Create(r.Context(), code, target)
		if errors.Is(err, ErrCodeTaken) {
			continue
		}
		if err != nil {
			a.serverError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, link)
		return
	}
	a.serverError(w, errors.New("could not find a free short code"))
}

// listLinks returns the newest links, ?limit= of them (default DefaultLimit, at most MaxLimit).
func (a *API) listLinks(w http.ResponseWriter, r *http.Request) {
	limit, err := parseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	links, err := a.store.List(r.Context(), limit)
	if err != nil {
		a.serverError(w, err)
		return
	}
	if links == nil {
		links = []Link{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"links": links})
}

// getLink returns the link and counts the visit.
func (a *API) getLink(w http.ResponseWriter, r *http.Request) {
	link, ok := a.lookup(w, r)
	if !ok {
		return
	}
	visits, err := a.visits.Incr(r.Context(), link.Code)
	if err != nil {
		a.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, visitedLink{Link: link, Visits: visits})
}

func (a *API) linkStats(w http.ResponseWriter, r *http.Request) {
	link, ok := a.lookup(w, r)
	if !ok {
		return
	}
	visits, err := a.visits.Count(r.Context(), link.Code)
	if err != nil {
		a.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats{Code: link.Code, Visits: visits})
}

func (a *API) lookup(w http.ResponseWriter, r *http.Request) (Link, bool) {
	link, err := a.store.Get(r.Context(), r.PathValue("code"))
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "link not found")
		return Link{}, false
	}
	if err != nil {
		a.serverError(w, err)
		return Link{}, false
	}
	return link, true
}

func (a *API) serverError(w http.ResponseWriter, err error) {
	log.Printf("internal error: %v", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
