package core

import (
	"strings"
	"testing"
)

func TestParseAndSanitizeManifest_ValidV1(t *testing.T) {
	raw := `{
		"apiVersion": "reconciliation.dev/v1",
		"kind": "TargetManifest",
		"metadata": {
			"name": "prod-billing-cluster",
			"namespace": "production"
		},
		"spec": {
			"targetId": "billing-core",
			"reconciliationPolicy": "StrictConvergence",
			"syncIntervalSec": 60,
			"concurrencyLimit": 10,
			"retryPolicy": {
				"maxRetries": 3,
				"backoffType": "exponential",
				"baseIntervalSec": 2,
				"maxIntervalSec": 30
			},
			"resources": [
				{
					"kind": "Database",
					"name": "primary-postgres",
					"required": true
				},
				{
					"kind": "Service",
					"name": "billing-api",
					"required": true,
					"dependencies": ["Database/primary-postgres"]
				}
			],
			"degradationThresholdPercent": 5.0
		}
	}`

	manifest, err := ParseAndSanitizeManifest([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	if manifest.APIVersion != APIVersionV1 {
		t.Errorf("expected apiVersion %s, got %s", APIVersionV1, manifest.APIVersion)
	}
	if manifest.Metadata.Namespace != "production" {
		t.Errorf("expected namespace 'production', got '%s'", manifest.Metadata.Namespace)
	}
	if manifest.SpecHash == "" {
		t.Error("expected non-empty SpecHash")
	}
}

func TestParseAndSanitizeManifest_SchemaEvolution(t *testing.T) {
	// Legacy v1alpha1 manifest should auto-evolve to v1
	rawAlpha := `{
		"apiVersion": "reconciliation.dev/v1alpha1",
		"kind": "TargetManifest",
		"metadata": { "name": "legacy-app" },
		"spec": {
			"targetId": "legacy-service",
			"resources": [{ "kind": "Deployment", "name": "app", "required": true }]
		}
	}`

	manifest, err := ParseAndSanitizeManifest([]byte(rawAlpha))
	if err != nil {
		t.Fatalf("failed to evolve legacy alpha manifest: %v", err)
	}

	if manifest.APIVersion != APIVersionV1 {
		t.Errorf("expected evolved apiVersion %s, got %s", APIVersionV1, manifest.APIVersion)
	}
	if manifest.Metadata.Namespace != "default" {
		t.Errorf("expected default namespace fallback, got %s", manifest.Metadata.Namespace)
	}
	if manifest.Spec.ReconciliationPolicy != PolicyStrictConvergence {
		t.Errorf("expected default StrictConvergence policy, got %s", manifest.Spec.ReconciliationPolicy)
	}
}

func TestParseAndSanitizeManifest_Rejections(t *testing.T) {
	tests := []struct {
		name      string
		rawJSON   string
		errSubstr string
	}{
		{
			name: "unsupported apiVersion",
			rawJSON: `{
				"apiVersion": "reconciliation.dev/v99",
				"kind": "TargetManifest",
				"metadata": { "name": "test" },
				"spec": { "targetId": "t1", "resources": [{ "kind": "A", "name": "B" }] }
			}`,
			errSubstr: "unsupported manifest apiVersion",
		},
		{
			name: "invalid kind",
			rawJSON: `{
				"apiVersion": "reconciliation.dev/v1",
				"kind": "UnknownKind",
				"metadata": { "name": "test" },
				"spec": { "targetId": "t1", "resources": [{ "kind": "A", "name": "B" }] }
			}`,
			errSubstr: "invalid manifest kind",
		},
		{
			name: "invalid metadata name uppercase",
			rawJSON: `{
				"apiVersion": "reconciliation.dev/v1",
				"kind": "TargetManifest",
				"metadata": { "name": "INVALID_UPPERCASE" },
				"spec": { "targetId": "t1", "resources": [{ "kind": "A", "name": "B" }] }
			}`,
			errSubstr: "invalid metadata.name",
		},
		{
			name: "invalid targetId with spaces",
			rawJSON: `{
				"apiVersion": "reconciliation.dev/v1",
				"kind": "TargetManifest",
				"metadata": { "name": "valid-name" },
				"spec": { "targetId": "bad target id", "resources": [{ "kind": "A", "name": "B" }] }
			}`,
			errSubstr: "invalid spec.targetId",
		},
		{
			name: "empty resources list",
			rawJSON: `{
				"apiVersion": "reconciliation.dev/v1",
				"kind": "TargetManifest",
				"metadata": { "name": "valid-name" },
				"spec": { "targetId": "valid-id", "resources": [] }
			}`,
			errSubstr: "manifest must define at least one resource",
		},
		{
			name: "duplicate resource declaration",
			rawJSON: `{
				"apiVersion": "reconciliation.dev/v1",
				"kind": "TargetManifest",
				"metadata": { "name": "valid-name" },
				"spec": {
					"targetId": "valid-id",
					"resources": [
						{ "kind": "Service", "name": "web" },
						{ "kind": "Service", "name": "web" }
					]
				}
			}`,
			errSubstr: "duplicate resource definition",
		},
		{
			name: "unknown dependency reference",
			rawJSON: `{
				"apiVersion": "reconciliation.dev/v1",
				"kind": "TargetManifest",
				"metadata": { "name": "valid-name" },
				"spec": {
					"targetId": "valid-id",
					"resources": [
						{ "kind": "Service", "name": "web", "dependencies": ["Database/missing-db"] }
					]
				}
			}`,
			errSubstr: "depends on unknown resource",
		},
		{
			name: "circular dependency 2 nodes",
			rawJSON: `{
				"apiVersion": "reconciliation.dev/v1",
				"kind": "TargetManifest",
				"metadata": { "name": "valid-name" },
				"spec": {
					"targetId": "valid-id",
					"resources": [
						{ "kind": "Service", "name": "A", "dependencies": ["Service/B"] },
						{ "kind": "Service", "name": "B", "dependencies": ["Service/A"] }
					]
				}
			}`,
			errSubstr: "cyclic dependency",
		},
		{
			name: "self dependency",
			rawJSON: `{
				"apiVersion": "reconciliation.dev/v1",
				"kind": "TargetManifest",
				"metadata": { "name": "valid-name" },
				"spec": {
					"targetId": "valid-id",
					"resources": [
						{ "kind": "Service", "name": "A", "dependencies": ["Service/A"] }
					]
				}
			}`,
			errSubstr: "self-depends",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseAndSanitizeManifest([]byte(tt.rawJSON))
			if err == nil {
				t.Fatalf("expected error containing '%s', got nil", tt.errSubstr)
			}
			if !strings.Contains(err.Error(), tt.errSubstr) {
				t.Errorf("expected error containing '%s', got '%v'", tt.errSubstr, err)
			}
		})
	}
}

func TestComputeSpecHash_Deterministic(t *testing.T) {
	spec1 := TargetManifestSpec{
		TargetID:             "edge-cluster",
		ReconciliationPolicy: PolicyBestEffort,
		SyncIntervalSec:      15,
		Resources: []ResourceRequirement{
			{Kind: "Service", Name: "api", Dependencies: []string{"Database/db", "Cache/redis"}},
			{Kind: "Database", Name: "db"},
			{Kind: "Cache", Name: "redis"},
		},
	}

	// Permute resource order and dependency order
	spec2 := TargetManifestSpec{
		TargetID:             "edge-cluster",
		ReconciliationPolicy: PolicyBestEffort,
		SyncIntervalSec:      15,
		Resources: []ResourceRequirement{
			{Kind: "Cache", Name: "redis"},
			{Kind: "Database", Name: "db"},
			{Kind: "Service", Name: "api", Dependencies: []string{"Cache/redis", "Database/db"}},
		},
	}

	hash1, err := ComputeSpecHash(spec1)
	if err != nil {
		t.Fatalf("hash1 failed: %v", err)
	}

	hash2, err := ComputeSpecHash(spec2)
	if err != nil {
		t.Fatalf("hash2 failed: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("expected identical hashes regardless of ordering:\nhash1: %s\nhash2: %s", hash1, hash2)
	}
}
