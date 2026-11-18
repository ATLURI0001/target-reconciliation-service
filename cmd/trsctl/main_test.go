package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunArgs_ValidationErrors(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantSubstr string
	}{
		{
			name:       "empty args",
			args:       []string{},
			wantSubstr: "usage: trsctl",
		},
		{
			name:       "unknown command",
			args:       []string{"foo"},
			wantSubstr: "unknown command",
		},
		{
			name:       "register missing id",
			args:       []string{"register"},
			wantSubstr: "register requires cluster ID",
		},
		{
			name:       "register blank id",
			args:       []string{"register", "   "},
			wantSubstr: "register requires cluster ID",
		},
		{
			name:       "policy missing file",
			args:       []string{"policy"},
			wantSubstr: "policy requires JSON file",
		},
		{
			name:       "policy blank file",
			args:       []string{"policy", "  "},
			wantSubstr: "policy requires JSON file",
		},
		{
			name:       "policy nonexistent file",
			args:       []string{"policy", "nonexistent-file.json"},
			wantSubstr: "nonexistent-file.json",
		},
		{
			name:       "approve missing id",
			args:       []string{"approve"},
			wantSubstr: "incident ID required",
		},
		{
			name:       "approve blank id",
			args:       []string{"approve", "   "},
			wantSubstr: "incident ID required",
		},
		{
			name:       "cancel missing id",
			args:       []string{"cancel"},
			wantSubstr: "incident ID required",
		},
		{
			name:       "cancel blank id",
			args:       []string{"cancel", "\t"},
			wantSubstr: "incident ID required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := runArgs(tc.args, &out)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantSubstr)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantSubstr, err)
			}
		})
	}
}

func TestRunArgs_MissingToken(t *testing.T) {
	t.Setenv("TRS_TOKEN", "")
	var out bytes.Buffer
	err := runArgs([]string{"snapshot"}, &out)
	if err == nil || !strings.Contains(err.Error(), "TRS_TOKEN") {
		t.Fatalf("expected TRS_TOKEN error, got %v", err)
	}
}

func TestRunArgs_ServerInteractions(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/snapshot", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-operator-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})
	mux.HandleFunc("/v1/clusters", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"registered":true}`))
	})
	mux.HandleFunc("/v1/policy", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"updated":true}`))
	})
	mux.HandleFunc("/v1/incidents/inc-123/approve", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"approved":true}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	t.Setenv("TRS_API_URL", srv.URL)
	t.Setenv("TRS_TOKEN", "test-operator-token")

	t.Run("snapshot success", func(t *testing.T) {
		var out bytes.Buffer
		err := runArgs([]string{"snapshot"}, &out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.String(), "healthy") {
			t.Fatalf("expected snapshot response in stdout, got %q", out.String())
		}
	})

	t.Run("register success with trimmed arg", func(t *testing.T) {
		var out bytes.Buffer
		err := runArgs([]string{"register", "  prod-cluster-01  "}, &out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.String(), "registered") {
			t.Fatalf("expected register response, got %q", out.String())
		}
	})

	t.Run("policy update success", func(t *testing.T) {
		tempDir := t.TempDir()
		policyFile := filepath.Join(tempDir, "policy.json")
		if err := os.WriteFile(policyFile, []byte(`{"enforce":true}`), 0644); err != nil {
			t.Fatalf("failed to write policy file: %v", err)
		}

		var out bytes.Buffer
		err := runArgs([]string{"policy", policyFile}, &out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.String(), "updated") {
			t.Fatalf("expected policy update response, got %q", out.String())
		}
	})

	t.Run("policy empty file rejected", func(t *testing.T) {
		tempDir := t.TempDir()
		emptyFile := filepath.Join(tempDir, "empty_policy.json")
		if err := os.WriteFile(emptyFile, []byte("   \t\n "), 0644); err != nil {
			t.Fatalf("failed to write empty file: %v", err)
		}

		var out bytes.Buffer
		err := runArgs([]string{"policy", emptyFile}, &out)
		if err == nil || !strings.Contains(err.Error(), "policy file is empty") {
			t.Fatalf("expected 'policy file is empty' error, got %v", err)
		}
	})

	t.Run("policy invalid json rejected", func(t *testing.T) {
		tempDir := t.TempDir()
		invalidFile := filepath.Join(tempDir, "invalid.json")
		if err := os.WriteFile(invalidFile, []byte("{not-valid-json}"), 0644); err != nil {
			t.Fatalf("failed to write invalid file: %v", err)
		}

		var out bytes.Buffer
		err := runArgs([]string{"policy", invalidFile}, &out)
		if err == nil || !strings.Contains(err.Error(), "invalid policy JSON") {
			t.Fatalf("expected 'invalid policy JSON' error, got %v", err)
		}
	})

	t.Run("approve success", func(t *testing.T) {
		var out bytes.Buffer
		err := runArgs([]string{"approve", "inc-123"}, &out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.String(), "approved") {
			t.Fatalf("expected approve response, got %q", out.String())
		}
	})
}
