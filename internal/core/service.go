package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

var clusterID = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
var digest = regexp.MustCompile(`^[^\s]+@sha256:[a-f0-9]{64}$`)

type Service struct {
	Store Store
	Now   func() time.Time
}

func NewService(s Store) *Service { return &Service{Store: s, Now: time.Now} }
func audit(s *State, now time.Time, who, action, resource, result string) {
	s.Audit = append(s.Audit, Audit{Sequence: len(s.Audit) + 1, At: now, Actor: who, Action: action, Resource: resource, Result: result})
}
func id() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func (v *Service) Register(ctx context.Context, who Identity, c Cluster) error {
	var denied bool
	err := v.Store.Update(ctx, func(s *State) error {
		if who.Role != "operator" {
			audit(s, v.Now(), who.Subject, "cluster.register", c.ID, "denied")
			denied = true
			return nil
		}
		if !clusterID.MatchString(c.ID) || len(c.ID) > 63 || c.Environment != "test" {
			return fmt.Errorf("%w: DNS-label cluster id required; only test environment supported", ErrInvalid)
		}
		if _, ok := s.Clusters[c.ID]; !ok && len(s.Clusters) >= 100 {
			return fmt.Errorf("cluster limit reached")
		}
		if old, ok := s.Clusters[c.ID]; ok {
			c.LastSeen = old.LastSeen
			c.Connected = old.Connected
		} else {
			c.LastSeen = time.Time{}
			c.Connected = false
		}
		s.Clusters[c.ID] = c
		audit(s, v.Now(), who.Subject, "cluster.register", c.ID, "registered")
		return nil
	})
	if err != nil {
		return err
	}
	if denied {
		return ErrDenied
	}
	return nil
}
func allowed(s *State, o Observation, now time.Time) string {
	p := s.Policy
	if p.EmergencyStop {
		return "emergency stop enabled"
	}
	c, ok := s.Clusters[o.Cluster]
	if !ok || c.Environment != "test" {
		return "unregistered or non-test cluster"
	}
	if !slices.Contains(p.Namespaces, o.Namespace) || o.Namespace == "kube-system" || o.Namespace == "default" || o.Namespace == "target-reconciliation-service" {
		return "namespace not permitted"
	}
	if !o.OptIn || o.GitOps {
		return "target requires opt-in and exclusive runtime ownership"
	}
	if o.Desired < 1 || o.Desired > p.MaxReplicas {
		return "blast radius exceeds policy"
	}
	if now.Sub(o.At) > time.Duration(p.FreshnessSeconds)*time.Second || o.At.After(now) {
		return "stale evidence"
	}
	if o.UID == "" || o.ResourceVersion == "" || o.Container == "" || !digest.MatchString(o.HealthyImage) {
		return "missing identity or digest-pinned recovery image"
	}
	return ""
}

