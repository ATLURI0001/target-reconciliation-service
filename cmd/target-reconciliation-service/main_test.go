package main

import (
	"strings"
	"testing"
)

func TestConfigurationFailsBeforeDatabaseAccess(t *testing.T) {
	for _, tc := range []struct {
		name, key, value string
		wantError        bool
	}{
		{"valid", "DATABASE_URL", "postgres://placeholder/isolated", false},
		{"explicit database", "DATABASE_URL", "", true},
		{"weak token", "TRS_OPERATOR_TOKEN", "short", true},
		{"distinct roles", "TRS_VIEWER_TOKEN", strings.Repeat("o", 32), true},
		{"boolean", "TRS_KUBERNETES", "maybe", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://placeholder/isolated")
			t.Setenv("TRS_OPERATOR_TOKEN", strings.Repeat("o", 32))
			t.Setenv("TRS_VIEWER_TOKEN", strings.Repeat("v", 32))
			t.Setenv("TRS_KUBERNETES", "false")
			t.Setenv(tc.key, tc.value)
			if (validateEnvironment() != nil) != tc.wantError {
				t.Fatal("configuration validation mismatch")
			}
		})
	}
	t.Run("namespace", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://placeholder/isolated")
		t.Setenv("TRS_OPERATOR_TOKEN", strings.Repeat("o", 32))
		t.Setenv("TRS_VIEWER_TOKEN", strings.Repeat("v", 32))
		t.Setenv("TRS_KUBERNETES", "true")
		t.Setenv("TRS_NAMESPACE", "kube-system")
		if validateEnvironment() == nil {
			t.Fatal("unsafe namespace")
		}
	})
}
