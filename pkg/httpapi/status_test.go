package httpapi

import (
	"testing"
	"time"
)

func TestSystemHealthResponse_AllHealthy(t *testing.T) {
	resp := SystemHealthResponse{
		Status: "SERVING",
		Components: []ComponentStatus{
			{Name: "postgres", Healthy: true, Latency: 5 * time.Millisecond},
			{Name: "redis", Healthy: true, Latency: 2 * time.Millisecond},
		},
		Timestamp: time.Now(),
	}

	if !resp.AllHealthy() {
		t.Fatal("expected AllHealthy to be true when all components are healthy")
	}

	// Degraded
	resp.Components = append(resp.Components, ComponentStatus{
		Name:    "vault",
		Healthy: false,
		Latency: 500 * time.Millisecond,
	})
	if resp.AllHealthy() {
		t.Fatal("expected AllHealthy to be false when one component is unhealthy")
	}

	// Empty components
	empty := SystemHealthResponse{}
	if empty.AllHealthy() {
		t.Fatal("expected AllHealthy to be false for empty component list")
	}
}
