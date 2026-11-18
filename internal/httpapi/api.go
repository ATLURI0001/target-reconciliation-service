package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ATLURI0001/target-reconciliation-service/internal/core"
)

type API struct {
	Service                    *core.Service
	OperatorToken, ViewerToken string
	requests                   atomic.Uint64
	failures                   atomic.Uint64
	slots                      chan struct{}
	mu                         sync.Mutex
	tokens                     float64
	last                       time.Time
}

func New(s *core.Service, operator, viewer string) (*API, error) {
	if len(operator) < 32 || len(viewer) < 32 || operator == viewer {
		return nil, fmt.Errorf("distinct operator and viewer tokens of at least 32 bytes required")
	}
	return &API{Service: s, OperatorToken: operator, ViewerToken: viewer, slots: make(chan struct{}, 32), tokens: 100, last: time.Now()}, nil
}
func equal(a, b string) bool {
	x := sha256.Sum256([]byte(a))
	y := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(x[:], y[:]) == 1
}
func (a *API) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200); _, _ = w.Write([]byte("ok\n")) })
	m.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		_, err := a.Service.Store.View(ctx)
		if err != nil {
			http.Error(w, "store unavailable", 503)
			return
		}
		_, _ = w.Write([]byte("ready\n"))
	})
	m.HandleFunc("GET /metrics", a.metrics)
	m.HandleFunc("GET /v1/audit", a.auth(func(w http.ResponseWriter, r *http.Request, who core.Identity) {
		cursorRaw := strings.TrimSpace(r.URL.Query().Get("cursor"))
		cursor := 0
		var err error
		if cursorRaw != "" {
			cursor, err = strconv.Atoi(cursorRaw)
		}
		if err != nil || cursor < 0 {
			http.Error(w, "invalid cursor", 400)
			return
		}
		limit := 100
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			limit, err = strconv.Atoi(raw)
		}
		if err != nil || limit < 1 || limit > 200 {
			http.Error(w, "limit must be 1..200", 400)
			return
		}
		s, err := a.Service.Store.View(r.Context())
		if err != nil {
			a.result(w, nil, err)
			return
		}
		items := []core.Audit{}
		next := cursor
		for _, event := range s.Audit {
			if event.Sequence > cursor {
				items = append(items, event)
				next = event.Sequence
				if len(items) == limit {
					break
				}
			}
		}
		a.result(w, map[string]any{"items": items, "nextCursor": next}, nil)
	}))

	m.HandleFunc("GET /v1/snapshot", a.auth(func(w http.ResponseWriter, r *http.Request, who core.Identity) {
		s, e := a.Service.Store.View(r.Context())
		if e != nil {
			a.result(w, nil, e)
			return
		}
		now := a.Service.Now()
		for k, c := range s.Clusters {
			c.Connected = !c.LastSeen.IsZero() && now.Sub(c.LastSeen) <= time.Duration(s.Policy.FreshnessSeconds)*time.Second
			s.Clusters[k] = c
		}
		if len(s.Audit) > 200 {
			s.Audit = s.Audit[len(s.Audit)-200:]
		}
		a.result(w, s, nil)
	}))
	m.HandleFunc("POST /v1/clusters", a.auth(func(w http.ResponseWriter, r *http.Request, who core.Identity) {
		var c core.Cluster
		if !decode(w, r, &c) {
			return
		}
		a.result(w, map[string]string{"result": "registered"}, a.Service.Register(r.Context(), who, c))
	}))
	m.HandleFunc("PUT /v1/policy", a.auth(func(w http.ResponseWriter, r *http.Request, who core.Identity) {
		var p core.Policy
		if !decode(w, r, &p) {
			return
		}
		a.result(w, map[string]string{"result": "updated"}, a.Service.SetPolicy(r.Context(), who, p))
	}))
	m.HandleFunc("POST /v1/incidents/{id}/approve", a.auth(func(w http.ResponseWriter, r *http.Request, who core.Identity) {
		id := strings.TrimSpace(r.PathValue("id"))
		if id == "" {
			http.Error(w, "incident id required", 400)
			return
		}
		a.result(w, map[string]string{"result": "evaluated"}, a.Service.Approve(r.Context(), who, id))
	}))
	m.HandleFunc("POST /v1/incidents/{id}/cancel", a.auth(func(w http.ResponseWriter, r *http.Request, who core.Identity) {
		id := strings.TrimSpace(r.PathValue("id"))
		if id == "" {
			http.Error(w, "incident id required", 400)
			return
		}
		a.result(w, map[string]string{"result": "cancelled"}, a.Service.Cancel(r.Context(), who, id))
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.requests.Add(1)
		tracked := &statusWriter{ResponseWriter: w, status: 200}
		w = tracked
		defer func() {
			if tracked.status >= 400 {
				a.failures.Add(1)
			}
		}()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		a.mu.Lock()
		now := time.Now()
		a.tokens += now.Sub(a.last).Seconds() * 20
		if a.tokens > 100 {
			a.tokens = 100
		}
		a.last = now
		allow := a.tokens >= 1
		if allow {
			a.tokens--
		}
		a.mu.Unlock()
		if !allow {
			http.Error(w, "rate limit", 429)
			return
		}
		select {
		case a.slots <- struct{}{}:
			defer func() { <-a.slots }()
		default:
			http.Error(w, "busy", 503)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		m.ServeHTTP(w, r.WithContext(ctx))
	})
}
func (a *API) auth(next func(http.ResponseWriter, *http.Request, core.Identity)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, "unauthorized", 401)
			return
		}
		token := parts[1]
		who := core.Identity{}
		if equal(token, a.OperatorToken) {
			who = core.Identity{Subject: "local-operator", Role: "operator"}
		} else if equal(token, a.ViewerToken) {
			who = core.Identity{Subject: "local-viewer", Role: "viewer"}
		} else {
			http.Error(w, "unauthorized", 401)
			return
		}
		next(w, r, who)
	}
}
// TotalRequests returns the total number of HTTP requests handled by the API.
func (a *API) TotalRequests() uint64 {
	return a.requests.Load()
}

