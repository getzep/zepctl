package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/getzep/zepctl/internal/abac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testUUID2 is a second arbitrary valid UUID (testUUID is in policy_set_test.go).
const testUUID2 = "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"

// --- Credential Type ---

func TestAPIKeyCommands_CredentialType(t *testing.T) {
	cmds := map[string]*cobra.Command{
		"list":               apiKeyListCmd,
		"settings-get":       apiKeySettingsGetCmd,
		"settings-set":       apiKeySettingsSetCmd,
		"policy-sets-list":   apiKeyPolicySetsListCmd,
		"policy-sets-attach": apiKeyPolicySetsAttachCmd,
		"policy-sets-detach": apiKeyPolicySetsDetachCmd,
		"evaluate":           apiKeyEvaluateCmd,
		"explain":            apiKeyExplainCmd,
	}
	for name, cmd := range cmds {
		t.Run(name, func(t *testing.T) {
			require.NotNil(t, cmd.Annotations, "command %q has no annotations", name)
			assert.Equal(t, "bearer", cmd.Annotations["zepctl_credential_type"],
				"command %q should declare CredentialBearer", name)
		})
	}
}

// --- UUID Validation (API key commands) ---

func TestAPIKeySettingsGet_InvalidUUID(t *testing.T) {
	err := apiKeySettingsGetCmd.RunE(apiKeySettingsGetCmd, []string{"bad"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid API key UUID: "bad"`)
}

func TestAPIKeySettingsSet_InvalidUUID(t *testing.T) {
	err := apiKeySettingsSetCmd.RunE(apiKeySettingsSetCmd, []string{"bad"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid API key UUID: "bad"`)
}

func TestAPIKeyPolicySetsAttach_InvalidAPIKeyUUID(t *testing.T) {
	err := apiKeyPolicySetsAttachCmd.RunE(apiKeyPolicySetsAttachCmd,
		[]string{"bad", testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid API key UUID: "bad"`)
}

func TestAPIKeyPolicySetsAttach_InvalidPolicySetUUID(t *testing.T) {
	err := apiKeyPolicySetsAttachCmd.RunE(apiKeyPolicySetsAttachCmd,
		[]string{testUUID, "bad"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid policy set UUID: "bad"`)
}

func TestAPIKeyPolicySetsDetach_InvalidAPIKeyUUID(t *testing.T) {
	err := apiKeyPolicySetsDetachCmd.RunE(apiKeyPolicySetsDetachCmd,
		[]string{"bad", testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid API key UUID: "bad"`)
}

func TestAPIKeyPolicySetsDetach_InvalidPolicySetUUID(t *testing.T) {
	err := apiKeyPolicySetsDetachCmd.RunE(apiKeyPolicySetsDetachCmd,
		[]string{testUUID, "bad"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid policy set UUID: "bad"`)
}

// --- Mode Validation ---

func TestAPIKeySettingsSet_NoFlags(t *testing.T) {
	_ = apiKeySettingsSetCmd.Flags().Set("mode", "")
	err := apiKeySettingsSetCmd.RunE(apiKeySettingsSetCmd,
		[]string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one setting flag is required")
}

func TestAPIKeySettingsSet_InvalidMode(t *testing.T) {
	_ = apiKeySettingsSetCmd.Flags().Set("mode", "invalid")
	defer func() { _ = apiKeySettingsSetCmd.Flags().Set("mode", "") }()

	err := apiKeySettingsSetCmd.RunE(apiKeySettingsSetCmd,
		[]string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid mode "invalid": must be one of: off, report_only, enforce`)
}

func TestAPIKeySettingsSet_ValidModes(t *testing.T) {
	for _, mode := range []string{"off", "report_only", "enforce"} {
		t.Run(mode, func(t *testing.T) {
			assert.True(t, validModes[mode], "mode %q should be valid", mode)
		})
	}
}

// --- API Key Settings Get (Sections 4.1.3, 4.1.4) ---

func TestAPIKeySettingsGet_TableOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(abac.APIKeySettings{ABACMode: "enforce", Capabilities: "read"})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(apiKeySettingsGetCmd)
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.RunE(cmd, []string{testUUID})
	assert.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "ABAC Mode:")
	assert.Contains(t, out, "enforce")
	assert.Contains(t, out, "Capabilities:")
	assert.Contains(t, out, "read")
}

func TestAPIKeySettingsGet_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "not found"})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(apiKeySettingsGetCmd)
	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key not found:")
}

// --- API Key Settings Set (Sections 4.2.3, 4.2.5) ---

func TestAPIKeySettingsSet_TableOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(abac.APIKeySettings{ABACMode: "enforce", Capabilities: "read"})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(apiKeySettingsSetCmd)
	_ = cmd.Flags().Set("mode", "enforce")
	defer func() { _ = apiKeySettingsSetCmd.Flags().Set("mode", "") }()

	var runErr error
	out := captureStderr(t, func() {
		runErr = cmd.RunE(cmd, []string{testUUID})
	})
	assert.NoError(t, runErr)

	assert.Contains(t, out, "Updated API key settings:")
	assert.Contains(t, out, "ABAC Mode:")
	assert.Contains(t, out, "enforce")
	assert.Contains(t, out, "Capabilities:")
	assert.Contains(t, out, "read")
}

func TestAPIKeySettingsSet_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "not found"})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(apiKeySettingsSetCmd)
	_ = cmd.Flags().Set("mode", "enforce")
	defer func() { _ = apiKeySettingsSetCmd.Flags().Set("mode", "") }()

	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key not found:")
}

// --- API Key Policy Sets List (Sections 4.3.2, 4.3.3, 4.3.4) ---

func TestAPIKeyPolicySetsListCmd_InvalidUUID(t *testing.T) {
	err := apiKeyPolicySetsListCmd.RunE(apiKeyPolicySetsListCmd, []string{"bad"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid API key UUID: "bad"`)
}

func TestAPIKeyPolicySetsListCmd_TableOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(abac.AttachedPolicySets{
			PolicySets: []abac.PolicySetSummary{
				{UUID: "ps-1", Name: "policy_one", Mode: "enforce", Version: 2},
				{UUID: "ps-2", Name: "policy_two", Mode: "off", Version: 1},
			},
		})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(apiKeyPolicySetsListCmd)
	var runErr error
	out := captureStdout(t, func() {
		runErr = cmd.RunE(cmd, []string{testUUID})
	})
	assert.NoError(t, runErr)

	assert.Contains(t, out, "UUID")
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "MODE")
	assert.Contains(t, out, "VERSION")
	assert.Contains(t, out, "ps-1")
	assert.Contains(t, out, "policy_one")
	assert.Contains(t, out, "enforce")
	assert.Contains(t, out, "ps-2")
	assert.Contains(t, out, "policy_two")

	// Verify header + 2 data rows.
	lines := strings.Split(strings.TrimSpace(out), "\n")
	assert.Equal(t, 3, len(lines))
}

func TestAPIKeyPolicySetsListCmd_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "not found"})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(apiKeyPolicySetsListCmd)
	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key not found:")
}

