package httpapi

import (
	"context"
	"github.com/ATLURI0001/target-reconciliation-service/internal/core"
	"github.com/ATLURI0001/target-reconciliation-service/internal/store"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIAuthenticationAndAuditedAuthorization(t *testing.T) {
	s := core.NewService(store.NewMemory())
	op := strings.Repeat("o", 32)
	view := strings.Repeat("v", 32)
	a, e := New(s, op, view)
	if e != nil {
		t.Fatal(e)
	}
	h := a.Handler()
	for _, tc := range []struct {
		method, path, body, token string
		want                      int
	}{
		{"GET", "/healthz", "", "", 200}, {"GET", "/v1/snapshot", "", "", 401}, {"POST", "/v1/clusters", `{"id":"local","environment":"test"}`, view, 403}, {"POST", "/v1/clusters", `{"id":"local","environment":"test"}`, op, 200}, {"GET", "/v1/snapshot", "", view, 200}, {"POST", "/v1/clusters", `{"id":"bad","environment":"test","role":"operator"}`, op, 400}, {"POST", "/v1/clusters", `{"id":"bad","environment":"test"}garbage`, op, 400},
		{"POST", "/v1/incidents/%20%20/approve", "", op, 400},
		{"POST", "/v1/incidents/%20%20/cancel", "", op, 400},
		{"GET", "/v1/snapshot", "", "bearer " + view, 200},
		{"GET", "/v1/snapshot", "", "Bearer   " + view, 200},
		{"GET", "/v1/audit?cursor=%200%20&limit=%2050%20", "", op, 200},
	} {
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		if tc.token != "" {
			if strings.HasPrefix(strings.ToLower(tc.token), "bearer ") {
				r.Header.Set("Authorization", tc.token)
			} else {
				r.Header.Set("Authorization", "Bearer "+tc.token)
			}
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("%s %s got %d %s", tc.method, tc.path, w.Code, w.Body)
		}
	}
	st, _ := s.Store.View(context.Background())
	if len(st.Clusters) != 1 || len(st.Audit) != 2 || st.Audit[0].Result != "denied" {
		t.Fatal(st)
	}
}
func TestBoundedAPIAndInvalidTokens(t *testing.T) {
	s := core.NewService(store.NewMemory())
	a, _ := New(s, strings.Repeat("o", 32), strings.Repeat("v", 32))
	a.tokens = 0
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
	if w.Code != 429 {
		t.Fatal(w.Code)
	}
	if _, e := New(s, "short", "short"); e == nil {
		t.Fatal("weak tokens")
	}
}
