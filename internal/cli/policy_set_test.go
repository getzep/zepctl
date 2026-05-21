package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/getzep/zepctl/internal/abac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Output capture helpers ---

// captureStderr redirects os.Stderr during fn and returns what was written.
// (captureStdout is defined in auth_test.go and shared across this package.)
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w
	defer func() {
		w.Close()
		os.Stderr = old
	}()

	fn()

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

// testUUID is an arbitrary valid UUID used throughout tests.
const testUUID = "9f1a2b3c-4d5e-6f7a-8b9c-0d1e2f3a4b5c"

// --- UUID Validation ---

func TestValidateUUID_Valid(t *testing.T) {
	err := validateUUID(testUUID, "policy set")
	assert.NoError(t, err)
}

func TestValidateUUID_Invalid(t *testing.T) {
	err := validateUUID("not-a-uuid", "policy set")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid policy set UUID: "not-a-uuid"`)
}

func TestValidateUUID_Empty(t *testing.T) {
	err := validateUUID("", "API key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid API key UUID`)
}

// --- ExitCodeError ---

func TestExitCodeError(t *testing.T) {
	inner := errors.New("something failed")
	err := &ExitCodeError{Code: 2, Err: inner}

	assert.Equal(t, "something failed", err.Error())
	assert.Equal(t, 2, err.Code)
	assert.True(t, errors.Is(err, inner))
}

// --- Truncate UUID ---

func TestTruncateUUID(t *testing.T) {
	assert.Equal(t, "9f1a2b3c...", truncateUUID(testUUID))
	assert.Equal(t, "short", truncateUUID("short"))
}

// --- Credential Type ---

func TestPolicySetCommands_CredentialType(t *testing.T) {
	cmds := map[string]*cobra.Command{
		"list":     policySetListCmd,
		"get":      policySetGetCmd,
		"create":   policySetCreateCmd,
		"update":   policySetUpdateCmd,
		"delete":   policySetDeleteCmd,
		"validate": policySetValidateCmd,
	}
	for name, cmd := range cmds {
		t.Run(name, func(t *testing.T) {
			require.NotNil(t, cmd.Annotations, "command %q has no annotations", name)
			assert.Equal(t, "bearer", cmd.Annotations["zepctl_credential_type"],
				"command %q should declare CredentialBearer", name)
		})
	}
}

// --- Policy Set List (Sections 3.1.2, 3.1.3) ---

func TestPolicySetList_TableOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(abac.PolicySetList{
			PolicySets: []abac.PolicySetSummary{
				{UUID: "uuid-1", Name: "policy_one", Mode: "enforce", Version: 2},
				{UUID: "uuid-2", Name: "policy_two", Mode: "off", Version: 1},
			},
		})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(policySetListCmd)
	var runErr error
	out := captureStdout(t, func() {
		runErr = cmd.RunE(cmd, nil)
	})
	assert.NoError(t, runErr)

	assert.Contains(t, out, "UUID")
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "MODE")
	assert.Contains(t, out, "VERSION")
	assert.Contains(t, out, "uuid-1")
	assert.Contains(t, out, "policy_one")
	assert.Contains(t, out, "enforce")
	assert.Contains(t, out, "uuid-2")
	assert.Contains(t, out, "policy_two")
	assert.Contains(t, out, "off")
}

func TestPolicySetList_EmptyResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(abac.PolicySetList{PolicySets: []abac.PolicySetSummary{}})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(policySetListCmd)
	var runErr error
	out := captureStdout(t, func() {
		runErr = cmd.RunE(cmd, nil)
	})
	assert.NoError(t, runErr)

	assert.Contains(t, out, "UUID")
	assert.Contains(t, out, "NAME")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	assert.Equal(t, 1, len(lines), "expected only header row for empty result")
}

// --- Policy Set Get ---