// --- API Key Policy Sets Attach (Sections 4.4.3, 4.4.4) ---

func TestAPIKeyPolicySetsAttach_TableOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(abac.AttachedPolicySets{
			PolicySets: []abac.PolicySetSummary{
				{UUID: testUUID2, Name: "attached_policy", Mode: "enforce", Version: 1},
			},
		})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(apiKeyPolicySetsAttachCmd)
	var runErr error
	out := captureStderr(t, func() {
		runErr = cmd.RunE(cmd, []string{testUUID, testUUID2})
	})
	assert.NoError(t, runErr)

	assert.Contains(t, out, "Attached policy set")
	assert.Contains(t, out, `"attached_policy"`)
	assert.Contains(t, out, testUUID)
}

func TestAPIKeyPolicySetsAttach_Idempotent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(abac.AttachedPolicySets{
			PolicySets: []abac.PolicySetSummary{
				{UUID: testUUID2, Name: "policy", Mode: "enforce", Version: 1},
			},
		})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	// Call twice -- both should succeed (idempotent attach).
	for i := 0; i < 2; i++ {
		cmd := newTestCommand(apiKeyPolicySetsAttachCmd)
		captureStderr(t, func() {
			err := cmd.RunE(cmd, []string{testUUID, testUUID2})
			assert.NoError(t, err)
		})
	}
}

// --- API Key Policy Sets Detach (Sections 4.5.3, 4.5.4) ---

func TestAPIKeyPolicySetsDetach_TableOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(abac.AttachedPolicySets{PolicySets: []abac.PolicySetSummary{}})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(apiKeyPolicySetsDetachCmd)
	var runErr error
	out := captureStderr(t, func() {
		runErr = cmd.RunE(cmd, []string{testUUID, testUUID2})
	})
	assert.NoError(t, runErr)

	assert.Contains(t, out, "Detached policy set from API key")
	assert.Contains(t, out, testUUID)
}

