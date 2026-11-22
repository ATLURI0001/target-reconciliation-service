package core

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Allowed JSON Patch (RFC 6902) Operations
const (
	OpAdd     = "add"
	OpRemove  = "remove"
	OpReplace = "replace"
	OpMove    = "move"
	OpCopy    = "copy"
	OpTest    = "test"
)

// Supported Patch Types
const (
	PatchTypeJSONPatch  = "application/json-patch+json"
	PatchTypeMergePatch = "application/merge-patch+json"
)

var (
	validURNRegex = regexp.MustCompile(`^urn:[a-z0-9_-]+:[a-z0-9_.-]+:[a-z0-9_./-]+$`)
	forbiddenPaths = []string{
		"/metadata/uid",
		"/metadata/generation",
		"/metadata/creationTimestamp",
		"/status/reconciledAt",
		"/status/observedGeneration",
		"/system",
	}
	dangerousKeyRegex = regexp.MustCompile(`(?i)(__proto__|constructor|prototype)`)
)

// PatchOperation represents a single RFC 6902 JSON Patch action.
type PatchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	From  string      `json:"from,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

// ResourcePatchContract defines the validated ingress payload for applying patches.
type ResourcePatchContract struct {
	TargetURN   string           `json:"target_urn"`
	PatchType   string           `json:"patch_type"`
	Operations  []PatchOperation `json:"operations,omitempty"`
	Author      string           `json:"author"`
	SubmittedAt time.Time        `json:"submitted_at"`
}

// PatchValidationViolation details an individual rule breach.
type PatchValidationViolation struct {
	Field   string `json:"field"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

// PatchValidationResult holds the overall pass/fail status and sanitized contract.
type PatchValidationResult struct {
	Valid              bool                       `json:"valid"`
	Violations         []PatchValidationViolation `json:"violations,omitempty"`
	SanitizedContract  *ResourcePatchContract     `json:"sanitized_contract,omitempty"`
}

