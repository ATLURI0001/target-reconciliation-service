package core

import (
	"testing"
)

func TestValidateTargetSpec_ValidSpec(t *testing.T) {
	spec := TargetResourceSpec{
		URN:      "urn:cloud:deployment:prod/auth-service",
		Kind:     "deployment",
		Replicas: 3,
		CPU:      "500m",
		Memory:   "1Gi",
		Labels: map[string]string{
			"app": "auth-service",
			"tier": "backend",
		},
		Env: map[string]string{
			"PORT": "8080",
			"LOG_LEVEL": "info",
			"DATABASE_HOST": "db.internal.local",
		},
	}

	report := ValidateTargetSpec(spec)
	if !report.Valid {
		t.Fatalf("expected valid spec, got errors: %v", report.Errors)
	}
	if len(report.SanitizedEnv) != 3 {
		t.Fatalf("expected 3 sanitized env vars, got %d", len(report.SanitizedEnv))
	}
}

func TestValidateTargetSpec_InvalidURNAndKind(t *testing.T) {
	spec := TargetResourceSpec{
		URN:      "invalid-urn-format",
		Kind:     "unsupported-kind",
		Replicas: -5,
	}

	report := ValidateTargetSpec(spec)
	if report.Valid {
		t.Fatalf("expected invalid report, got valid")
	}

	var hasURN, hasKind, hasReplicas bool
	for _, err := range report.Errors {
		if err.Field == "urn" {
			hasURN = true
		}
		if err.Field == "kind" {
			hasKind = true
		}
		if err.Field == "replicas" {
			hasReplicas = true
		}
	}

	if !hasURN || !hasKind || !hasReplicas {
		t.Errorf("missing expected validation errors: urn=%v, kind=%v, replicas=%v", hasURN, hasKind, hasReplicas)
	}
}

func TestValidateTargetSpec_ShellInjectionAndForbiddenEnv(t *testing.T) {
	spec := TargetResourceSpec{
		URN:      "urn:cloud:deployment:prod/api-gateway",
		Kind:     "deployment",
		Replicas: 2,
		Env: map[string]string{
			"SAFE_VAR":       "normal_value_123",
			"INJECTION_CMD":  "val; rm -rf /",
			"INJECTION_PIPE": "val | cat /etc/passwd",
			"LD_PRELOAD":     "/tmp/evil.so",
			"123_INVALID":    "numeric_start",
		},
	}

	report := ValidateTargetSpec(spec)
	if report.Valid {
		t.Fatalf("expected validation failures on malicious env vars")
	}

	if _, ok := report.SanitizedEnv["SAFE_VAR"]; !ok {
		t.Errorf("SAFE_VAR should have been retained in sanitized env")
	}
	if _, ok := report.SanitizedEnv["INJECTION_CMD"]; ok {
		t.Errorf("INJECTION_CMD should have been purged")
	}
	if _, ok := report.SanitizedEnv["LD_PRELOAD"]; ok {
		t.Errorf("LD_PRELOAD should have been rejected as restricted")
	}
}

func TestValidateTargetSpec_ComputeFormat(t *testing.T) {
	spec := TargetResourceSpec{
		URN:    "urn:cloud:deployment:prod/cache",
		Kind:   "deployment",
		CPU:    "bad-cpu",
		Memory: "bad-memory",
	}

	report := ValidateTargetSpec(spec)
	if report.Valid {
		t.Fatalf("expected failure on bad cpu/memory format")
	}

	var hasCPU, hasMem bool
	for _, err := range report.Errors {
		if err.Field == "cpu" {
			hasCPU = true
		}
		if err.Field == "memory" {
			hasMem = true
		}
	}

	if !hasCPU || !hasMem {
		t.Errorf("missing cpu/memory errors: cpu=%v, mem=%v", hasCPU, hasMem)
	}
}