func TestAPIKeyPolicySetsDetach_Idempotent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(abac.AttachedPolicySets{PolicySets: []abac.PolicySetSummary{}})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	// Call twice -- both should succeed (idempotent detach).
	for i := 0; i < 2; i++ {
		cmd := newTestCommand(apiKeyPolicySetsDetachCmd)
		captureStderr(t, func() {
			err := cmd.RunE(cmd, []string{testUUID, testUUID2})
			assert.NoError(t, err)
		})
	}
}

// --- API Key Evaluate ---

func TestAPIKeyEvaluate_InvalidUUID(t *testing.T) {
	err := apiKeyEvaluateCmd.RunE(apiKeyEvaluateCmd, []string{"bad"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid API key UUID: "bad"`)
}

// Locks in MarkFlagRequired so a future refactor that drops it is caught.
// Cobra enforces required flags during Execute, which the RunE-direct tests
// in this file bypass; this assertion is the cheap belt-and-braces.
func TestAPIKeyEvaluate_ActionFlagRequired(t *testing.T) {
	ann := apiKeyEvaluateCmd.Flags().Lookup("action").Annotations[cobra.BashCompOneRequiredFlag]
	require.Len(t, ann, 1)
	assert.Equal(t, "true", ann[0])
}

func TestAPIKeyEvaluate_TableOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"outcome":"allow","abac":"ALLOW","abac_shadow":"NEUTRAL","role_allows":false,"would_log_disagreement":true}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(apiKeyEvaluateCmd)
	_ = cmd.Flags().Set("action", "thread.get")
	defer func() { _ = apiKeyEvaluateCmd.Flags().Set("action", "") }()

	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.RunE(cmd, []string{testUUID})
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "Outcome:                allow")
	assert.Contains(t, out, "ABAC:                   ALLOW")
	assert.Contains(t, out, "ABAC shadow:            NEUTRAL")
	assert.Contains(t, out, "Role allows:            false")
	assert.Contains(t, out, "Would log disagreement: true")
}

// Allow/deny outcome must not change the exit code -- the operator
// asked a question and the server answered it.
func TestAPIKeyEvaluate_DenyOutcomeExitsZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"outcome":"deny","abac":"NEUTRAL","abac_shadow":"NEUTRAL","role_allows":false,"would_log_disagreement":false}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(apiKeyEvaluateCmd)
	_ = cmd.Flags().Set("action", "thread.delete")
	defer func() { _ = apiKeyEvaluateCmd.Flags().Set("action", "") }()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.RunE(cmd, []string{testUUID})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Outcome:                deny")
}

func TestAPIKeyEvaluate_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "not found"})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(apiKeyEvaluateCmd)
	_ = cmd.Flags().Set("action", "thread.get")
	defer func() { _ = apiKeyEvaluateCmd.Flags().Set("action", "") }()

	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key not found:")
}

func TestAPIKeyEvaluate_BadAction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "action is not in the registry"})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(apiKeyEvaluateCmd)
	_ = cmd.Flags().Set("action", "no.such.action")
	defer func() { _ = apiKeyEvaluateCmd.Flags().Set("action", "") }()

	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "evaluating policy:")
	assert.Contains(t, err.Error(), "action is not in the registry")
}

// --- API Key Explain ---

func TestAPIKeyExplain_InvalidUUID(t *testing.T) {
	err := apiKeyExplainCmd.RunE(apiKeyExplainCmd, []string{"bad"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid API key UUID: "bad"`)
}

func TestAPIKeyExplain_ActionFlagRequired(t *testing.T) {
	ann := apiKeyExplainCmd.Flags().Lookup("action").Annotations[cobra.BashCompOneRequiredFlag]
	require.Len(t, ann, 1)
	assert.Equal(t, "true", ann[0])
}

func TestAPIKeyExplain_TableOutput_AllowWithMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"outcome": "allow", "abac": "ALLOW", "abac_shadow": "NEUTRAL",
			"role_allows": false, "would_log_disagreement": true,
			"trace": {
				"action": "thread.get",
				"registry_entry": {"read_only": true},
				"evaluated_sets": [
					{"uuid": "9f1a2b3c-set-uuid", "name": "read_everything", "set_mode": "enforce",
					 "matched": [{"policy_id": "allow_reads", "effect": "allow", "matched_via": "readonly"}]}
				],
				"skipped_sets": []
			}
		}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(apiKeyExplainCmd)
	_ = cmd.Flags().Set("action", "thread.get")
	defer func() { _ = apiKeyExplainCmd.Flags().Set("action", "") }()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.RunE(cmd, []string{testUUID})
	require.NoError(t, err)
	out := buf.String()

	assert.Contains(t, out, "Action:                 thread.get")
	assert.Contains(t, out, "Registry read-only:     true")
	assert.Contains(t, out, "Evaluated policy sets:")
	assert.Contains(t, out, "read_everything (9f1a2b3c..., set mode: enforce)")
	assert.Contains(t, out, "    [allow] allow_reads -- matched via readonly")
	assert.Contains(t, out, "Skipped policy sets: (none)")
}