func TestPolicySetGet_TableOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(abac.PolicySet{
			UUID: testUUID,
			Name: "test_policy", Description: "A test policy",
			Mode: "report_only", Version: 3,
			ProjectUUID: "proj-uuid",
			CreatedAt:   "2026-04-15T10:30:00Z",
			UpdatedAt:   "2026-04-20T14:15:00Z",
			Spec:        map[string]any{"policies": []any{}},
		})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(policySetGetCmd)
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.RunE(cmd, []string{testUUID})
	assert.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "UUID:")
	assert.Contains(t, out, "9f1a2b3c-4d5e-6f7a-8b9c-0d1e2f3a4b5c")
	assert.Contains(t, out, "Name:")
	assert.Contains(t, out, "test_policy")
	assert.Contains(t, out, "Description:")
	assert.Contains(t, out, "A test policy")
	assert.Contains(t, out, "Mode:")
	assert.Contains(t, out, "report_only")
	assert.Contains(t, out, "Version:")
	assert.Contains(t, out, "3")
	assert.Contains(t, out, "Project:")
	assert.Contains(t, out, "proj-uuid")
	assert.Contains(t, out, "Created:")
	assert.Contains(t, out, "Updated:")
	assert.Contains(t, out, "Spec:")
	assert.Contains(t, out, "policies")
}

func TestPolicySetGet_InvalidUUID(t *testing.T) {
	err := policySetGetCmd.RunE(policySetGetCmd, []string{"not-valid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid policy set UUID: "not-valid"`)
}

// --- Policy Set Create ---

func TestPolicySetCreate_FileNotFound(t *testing.T) {
	cmd := newTestCommand(policySetCreateCmd)
	_ = cmd.Flags().Set("file", "/nonexistent/path.yaml")

	err := cmd.RunE(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading file:")
}

func TestPolicySetCreate_TableOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(abac.PolicySet{
			UUID: testUUID,
			Name: "new_policy", Mode: "off", Version: 1,
		})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(policySetCreateCmd)
	_ = cmd.Flags().Set("file", writeTestFile(t, "yaml content"))

	var runErr error
	out := captureStderr(t, func() {
		runErr = cmd.RunE(cmd, nil)
	})
	assert.NoError(t, runErr)

	assert.Contains(t, out, "Created policy set")
	assert.Contains(t, out, `"new_policy"`)
	assert.Contains(t, out, "9f1a2b3c...")
	assert.Contains(t, out, "version 1")
}

// --- Policy Set Update (Sections 3.4.2, 3.4.3, 3.4.4) ---

func TestPolicySetUpdate_InvalidUUID(t *testing.T) {
	err := policySetUpdateCmd.RunE(policySetUpdateCmd, []string{"bad-uuid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid policy set UUID: "bad-uuid"`)
}

func TestPolicySetUpdate_FileNotFound(t *testing.T) {
	cmd := newTestCommand(policySetUpdateCmd)
	_ = cmd.Flags().Set("file", "/nonexistent/path.yaml")

	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading file:")
}

func TestPolicySetUpdate_TableOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(abac.PolicySet{
			UUID: testUUID,
			Name: "updated_policy", Mode: "enforce", Version: 4,
		})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(policySetUpdateCmd)
	_ = cmd.Flags().Set("file", writeTestFile(t, "yaml content"))

	var runErr error
	out := captureStderr(t, func() {
		runErr = cmd.RunE(cmd, []string{testUUID})
	})
	assert.NoError(t, runErr)

	assert.Contains(t, out, "Updated policy set")
	assert.Contains(t, out, `"updated_policy"`)
	assert.Contains(t, out, "version 4")
}

func TestPolicySetUpdate_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "not found"})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(policySetUpdateCmd)
	_ = cmd.Flags().Set("file", writeTestFile(t, "yaml content"))

	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy set not found: 9f1a2b3c")
}

// --- Policy Set Delete (Sections 3.5.3, 3.5.4, 3.5.5) ---