// ValidateAndSanitizePatchIngress verifies schema conformance, validates RFC 6901 pointers,
// guards against protected system field mutations, and prevents prototype/constructor pollution.
func ValidateAndSanitizePatchIngress(rawJSON []byte, targetURN string, author string) PatchValidationResult {
	var violations []PatchValidationViolation

	// 1. Target URN validation
	sanitizedURN := strings.TrimSpace(targetURN)
	if sanitizedURN == "" {
		violations = append(violations, PatchValidationViolation{
			Field:   "target_urn",
			Rule:    "required",
			Message: "target URN must not be empty",
		})
	} else if len(sanitizedURN) > 256 {
		violations = append(violations, PatchValidationViolation{
			Field:   "target_urn",
			Rule:    "max_length",
			Message: "target URN cannot exceed 256 characters",
		})
	} else if !validURNRegex.MatchString(sanitizedURN) {
		violations = append(violations, PatchValidationViolation{
			Field:   "target_urn",
			Rule:    "format_mismatch",
			Message: fmt.Sprintf("target URN '%s' does not match urn:namespace:type:id format", sanitizedURN),
		})
	}

	// 2. Author sanitization
	sanitizedAuthor := strings.TrimSpace(author)
	if sanitizedAuthor == "" {
		sanitizedAuthor = "system/anonymous"
	} else if len(sanitizedAuthor) > 128 {
		sanitizedAuthor = sanitizedAuthor[:128]
	}

	// 3. Raw JSON parse & array inspection
	if len(rawJSON) == 0 {
		violations = append(violations, PatchValidationViolation{
			Field:   "payload",
			Rule:    "required",
			Message: "patch payload must not be empty",
		})
		return PatchValidationResult{Valid: false, Violations: violations}
	}

	var rawOps []PatchOperation
	if err := json.Unmarshal(rawJSON, &rawOps); err != nil {
		violations = append(violations, PatchValidationViolation{
			Field:   "payload",
			Rule:    "malformed_json",
			Message: fmt.Sprintf("payload must be a valid JSON array of patch operations: %v", err),
		})
		return PatchValidationResult{Valid: false, Violations: violations}
	}

	if len(rawOps) == 0 {
		violations = append(violations, PatchValidationViolation{
			Field:   "operations",
			Rule:    "min_items",
			Message: "patch operations array cannot be empty",
		})
		return PatchValidationResult{Valid: false, Violations: violations}
	}

	if len(rawOps) > 100 {
		violations = append(violations, PatchValidationViolation{
			Field:   "operations",
			Rule:    "max_items",
			Message: fmt.Sprintf("operations count (%d) exceeds max boundary of 100", len(rawOps)),
		})
	}

	var sanitizedOps []PatchOperation
	for idx, op := range rawOps {
		opFieldPrefix := fmt.Sprintf("operations[%d]", idx)
		normOp := strings.ToLower(strings.TrimSpace(op.Op))

		switch normOp {
		case OpAdd, OpReplace, OpTest:
			if op.Value == nil {
				violations = append(violations, PatchValidationViolation{
					Field:   fmt.Sprintf("%s.value", opFieldPrefix),
					Rule:    "required_value",
					Message: fmt.Sprintf("operation '%s' requires a non-nil 'value'", normOp),
				})
			}
		case OpRemove:
			// No value needed
		case OpMove, OpCopy:
			if strings.TrimSpace(op.From) == "" {
				violations = append(violations, PatchValidationViolation{
					Field:   fmt.Sprintf("%s.from", opFieldPrefix),
					Rule:    "required_from",
					Message: fmt.Sprintf("operation '%s' requires a non-empty 'from' path", normOp),
				})
			} else {
				normFrom := sanitizeJSONPointer(op.From)
				if !strings.HasPrefix(normFrom, "/") {
					violations = append(violations, PatchValidationViolation{
						Field:   fmt.Sprintf("%s.from", opFieldPrefix),
						Rule:    "invalid_pointer",
						Message: fmt.Sprintf("'from' pointer '%s' must start with '/'", op.From),
					})
				}
				if checkDangerousSegments(normFrom) {
					violations = append(violations, PatchValidationViolation{
						Field:   fmt.Sprintf("%s.from", opFieldPrefix),
						Rule:    "pollution_attempt",
						Message: fmt.Sprintf("'from' pointer '%s' contains forbidden prototype/constructor property", op.From),
					})
				}
			}
		default:
			violations = append(violations, PatchValidationViolation{
				Field:   fmt.Sprintf("%s.op", opFieldPrefix),
				Rule:    "unsupported_op",
				Message: fmt.Sprintf("unsupported patch op '%s'", op.Op),
			})
		}

		// Validate Path
		normPath := sanitizeJSONPointer(op.Path)
		if normPath == "" || !strings.HasPrefix(normPath, "/") {
			violations = append(violations, PatchValidationViolation{
				Field:   fmt.Sprintf("%s.path", opFieldPrefix),
				Rule:    "invalid_pointer",
				Message: fmt.Sprintf("path '%s' must be a valid RFC 6901 pointer beginning with '/'", op.Path),
			})
		}

		// Check forbidden protected system paths
		for _, forbidden := range forbiddenPaths {
			if normPath == forbidden || strings.HasPrefix(normPath, forbidden+"/") {
				violations = append(violations, PatchValidationViolation{
					Field:   fmt.Sprintf("%s.path", opFieldPrefix),
					Rule:    "protected_path",
					Message: fmt.Sprintf("path '%s' modifies immutable system or metadata field", normPath),
				})
				break
			}
		}

		// Check prototype pollution or constructor injection
		if checkDangerousSegments(normPath) {
			violations = append(violations, PatchValidationViolation{
				Field:   fmt.Sprintf("%s.path", opFieldPrefix),
				Rule:    "pollution_attempt",
				Message: fmt.Sprintf("path '%s' contains forbidden prototype or constructor segment", normPath),
			})
		}

		// Check control characters / null bytes in path
		if strings.ContainsAny(normPath, "\x00\r\n\t") {
			violations = append(violations, PatchValidationViolation{
				Field:   fmt.Sprintf("%s.path", opFieldPrefix),
				Rule:    "control_characters",
				Message: fmt.Sprintf("path '%s' contains illegal control characters", normPath),
			})
		}

		sanitizedOps = append(sanitizedOps, PatchOperation{
			Op:    normOp,
			Path:  normPath,
			From:  sanitizeJSONPointer(op.From),
			Value: op.Value,
		})
	}

	if len(violations) > 0 {
		return PatchValidationResult{
			Valid:      false,
			Violations: violations,
		}
	}

	return PatchValidationResult{
		Valid: true,
		SanitizedContract: &ResourcePatchContract{
			TargetURN:   sanitizedURN,
			PatchType:   PatchTypeJSONPatch,
			Operations:  sanitizedOps,
			Author:      sanitizedAuthor,
			SubmittedAt: time.Now().UTC(),
		},
	}
}

// sanitizeJSONPointer removes redundant trailing slashes and trims whitespace.
func sanitizeJSONPointer(p string) string {
	trimmed := strings.TrimSpace(p)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) > 1 && strings.HasSuffix(trimmed, "/") {
		trimmed = strings.TrimRight(trimmed, "/")
	}
	return trimmed
}

// checkDangerousSegments returns true if any segment attempts prototype pollution.
func checkDangerousSegments(p string) bool {
	segments := strings.Split(p, "/")
	for _, seg := range segments {
		if dangerousKeyRegex.MatchString(seg) {
			return true
		}
	}
	return false
}