// Empty matched on an evaluated set must render the "no policies matched"
// suffix on the header.
func TestAPIKeyExplain_TableOutput_NoMatchedPolicies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"outcome": "deny", "abac": "NEUTRAL", "abac_shadow": "NEUTRAL",
			"role_allows": false, "would_log_disagreement": false,
			"trace": {
				"action": "thread.delete",
				"registry_entry": {"read_only": false},
				"evaluated_sets": [
					{"uuid": "9f1a2b3c-set-uuid", "name": "read_everything", "set_mode": "enforce", "matched": []}
				],
				"skipped_sets": []
			}
		}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(apiKeyExplainCmd)
	_ = cmd.Flags().Set("action", "thread.delete")
	defer func() { _ = apiKeyExplainCmd.Flags().Set("action", "") }()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.RunE(cmd, []string{testUUID})
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "read_everything (9f1a2b3c..., set mode: enforce) -- no policies matched")
}

// Empty evaluated_sets must render the "(none)" parallel form.
func TestAPIKeyExplain_TableOutput_NoEvaluatedSets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"outcome": "deny", "abac": "NEUTRAL", "abac_shadow": "NEUTRAL",
			"role_allows": false, "would_log_disagreement": false,
			"trace": {
				"action": "thread.get",
				"registry_entry": {"read_only": true},
				"evaluated_sets": [],
				"skipped_sets": [
					{"uuid": "1a2b3c4d-set-uuid", "name": "off_set", "reason": "set mode is off"}
				]
			}
		}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(apiKeyExplainCmd)
	_ = cmd.Flags().Set("action", "thread.get")
	defer func() { _ = apiKeyExplainCmd.Flags().Set("action", "") }()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.RunE(cmd, []string{testUUID})
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "Evaluated policy sets: (none)")
	assert.Contains(t, out, "off_set (1a2b3c4d...) -- set mode is off")
}

// An unknown trace field must not break table rendering and must be
// preserved by the JSON path (forward-compat).
func TestAPIKeyExplain_TableOutput_UnknownTraceField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"outcome": "allow", "abac": "ALLOW", "abac_shadow": "NEUTRAL",
			"role_allows": false, "would_log_disagreement": true,
			"trace": {
				"action": "thread.get",
				"registry_entry": {"read_only": true, "future_capability": "redact"},
				"future_top_level": "surprise",
				"evaluated_sets": [],
				"skipped_sets": []
			}
		}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(apiKeyExplainCmd)
	_ = cmd.Flags().Set("action", "thread.get")
	defer func() { _ = apiKeyExplainCmd.Flags().Set("action", "") }()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.RunE(cmd, []string{testUUID})
	require.NoError(t, err)
	out := buf.String()
	// Table render still succeeds with the known fields.
	assert.Contains(t, out, "Action:                 thread.get")
	assert.Contains(t, out, "Registry read-only:     true")
}

// End-to-end forward-compat: --output json must surface server-added
// fields the typed shape doesn't model. This locks in the CLI ->
// FprintRaw -> stdout path that table-only tests don't exercise.
func TestAPIKeyExplain_JSONOutput_PreservesUnknownFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"outcome": "allow", "abac": "ALLOW", "abac_shadow": "NEUTRAL",
			"role_allows": false, "would_log_disagreement": true,
			"trace": {
				"action": "thread.get",
				"registry_entry": {"read_only": true, "future_capability": "redact"},
				"future_top_level": "surprise",
				"evaluated_sets": [],
				"skipped_sets": []
			}
		}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	viper.Set("output", "json")
	defer viper.Set("output", "")

	cmd := newTestCommand(apiKeyExplainCmd)
	_ = cmd.Flags().Set("action", "thread.get")
	defer func() { _ = apiKeyExplainCmd.Flags().Set("action", "") }()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.RunE(cmd, []string{testUUID})
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, `"future_capability": "redact"`)
	assert.Contains(t, out, `"future_top_level": "surprise"`)
}
