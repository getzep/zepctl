package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getzep/zepctl/internal/config"
	"github.com/spf13/viper"
)

// TestConfigSetProject_DirectUUID verifies that "config set-project <uuid>"
// sets the project without interactive prompt.
func TestConfigSetProject_DirectUUID(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	// Write config file so Load() and Save() work.
	writeTestConfig(t, tmpDir)
	_, _ = config.Reload()

	cmd := configSetProjectCmd
	cmd.SetContext(context.Background())

	err := cmd.RunE(cmd, []string{"proj-uuid-456"})
	if err != nil {
		t.Fatalf("set-project direct UUID: %v", err)
	}

	// Reload and verify project was set.
	cfg, err := config.Reload()
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	profile := cfg.GetProfile("test")
	if profile == nil {
		t.Fatal("profile 'test' not found after set-project")
	}
	if profile.ProjectUUID != "proj-uuid-456" {
		t.Errorf("ProjectUUID = %q, want %q", profile.ProjectUUID, "proj-uuid-456")
	}
}

func TestAuthenticateAndGetProjects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/web/v1/authenticate" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"account_uuid": "acc-uuid-123",
			"projects": []projectInfo{
				{UUID: "proj-1", Name: "Project One"},
				{UUID: "proj-2", Name: "Project Two"},
			},
		})
	}))
	defer srv.Close()

	accountUUID, projects, err := authenticateAndGetProjects(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("authenticateAndGetProjects: %v", err)
	}
	if accountUUID != "acc-uuid-123" {
		t.Errorf("account UUID = %q, want %q", accountUUID, "acc-uuid-123")
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}
	if projects[0].UUID != "proj-1" || projects[0].Name != "Project One" {
		t.Errorf("first project = %+v, want {UUID:proj-1, Name:Project One}", projects[0])
	}
}

func TestAuthenticateAndGetProjects_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"account_uuid": "acc-uuid-123",
			"projects":     []projectInfo{},
		})
	}))
	defer srv.Close()

	_, projects, err := authenticateAndGetProjects(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("authenticateAndGetProjects: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}
}

func TestAuthenticateAndGetProjects_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, _, err := authenticateAndGetProjects(context.Background(), srv.Client(), srv.URL)
	if err == nil {
		t.Error("expected error for 401 response")
	}
}

func TestAuthenticateAndGetProjects_MissingAccountUUID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"projects": []projectInfo{}})
	}))
	defer srv.Close()

	_, _, err := authenticateAndGetProjects(context.Background(), srv.Client(), srv.URL)
	if err == nil {
		t.Error("expected error when account_uuid is missing")
	}
}