// TotalFailures returns the total number of requests that resulted in 4xx or 5xx responses.
func (a *API) TotalFailures() uint64 {
	return a.failures.Load()
}

func decode(w http.ResponseWriter, r *http.Request, out any) bool {
	if ct := r.Header.Get("Content-Type"); ct != "" {
		mediaType := strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
		if mediaType != "application/json" {
			http.Error(w, "application/json required", http.StatusUnsupportedMediaType)
			return false
		}
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		http.Error(w, "invalid JSON", 400)
		return false
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		http.Error(w, "single JSON value required", 400)
		return false
	}
	return true
}
func (a *API) result(w http.ResponseWriter, v any, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		status := 500
		msg := "internal operation failed"
		switch {
		case errors.Is(err, core.ErrInvalid):
			status = 400
			msg = err.Error()
		case errors.Is(err, core.ErrDenied):
			status = 403
			msg = err.Error()
		case errors.Is(err, core.ErrNotFound):
			status = 404
			msg = err.Error()
		case errors.Is(err, core.ErrConflict):
			status = 409
			msg = err.Error()
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

// metrics expose observed counters and Kubernetes replica gauges, never synthetic utilization.
func (a *API) metrics(w http.ResponseWriter, r *http.Request) {
	s, e := a.Service.Store.View(r.Context())
	if e != nil {
		http.Error(w, "metrics store unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# TYPE trs_http_requests_total counter\ntrs_http_requests_total %d\n# TYPE trs_http_failures_total counter\ntrs_http_failures_total %d\n", a.requests.Load(), a.failures.Load())
	fmt.Fprintln(w, "# TYPE trs_cluster_last_seen_seconds gauge")
	for _, c := range s.Clusters {
		ts := float64(0)
		if !c.LastSeen.IsZero() {
			ts = float64(c.LastSeen.Unix())
		}
		fmt.Fprintf(w, "trs_cluster_last_seen_seconds{cluster=%s} %g\n", strconv.Quote(c.ID), ts)
	}
	fmt.Fprintln(w, "# TYPE trs_deployment_available_replicas gauge\n# TYPE trs_deployment_desired_replicas gauge")
	for _, o := range s.Observations {
		labels := fmt.Sprintf("cluster=%s,namespace=%s,deployment=%s", strconv.Quote(o.Cluster), strconv.Quote(o.Namespace), strconv.Quote(o.Name))
		fmt.Fprintf(w, "trs_deployment_available_replicas{%s} %d\ntrs_deployment_desired_replicas{%s} %d\n", labels, o.Available, labels, o.Desired)
	}
	fmt.Fprintln(w, "# TYPE trs_incidents gauge")
	counts := map[string]int{}
	for _, i := range s.Incidents {
		counts[i.State]++
	}
	for state, count := range counts {
		fmt.Fprintf(w, "trs_incidents{state=%s} %d\n", strconv.Quote(state), count)
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *statusWriter) WriteHeader(status int) {
	if w.wrote {
		return
	}
	w.wrote = true
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *statusWriter) Write(p []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(p)
}