func TestPolicySetDelete_NonInteractiveWithoutForce(t *testing.T) {
	// Replace stdin with a closed pipe (non-terminal, no data).
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	w.Close()
	defer func() { os.Stdin = oldStdin }()

	err := policySetDeleteCmd.RunE(policySetDeleteCmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use --force to delete without confirmation")
}

func TestPolicySetDelete_Force(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(policySetDeleteCmd)
	_ = cmd.Flags().Set("force", "true")
	defer func() { _ = policySetDeleteCmd.Flags().Set("force", "false") }()

	var runErr error
	out := captureStderr(t, func() {
		runErr = cmd.RunE(cmd, []string{testUUID})
	})
	assert.NoError(t, runErr)
	assert.Equal(t, http.MethodDelete, gotMethod)
	assert.Equal(t, "/api/v2/abac/policy-sets/"+testUUID, gotPath)
	assert.Contains(t, out, "Deleted policy set")
	assert.Contains(t, out, testUUID)
}

func TestPolicySetDelete_InvalidUUID(t *testing.T) {
	err := policySetDeleteCmd.RunE(policySetDeleteCmd, []string{"bad-uuid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid policy set UUID: "bad-uuid"`)
}

func TestPolicySetDelete_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "not found"})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(policySetDeleteCmd)
	_ = cmd.Flags().Set("force", "true")
	defer func() { _ = policySetDeleteCmd.Flags().Set("force", "false") }()

	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy set not found: 9f1a2b3c")
}

// --- Policy Set Validate (Sections 3.6.3, 3.6.4, 3.6.5) ---

// withTestABACClient replaces newABACClient with one that returns a client
// pointing at srv. Restores the original on cleanup.
func withTestABACClient(t *testing.T, srv *httptest.Server) {
	t.Helper()
	orig := newABACClient
	newABACClient = func(_ *cobra.Command) (*abac.Client, error) {
		return abac.NewClient(srv.Client(), srv.URL, "test-project"), nil
	}
	t.Cleanup(func() { newABACClient = orig })
}

func TestPolicySetValidate_FileNotFound(t *testing.T) {
	cmd := newTestCommand(policySetValidateCmd)
	_ = cmd.Flags().Set("file", "/nonexistent/path.yaml")

	err := cmd.RunE(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading file:")
	// File errors should NOT be ExitCodeError (default exit 1).
	var exitErr *ExitCodeError
	assert.False(t, errors.As(err, &exitErr))
}

func TestPolicySetValidate_TableOutput_Passed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(abac.ValidationResult{Valid: true, Errors: []abac.ValidationError{}})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(policySetValidateCmd)
	_ = cmd.Flags().Set("file", writeTestFile(t, "valid yaml"))

	var runErr error
	out := captureStderr(t, func() {
		runErr = cmd.RunE(cmd, nil)
	})
	assert.NoError(t, runErr)
	assert.Contains(t, out, "Validation passed.")
}

func TestPolicySetValidate_TableOutput_Failed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(abac.ValidationResult{
			Valid: false,
			Errors: []abac.ValidationError{
				{PolicyID: "bad_policy", Message: "unrecognized action"},
				{Message: "missing required field"},
			},
		})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(policySetValidateCmd)
	_ = cmd.Flags().Set("file", writeTestFile(t, "bad yaml"))

	var runErr error
	out := captureStderr(t, func() {
		runErr = cmd.RunE(cmd, nil)
	})
	require.Error(t, runErr)

	var exitErr *ExitCodeError
	require.True(t, errors.As(runErr, &exitErr))
	assert.Equal(t, 1, exitErr.Code)

	assert.Contains(t, out, "Validation failed:")
	assert.Contains(t, out, "unrecognized action (policy: bad_policy)")
	assert.Contains(t, out, "missing required field")
}

func TestPolicySetValidate_APIError_ExitCode2(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "server error"})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(policySetValidateCmd)
	_ = cmd.Flags().Set("file", writeTestFile(t, "yaml content"))

	err := cmd.RunE(cmd, nil)
	require.Error(t, err)
	var exitErr *ExitCodeError
	require.True(t, errors.As(err, &exitErr))
	assert.Equal(t, 2, exitErr.Code)
}

// --- Helpers ---

// newTestCommand creates a copy of cmd with a background context set.
func newTestCommand(cmd *cobra.Command) *cobra.Command {
	clone := *cmd
	clone.SetContext(context.Background())
	return &clone
}

// writeTestFile creates a temp file with the given content and returns its path.
func writeTestFile(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "policy.yaml")
	require.NoError(t, os.WriteFile(f, []byte(content), 0o600))
	return f
}
