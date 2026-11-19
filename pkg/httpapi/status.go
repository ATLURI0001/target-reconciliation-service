package httpapi

import "time"

// ComponentStatus represents the health of a subsystem.
type ComponentStatus struct {
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
	Latency time.Duration `json:"latency"`
}

// SystemHealthResponse formats the global system readiness report.
type SystemHealthResponse struct {
	Status     string            `json:"status"`
	Components []ComponentStatus `json:"components"`
	Timestamp  time.Time         `json:"timestamp"`
}

// AllHealthy returns true only if every reported component is operational.
func (s SystemHealthResponse) AllHealthy() bool {
	if len(s.Components) == 0 {
		return false
	}
	for _, c := range s.Components {
		if !c.Healthy {
			return false
		}
	}
	return true
}
