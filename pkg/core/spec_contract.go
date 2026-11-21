package core

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrContractViolation = errors.New("spec_contract: target resource specification violates contract")
	ErrShellInjection    = errors.New("spec_contract: shell injection character detected in environment variable")

	urnRegex      = regexp.MustCompile(`^urn:[a-z0-9-]+:[a-z0-9-]+:[a-z0-9-]+/[a-z0-9-]+$`)
	dnsLabelRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*(/[a-z0-9A-Z_.-]{1,63})?$`)
	cpuRegex      = regexp.MustCompile(`^([0-9]+m|[0-9]+(\.[0-9]+)?)$`)
	memoryRegex   = regexp.MustCompile(`^[0-9]+(Ei|Pi|Ti|Gi|Mi|Ki|E|P|T|G|M|K)?$`)
	envNameRegex  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)

	forbiddenEnvVars = map[string]bool{
		"LD_PRELOAD":        true,
		"LD_LIBRARY_PATH":   true,
		"IFS":               true,
		"BASH_ENV":          true,
		"ENV":               true,
		"PYTHONPATH":        true,
		"NODE_OPTIONS":      true,
		"PERL5LIB":          true,
	}

	shellMetaChars = []string{";", "&", "|", "`", "$", "\n", "\r", "<", ">", "\\", "\"", "'", "\x00"}
)

// AllowedResourceKinds defines supported resource categories.
var AllowedResourceKinds = map[string]bool{
	"deployment":  true,
	"service":     true,
	"configmap":   true,
	"secret":      true,
	"statefulset": true,
}