// Observe is trusted controller input, deliberately not exposed as a public HTTP endpoint.
func (v *Service) Observe(ctx context.Context, o Observation) error {
	now := v.Now().UTC()
	o.At = now
	return v.Store.Update(ctx, func(s *State) error {
		c, ok := s.Clusters[o.Cluster]
		if !ok {
			return ErrNotFound
		}
		c.LastSeen = now
		c.Connected = true
		s.Clusters[o.Cluster] = c
		key := o.Key()
		old := s.Observations[key]
		if _, ok := s.Observations[key]; !ok && len(s.Observations) >= 1000 {
			return fmt.Errorf("observation limit reached")
		}
		s.Observations[key] = o
		for k, i := range s.Incidents {
			if i.Target != key {
				continue
			}
			if i.State == "Executing" || i.State == "Verifying" {
				if !i.Deadline.After(now) {
					i.State = "Failed"
					i.Error = "execution/verification deadline exceeded"
					i.Updated = now
					s.Incidents[k] = i
					audit(s, now, "controller", "incident.timeout", i.ID, i.Error)
					continue
				}
				if o.ExecutionKey == i.ExecutionKey && o.UID == i.Plan.Target.UID && o.Container == i.Plan.Target.Container && o.Desired == i.Plan.Target.Desired && o.Image == i.Plan.Image && o.Healthy() {
					i.State = "Resolved"
					i.Updated = now
					s.Incidents[k] = i
					audit(s, now, "controller", "incident.verify", i.ID, "healthy rollout observed")
				}
			}
		}
		if o.Healthy() {
			delete(s.UnhealthySince, key)
			return nil
		}
		since, ok := s.UnhealthySince[key]
		if !ok || old.UID != o.UID || now.Sub(old.At) > time.Duration(s.Policy.FreshnessSeconds)*time.Second {
			s.UnhealthySince[key] = now
			return nil
		}
		if now.Sub(since) < time.Duration(s.Policy.DetectionSeconds)*time.Second {
			return nil
		}
		for _, i := range s.Incidents {
			if i.Target == key && !i.Created.Before(since) {
				return nil
			}
		}
		if len(s.Incidents) >= 10000 {
			return fmt.Errorf("incident retention limit reached")
		}
		reason := allowed(s, o, now)
		if o.Image == o.HealthyImage {
			reason = "no distinct known-good image; Kubernetes owns ordinary healing"
		}
		state := "AwaitingApproval"
		if reason != "" {
			state = "Rejected"
		}
		plan := Plan{Action: "rollback", Reason: "Deployment rollout remains incomplete across fresh observations; operator-declared recovery digest differs from deployed image", Target: o, Image: o.HealthyImage, RequiresApproval: true, Verification: "same deployment UID and container, approved replica count, execution annotation, recovery image, observed generation, no old replicas, and all desired updated replicas available", Abort: "cancel before execution; in-flight API requests cannot be retracted; inspect deployment before further intervention"}
		i := Incident{ID: id(), Target: key, State: state, Plan: plan, Created: now, Updated: now, Error: reason}
		s.Incidents[i.ID] = i
		audit(s, now, "controller", "incident.detect", i.ID, state)
		audit(s, now, "policy", "plan.evaluate", i.ID, reasonOr(reason, "approval required"))
		return nil
	})
}
func reasonOr(reason, fallback string) string {
	if reason != "" {
		return reason
	}
	return fallback
}
func (v *Service) SetPolicy(ctx context.Context, who Identity, p Policy) error {
	denied := false
	err := v.Store.Update(ctx, func(s *State) error {
		if who.Role != "operator" {
			denied = true
			audit(s, v.Now(), who.Subject, "policy.update", "policy", "denied")
			return nil
		}
		if err := p.Validate(); err != nil {
			return err
		}
		s.Policy = p
		audit(s, v.Now(), who.Subject, "policy.update", "policy", fmt.Sprintf("dryRun=%t emergencyStop=%t", p.DryRun, p.EmergencyStop))
		return nil
	})
	if err != nil {
		return err
	}
	if denied {
		return ErrDenied
	}
	return nil
}
func (v *Service) Approve(ctx context.Context, who Identity, incidentID string) error {
	incidentID = strings.TrimSpace(incidentID)
	if incidentID == "" {
		return ErrInvalid
	}
	denied := false
	err := v.Store.Update(ctx, func(s *State) error {
		i, ok := s.Incidents[incidentID]
		if !ok {
			return ErrNotFound
		}
		if who.Role != "operator" {
			denied = true
			audit(s, v.Now(), who.Subject, "incident.approve", i.ID, "denied")
			return nil
		}
		if i.State != "AwaitingApproval" && i.State != "DryRun" {
			return ErrConflict
		}
		o, ok := s.Observations[i.Target]
		reason := allowed(s, o, v.Now())
		if !ok || o.UID != i.Plan.Target.UID || o.Image != i.Plan.Target.Image || o.HealthyImage != i.Plan.Image || o.Container != i.Plan.Target.Container || o.Desired != i.Plan.Target.Desired || o.Generation != i.Plan.Target.Generation || o.Healthy() {
			reason = "target changed or recovered; replan required"
		}
		if reason != "" {
			denied = true
			audit(s, v.Now(), who.Subject, "incident.approve", i.ID, "denied: "+reason)
			return nil
		}
		if s.Policy.DryRun {
			i.State = "DryRun"
		} else {
			i.State = "Approved"
		}
		i.ApprovedBy = who.Subject
		i.Updated = v.Now()
		s.Incidents[i.ID] = i
		audit(s, v.Now(), who.Subject, "incident.approve", i.ID, i.State)
		return nil
	})
	if err != nil {
		return err
	}
	if denied {
		return ErrDenied
	}
	return nil
}

