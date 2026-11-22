package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateAndSanitizePatchIngress_ValidContract(t *testing.T) {
	patchJSON := []byte(`[
		{"op": "replace", "path": "/spec/replicas", "value": 5},
		{"op": "add", "path": "/spec/env/DEBUG", "value": "true"},
		{"op": "remove", "path": "/spec/deprecated_flag"},
		{"op": "move", "from": "/spec/old_target", "path": "/spec/new_target"},
		{"op": "test", "path": "/spec/version", "value": "v1.2"}
	]`)

	res := ValidateAndSanitizePatchIngress(patchJSON, "urn:deploy:service:payment-gateway", "deploy-agent-01")
	if !res.Valid {
		t.Fatalf("expected valid patch contract, got violations: %+v", res.Violations)
	}

	contract := res.SanitizedContract
	if contract == nil {
		t.Fatal("expected sanitized contract to be non-nil")
	}

	if contract.TargetURN != "urn:deploy:service:payment-gateway" {
		t.Errorf("expected target URN urn:deploy:service:payment-gateway, got %s", contract.TargetURN)
	}

	if len(contract.Operations) != 5 {
		t.Fatalf("expected 5 operations, got %d", len(contract.Operations))
	}

	if contract.Operations[0].Op != "replace" || contract.Operations[0].Path != "/spec/replicas" {
		t.Errorf("unexpected op 0: %+v", contract.Operations[0])
	}
}

func TestValidateAndSanitizePatchIngress_TargetURNValidation(t *testing.T) {
	patchJSON := []byte(`[{"op": "replace", "path": "/spec/replicas", "value": 3}]`)

	// 1. Empty URN
	res := ValidateAndSanitizePatchIngress(patchJSON, "", "author")
	if res.Valid || len(res.Violations) == 0 {
		t.Fatal("expected invalid on empty URN")
	}

	// 2. Malformed URN
	res = ValidateAndSanitizePatchIngress(patchJSON, "invalid-urn-format", "author")
	if res.Valid {
		t.Fatal("expected invalid on malformed URN")
	}

	// 3. Excessive length URN
	longURN := "urn:valid:namespace:" + strings.Repeat("a", 300)
	res = ValidateAndSanitizePatchIngress(patchJSON, longURN, "author")
	if res.Valid {
		t.Fatal("expected invalid on excessively long URN")
	}
}

func TestValidateAndSanitizePatchIngress_InvalidPayloadStructure(t *testing.T) {
	// Empty payload
	res := ValidateAndSanitizePatchIngress([]byte{}, "urn:svc:k8s:cluster", "author")
	if res.Valid {
		t.Fatal("expected invalid on empty payload")
	}

	// Malformed JSON object instead of array
	res = ValidateAndSanitizePatchIngress([]byte(`{"op": "add"}`), "urn:svc:k8s:cluster", "author")
	if res.Valid {
		t.Fatal("expected invalid on JSON object instead of array")
	}

	// Empty array
	res = ValidateAndSanitizePatchIngress([]byte(`[]`), "urn:svc:k8s:cluster", "author")
	if res.Valid {
		t.Fatal("expected invalid on empty operations array")
	}
}

func TestValidateAndSanitizePatchIngress_OperationRulesAndValues(t *testing.T) {
	// 1. Missing value on 'add'
	raw1 := []byte(`[{"op": "add", "path": "/spec/foo"}]`)
	res1 := ValidateAndSanitizePatchIngress(raw1, "urn:svc:k8s:cluster", "author")
	if res1.Valid {
		t.Fatal("expected violation for missing value in 'add'")
	}

	// 2. Missing from on 'move'
	raw2 := []byte(`[{"op": "move", "path": "/spec/new_loc"}]`)
	res2 := ValidateAndSanitizePatchIngress(raw2, "urn:svc:k8s:cluster", "author")
	if res2.Valid {
		t.Fatal("expected violation for missing 'from' in 'move'")
	}

	// 3. Unsupported op
	raw3 := []byte(`[{"op": "nuke", "path": "/spec/data"}]`)
	res3 := ValidateAndSanitizePatchIngress(raw3, "urn:svc:k8s:cluster", "author")
	if res3.Valid {
		t.Fatal("expected violation for unsupported op 'nuke'")
	}
}

func TestValidateAndSanitizePatchIngress_ProtectedPathsGuard(t *testing.T) {
	forbiddenCases := []string{
		"/metadata/uid",
		"/metadata/generation",
		"/metadata/creationTimestamp",
		"/status/reconciledAt",
		"/status/observedGeneration",
		"/system/internal_config",
	}

	for _, path := range forbiddenCases {
		ops := []PatchOperation{{Op: "replace", Path: path, Value: "injected"}}
		bytes, _ := json.Marshal(ops)
		res := ValidateAndSanitizePatchIngress(bytes, "urn:svc:k8s:cluster", "author")
		if res.Valid {
			t.Fatalf("expected violation for protected path '%s', but passed", path)
		}
	}
}

func TestValidateAndSanitizePatchIngress_PrototypePollutionAndControlChars(t *testing.T) {
	// Prototype pollution in path
	raw1 := []byte(`[{"op": "add", "path": "/spec/__proto__/polluted", "value": true}]`)
	res1 := ValidateAndSanitizePatchIngress(raw1, "urn:svc:k8s:cluster", "author")
	if res1.Valid {
		t.Fatal("expected violation for __proto__ pollution attempt")
	}

	// Prototype pollution in from
	raw2 := []byte(`[{"op": "move", "from": "/constructor/name", "path": "/spec/safe", "value": true}]`)
	res2 := ValidateAndSanitizePatchIngress(raw2, "urn:svc:k8s:cluster", "author")
	if res2.Valid {
		t.Fatal("expected violation for constructor in 'from' pointer")
	}

	// Null byte in path
	raw3 := []byte(`[{"op": "add", "path": "/spec/data\u0000injected", "value": 1}]`)
	res3 := ValidateAndSanitizePatchIngress(raw3, "urn:svc:k8s:cluster", "author")
	if res3.Valid {
		t.Fatal("expected violation for null byte in path")
	}
}