// TargetResourceSpec represents the ingress desired state for a managed resource.
type TargetResourceSpec struct {
	URN      string            `json:"urn"`
	Kind     string            `json:"kind"`
	Replicas int               `json:"replicas"`
	CPU      string            `json:"cpu"`
	Memory   string            `json:"memory"`
	Labels   map[string]string `json:"labels,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
}

// ContractError records a specific field validation failure.
type ContractError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ContractReport captures total validation result and sanitization details.
type ContractReport struct {
	Valid        bool            `json:"valid"`
	Errors       []ContractError `json:"errors,omitempty"`
	SanitizedEnv map[string]string `json:"sanitized_env,omitempty"`
}

// ValidateTargetSpec executes comprehensive contract schema validation and shell sanitization.
func ValidateTargetSpec(spec TargetResourceSpec) ContractReport {
	report := ContractReport{
		Valid:        true,
		Errors:       make([]ContractError, 0),
		SanitizedEnv: make(map[string]string),
	}

	// 1. URN Validation
	if spec.URN == "" {
		report.Errors = append(report.Errors, ContractError{
			Field:   "urn",
			Code:    "REQUIRED_FIELD",
			Message: "Resource URN is mandatory",
		})
	} else if !urnRegex.MatchString(spec.URN) {
		report.Errors = append(report.Errors, ContractError{
			Field:   "urn",
			Code:    "INVALID_FORMAT",
			Message: fmt.Sprintf("URN '%s' must match pattern urn:<domain>:<kind>:<namespace>/<name>", spec.URN),
		})
	}

	// 2. Kind Validation
	normalizedKind := strings.ToLower(strings.TrimSpace(spec.Kind))
	if normalizedKind == "" {
		report.Errors = append(report.Errors, ContractError{
			Field:   "kind",
			Code:    "REQUIRED_FIELD",
			Message: "Resource kind is mandatory",
		})
	} else if !AllowedResourceKinds[normalizedKind] {
		report.Errors = append(report.Errors, ContractError{
			Field:   "kind",
			Code:    "INVALID_KIND",
			Message: fmt.Sprintf("Kind '%s' is not supported; must be one of [deployment, service, configmap, secret, statefulset]", spec.Kind),
		})
	}

	// 3. Replicas Bounds
	if spec.Replicas < 0 || spec.Replicas > 1000 {
		report.Errors = append(report.Errors, ContractError{
			Field:   "replicas",
			Code:    "OUT_OF_BOUNDS",
			Message: fmt.Sprintf("Replica count %d must be in range [0, 1000]", spec.Replicas),
		})
	}

	// 4. Compute Resource Syntax (CPU & Memory)
	if spec.CPU != "" && !cpuRegex.MatchString(spec.CPU) {
		report.Errors = append(report.Errors, ContractError{
			Field:   "cpu",
			Code:    "INVALID_CPU_FORMAT",
			Message: fmt.Sprintf("CPU '%s' must be valid quantity (e.g., 500m, 2, 0.5)", spec.CPU),
		})
	}
	if spec.Memory != "" && !memoryRegex.MatchString(spec.Memory) {
		report.Errors = append(report.Errors, ContractError{
			Field:   "memory",
			Code:    "INVALID_MEMORY_FORMAT",
			Message: fmt.Sprintf("Memory '%s' must be valid quantity (e.g., 512Mi, 2Gi)", spec.Memory),
		})
	}

	// 5. Label RFC 1123 Validation
	if len(spec.Labels) > 64 {
		report.Errors = append(report.Errors, ContractError{
			Field:   "labels",
			Code:    "MAX_LABELS_EXCEEDED",
			Message: fmt.Sprintf("Labels map contains %d entries, exceeding maximum limit of 64", len(spec.Labels)),
		})
	} else {
		for k, v := range spec.Labels {
			if len(k) > 63 || !dnsLabelRegex.MatchString(k) {
				report.Errors = append(report.Errors, ContractError{
					Field:   fmt.Sprintf("labels[%s]", k),
					Code:    "INVALID_LABEL_KEY",
					Message: fmt.Sprintf("Label key '%s' violates DNS label format", k),
				})
			}
			if len(v) > 63 {
				report.Errors = append(report.Errors, ContractError{
					Field:   fmt.Sprintf("labels[%s]", k),
					Code:    "LABEL_VALUE_TOO_LONG",
					Message: fmt.Sprintf("Label value for '%s' exceeds 63 characters", k),
				})
			}
		}
	}

	// 6. Environment Variable Sanitization & Shell Injection Guard
	if len(spec.Env) > 128 {
		report.Errors = append(report.Errors, ContractError{
			Field:   "env",
			Code:    "MAX_ENV_EXCEEDED",
			Message: fmt.Sprintf("Env map contains %d entries, exceeding limit of 128", len(spec.Env)),
		})
	} else {
		for k, v := range spec.Env {
			if !envNameRegex.MatchString(k) {
				report.Errors = append(report.Errors, ContractError{
					Field:   fmt.Sprintf("env[%s]", k),
					Code:    "INVALID_ENV_NAME",
					Message: fmt.Sprintf("Environment variable key '%s' is not a valid POSIX identifier", k),
				})
				continue
			}

			if forbiddenEnvVars[strings.ToUpper(k)] {
				report.Errors = append(report.Errors, ContractError{
					Field:   fmt.Sprintf("env[%s]", k),
					Code:    "RESTRICTED_ENV_VAR",
					Message: fmt.Sprintf("Environment variable key '%s' is restricted for security isolation", k),
				})
				continue
			}

			// Audit for shell metacharacters in value
			hasInjection := false
			for _, meta := range shellMetaChars {
				if strings.Contains(v, meta) {
					report.Errors = append(report.Errors, ContractError{
						Field:   fmt.Sprintf("env[%s]", k),
						Code:    "SHELL_INJECTION_DETECTED",
						Message: fmt.Sprintf("Dangerous shell meta-character '%s' detected in value for %s", meta, k),
					})
					hasInjection = true
					break
				}
			}

			if !hasInjection {
				report.SanitizedEnv[k] = strings.TrimSpace(v)
			}
		}
	}

	if len(report.Errors) > 0 {
		report.Valid = false
	}

	return report
}