// Claim serializes admission. Executing replays return the same durable operation key.
func (v *Service) Claim(ctx context.Context, target string) (*Incident, error) {
	var out *Incident
	err := v.Store.Update(ctx, func(s *State) error {
		now := v.Now()
		active := 0
		activeTarget := ""
		activeExecutionID := ""
		for k, i := range s.Incidents {
			if i.State == "Executing" || i.State == "Verifying" {
				if !i.Deadline.After(now) {
					i.State = "Failed"
					i.Error = "execution/verification deadline exceeded"
					i.Updated = now
					s.Incidents[k] = i
					audit(s, now, "executor", "incident.timeout", i.ID, i.Error)
				} else {
					active++
					if i.Target == target {
						activeTarget = i.State
						activeExecutionID = i.ID
					}
				}
			}
		}
		if activeTarget == "Verifying" {
			return nil
		}
		for k, i := range s.Incidents {
			if activeTarget == "Executing" && i.ID != activeExecutionID {
				continue
			}
			if i.Target != target || (i.State != "Approved" && i.State != "Executing") {
				continue
			}
			o := s.Observations[target]
			reason := allowed(s, o, now)
			if s.Policy.DryRun {
				reason = "dry-run enabled"
			}
			if o.Healthy() {
				reason = "target recovered before execution; intervention unnecessary"
			}
			if o.UID != i.Plan.Target.UID || o.Container != i.Plan.Target.Container || o.Desired != i.Plan.Target.Desired || o.HealthyImage != i.Plan.Image || (o.Image == i.Plan.Target.Image && o.Generation != i.Plan.Target.Generation) || (o.Image != i.Plan.Target.Image && !(o.Image == i.Plan.Image && o.ExecutionKey == i.ExecutionKey)) {
				reason = "target changed; execution fenced"
			}
			if reason != "" {
				i.State = "Failed"
				i.Error = reason
				i.Updated = now
				s.Incidents[k] = i
				audit(s, now, "executor", "incident.fence", i.ID, reason)
				return nil
			}
			if i.State == "Approved" {
				if active >= s.Policy.MaxConcurrent {
					return nil
				}
				i.State = "Executing"
				i.ExecutionKey = i.ID
				i.Deadline = now.Add(2 * time.Minute)
				i.Updated = now
				s.Incidents[k] = i
				audit(s, now, "executor", "incident.execute", i.ID, "claimed")
			}
			copy := i
			out = &copy
			return nil
		}
		return nil
	})
	return out, err
}
func (v *Service) MarkVerifying(ctx context.Context, incidentID string) error {
	return v.transition(ctx, Identity{Subject: "executor", Role: "operator"}, incidentID, "Verifying", "")
}
func (v *Service) Fail(ctx context.Context, incidentID, reason string) error {
	return v.transition(ctx, Identity{Subject: "executor", Role: "operator"}, incidentID, "Failed", reason)
}
func (v *Service) Cancel(ctx context.Context, who Identity, incidentID string) error {
	return v.transition(ctx, who, incidentID, "Cancelled", "operator cancellation")
}
func (v *Service) transition(ctx context.Context, who Identity, incidentID, next, reason string) error {
	incidentID = strings.TrimSpace(incidentID)
	if incidentID == "" {
		return ErrInvalid
	}
	denied := false
	err := v.Store.Update(ctx, func(s *State) error {
		i, ok := s.Incidents[incidentID]
		if !ok {
			return ErrNotFound
		}
		if who.Role != "operator" {
			denied = true
			audit(s, v.Now(), who.Subject, "incident.transition", incidentID, "denied")
			return nil
		}
		if next == "Verifying" && i.State == "Verifying" {
			return nil
		}
		if i.State == "Resolved" || i.State == "Cancelled" || i.State == "Failed" || i.State == "Rejected" {
			return ErrConflict
		}
		if next == "Verifying" && i.State != "Executing" {
			return ErrConflict
		}
		i.State = next
		i.Error = reason
		i.Updated = v.Now()
		s.Incidents[incidentID] = i
		audit(s, v.Now(), who.Subject, "incident.transition", incidentID, next)
		return nil
	})
	if err != nil {
		return err
	}
	if denied {
		return ErrDenied
	}
	return nil
}
