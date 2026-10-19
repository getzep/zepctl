package abac

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient creates a Client pointing at the given test server.
func newTestClient(srv *httptest.Server) *Client {
	return NewClient(srv.Client(), srv.URL, "test-project-uuid", "test-account-uuid")
}

// --- Policy Set CRUD ---

func TestClient_ListPolicySets(t *testing.T) {
	var gotReq *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(PolicySetList{
			PolicySets: []PolicySetSummary{
				{UUID: "ps-1", Name: "policy_one", Mode: "enforce", Version: 2},
				{UUID: "ps-2", Name: "policy_two", Mode: "off", Version: 1},
			},
		})
	}))
	defer srv.Close()

	result, err := newTestClient(srv).ListPolicySets(context.Background())
	require.NoError(t, err)
	assert.Len(t, result.PolicySets, 2)
	assert.Equal(t, "policy_one", result.PolicySets[0].Name)
	assert.Equal(t, http.MethodGet, gotReq.Method)
	assert.Equal(t, "/api/v4/abac/policy-sets", gotReq.URL.Path)
	assert.Equal(t, "test-project-uuid", gotReq.Header.Get("X-Zep-Project"))
}

func TestClient_SendsAccountHeader(t *testing.T) {
	var gotReq *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(PolicySetList{})
	}))
	defer srv.Close()

	// A configured account UUID is sent as X-Zep-Account-UUID.
	_, err := NewClient(srv.Client(), srv.URL, "test-project", "acct-123").
		ListPolicySets(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "acct-123", gotReq.Header.Get("X-Zep-Account-UUID"))

	// An empty account UUID omits the header, letting the server resolve the
	// caller's default membership.
	_, err = NewClient(srv.Client(), srv.URL, "test-project", "").
		ListPolicySets(context.Background())
	require.NoError(t, err)
	assert.Empty(t, gotReq.Header.Get("X-Zep-Account-UUID"))
}

func TestClient_GetPolicySet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v4/abac/policy-sets/ps-uuid-1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(PolicySet{
			UUID: "ps-uuid-1", Name: "test_policy", Mode: "report_only", Version: 3,
			Spec: map[string]any{"policies": []any{}},
		})
	}))
	defer srv.Close()

	result, err := newTestClient(srv).GetPolicySet(context.Background(), "ps-uuid-1")
	require.NoError(t, err)
	assert.Equal(t, "test_policy", result.Name)
	assert.Equal(t, 3, result.Version)
	assert.NotNil(t, result.Spec)
}

func TestClient_CreatePolicySet(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(PolicySet{
			UUID: "new-uuid", Name: "created", Mode: "off", Version: 1,
		})
	}))
	defer srv.Close()

	result, err := newTestClient(srv).CreatePolicySet(context.Background(), "policy_set:\n  name: created\n  mode: off\n  spec: {}")
	require.NoError(t, err)
	assert.Equal(t, "created", result.Name)
	assert.Equal(t, 1, result.Version)
	assert.Contains(t, gotBody["yaml"], "policy_set:")
}

func TestClient_UpdatePolicySet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/api/v4/abac/policy-sets/ps-1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(PolicySet{
			UUID: "ps-1", Name: "updated", Mode: "enforce", Version: 4,
		})
	}))
	defer srv.Close()

	result, err := newTestClient(srv).UpdatePolicySet(context.Background(), "ps-1", "yaml content")
	require.NoError(t, err)
	assert.Equal(t, 4, result.Version)
}

func TestClient_DeletePolicySet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/api/v4/abac/policy-sets/ps-1", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	err := newTestClient(srv).DeletePolicySet(context.Background(), "ps-1")
	require.NoError(t, err)
}

func TestClient_ValidatePolicySet_Valid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v4/abac/policy-sets/validate", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ValidationResult{Valid: true, Errors: []ValidationError{}})
	}))
	defer srv.Close()

	result, err := newTestClient(srv).ValidatePolicySet(context.Background(), "yaml content")
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestClient_ValidatePolicySet_Invalid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ValidationResult{
			Valid: false,
			Errors: []ValidationError{
				{PolicyID: "bad_policy", Message: "unrecognized action"},
			},
		})
	}))
	defer srv.Close()

	result, err := newTestClient(srv).ValidatePolicySet(context.Background(), "yaml content")
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "bad_policy", result.Errors[0].PolicyID)
}

// --- Retention Targets ---

func TestClient_ListRetentionTargets(t *testing.T) {
	var gotReq *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.Empty(t, body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(RetentionTargets{
			Targets: []RetentionTarget{{ProjectUUID: "project-1", Pattern: "scratch-*"}},
		})
	}))
	defer srv.Close()

	result, err := newTestClient(srv).ListRetentionTargets(context.Background(), "policy-set-1")
	require.NoError(t, err)
	require.Len(t, result.Targets, 1)
	assert.Equal(t, "project-1", result.Targets[0].ProjectUUID)
	assert.Equal(t, "scratch-*", result.Targets[0].Pattern)
	assert.Equal(t, http.MethodGet, gotReq.Method)
	assert.Equal(t, "/api/v4/abac/policy-sets/policy-set-1/retention-targets", gotReq.URL.Path)
	assert.Equal(t, "test-project-uuid", gotReq.Header.Get("X-Zep-Project"))
}

func TestClient_AttachRetentionTarget(t *testing.T) {
	var gotReq *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"pattern":"scratch-*"}`, string(body))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(RetentionTargets{
			Targets: []RetentionTarget{{ProjectUUID: "project-1", Pattern: "scratch-*"}},
		})
	}))
	defer srv.Close()

	result, err := newTestClient(srv).AttachRetentionTarget(context.Background(), "policy-set-1", "scratch-*")
	require.NoError(t, err)
	require.Len(t, result.Targets, 1)
	assert.Equal(t, "scratch-*", result.Targets[0].Pattern)
	assert.Equal(t, http.MethodPost, gotReq.Method)
	assert.Equal(t, "/api/v4/abac/policy-sets/policy-set-1/retention-targets", gotReq.URL.Path)
	assert.Equal(t, "test-project-uuid", gotReq.Header.Get("X-Zep-Project"))
}

func TestClient_DetachRetentionTarget(t *testing.T) {
	var gotReq *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"pattern":"scratch-*"}`, string(body))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(RetentionTargets{
			Targets: []RetentionTarget{{ProjectUUID: "project-1", Pattern: "other-*"}},
		})
	}))
	defer srv.Close()

	result, err := newTestClient(srv).DetachRetentionTarget(context.Background(), "policy-set-1", "scratch-*")
	require.NoError(t, err)
	require.Len(t, result.Targets, 1)
	assert.Equal(t, "other-*", result.Targets[0].Pattern)
	assert.Equal(t, http.MethodDelete, gotReq.Method)
	assert.Equal(t, "/api/v4/abac/policy-sets/policy-set-1/retention-targets", gotReq.URL.Path)
	assert.Equal(t, "test-project-uuid", gotReq.Header.Get("X-Zep-Project"))
}

func TestClient_RetentionTargetsEmptyList(t *testing.T) {
	for _, response := range []string{`{"targets":null}`, `{}`} {
		t.Run(response, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(response))
			}))
			defer srv.Close()

			result, err := newTestClient(srv).ListRetentionTargets(context.Background(), "policy-set-1")
			require.NoError(t, err)
			require.NotNil(t, result.Targets)
			assert.Empty(t, result.Targets)
		})
	}
}

func TestClient_ListRetentionTargets_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"policy set not found"}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).ListRetentionTargets(context.Background(), "missing")
	require.Error(t, err)
	assert.True(t, IsNotFound(err))
}

// --- API Key List ---

func TestClient_ListAPIKeys(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v4/abac/api-keys", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "test-project-uuid", r.Header.Get("X-Zep-Project"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ProjectKeysResponse{
			Keys: []ProjectKey{
				{UUID: "key-1", Name: "prod", FirstFour: "z_1d", LastFour: "a4be", Role: "default_allow"},
			},
		})
	}))
	defer srv.Close()

	result, err := newTestClient(srv).ListAPIKeys(context.Background())
	require.NoError(t, err)
	assert.Len(t, result.Keys, 1)
	assert.Equal(t, "key-1", result.Keys[0].UUID)
	assert.Equal(t, "z_1d", result.Keys[0].FirstFour)
	assert.Equal(t, "default_allow", result.Keys[0].Role)
}

// --- API Key Settings ---

func TestClient_GetAPIKeySettings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v4/abac/api-keys/key-1/settings", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(APIKeySettings{ABACMode: "off", Capabilities: "read_write"})
	}))
	defer srv.Close()

	result, err := newTestClient(srv).GetAPIKeySettings(context.Background(), "key-1")
	require.NoError(t, err)
	assert.Equal(t, "off", result.ABACMode)
	assert.Equal(t, "read_write", result.Capabilities)
}

func TestClient_SetAPIKeySettings(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(APIKeySettings{ABACMode: "enforce", Capabilities: "read_write"})
	}))
	defer srv.Close()

	result, err := newTestClient(srv).SetAPIKeySettings(context.Background(), "key-1", "enforce")
	require.NoError(t, err)
	assert.Equal(t, "enforce", result.ABACMode)
	assert.Equal(t, "enforce", gotBody["abac_mode"])
}

// --- API Key Policy Attachments ---

func TestClient_ListAPIKeyPolicySets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v4/abac/api-keys/key-1/policy-sets", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(AttachedPolicySets{
			PolicySets: []PolicySetSummary{{UUID: "ps-1", Name: "test", Mode: "enforce", Version: 1}},
		})
	}))
	defer srv.Close()

	result, err := newTestClient(srv).ListAPIKeyPolicySets(context.Background(), "key-1")
	require.NoError(t, err)
	assert.Len(t, result.PolicySets, 1)
}

func TestClient_AttachPolicySet(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v4/abac/api-keys/key-1/policy-sets", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(AttachedPolicySets{
			PolicySets: []PolicySetSummary{{UUID: "ps-1", Name: "attached", Mode: "enforce", Version: 1}},
		})
	}))
	defer srv.Close()

	result, err := newTestClient(srv).AttachPolicySet(context.Background(), "key-1", "ps-1")
	require.NoError(t, err)
	assert.Len(t, result.PolicySets, 1)
	assert.Equal(t, "ps-1", gotBody["policy_set_uuid"])
}

func TestClient_DetachPolicySet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/api/v4/abac/api-keys/key-1/policy-sets/ps-1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(AttachedPolicySets{PolicySets: []PolicySetSummary{}})
	}))
	defer srv.Close()

	result, err := newTestClient(srv).DetachPolicySet(context.Background(), "key-1", "ps-1")
	require.NoError(t, err)
	assert.Empty(t, result.PolicySets)
}

// --- Policy Evaluation (Dry-Run) ---

func TestClient_EvaluatePolicy(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v4/abac/policies/evaluate", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "test-project-uuid", r.Header.Get("X-Zep-Project"))
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"outcome":"allow","abac":"ALLOW","abac_shadow":"NEUTRAL","role_allows":false,"would_log_disagreement":true}`))
	}))
	defer srv.Close()

	result, err := newTestClient(srv).EvaluatePolicy(
		context.Background(), Principal{Type: PrincipalAPIKey, UUID: "key-1"}, "thread.get", nil,
	)
	require.NoError(t, err)
	assert.Equal(t, "allow", result.Outcome)
	assert.Equal(t, "ALLOW", result.ABAC)
	assert.Equal(t, "NEUTRAL", result.ABACShadow)
	assert.False(t, result.RoleAllows)
	assert.True(t, result.WouldLogDisagreement)
	assert.Equal(t, "key-1", gotBody["api_key_uuid"])
	assert.Equal(t, "thread.get", gotBody["action"])
	// RawJSON preserves the server response body verbatim.
	assert.Contains(t, string(result.RawJSON), `"outcome":"allow"`)
}

// An API key principal keeps the original api_key_uuid field and never
// sends a typed principal or a resource.
func TestClient_EvaluatePolicy_APIKeyBodyShape(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"outcome":"deny"}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).EvaluatePolicy(
		context.Background(), Principal{Type: PrincipalAPIKey, UUID: "key-1"}, "thread.get", nil,
	)
	require.NoError(t, err)
	assert.Equal(t, "key-1", gotBody["api_key_uuid"])
	assert.NotContains(t, gotBody, "principal")
	assert.NotContains(t, gotBody, "resource")
}

// User and user group principals use the typed principal field, and the
// optional graph resource is sent when supplied.
func TestClient_EvaluatePolicy_TypedPrincipalAndResource(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"outcome": "allow", "abac": "ALLOW", "abac_shadow": "NEUTRAL",
			"role_allows": false, "would_log_disagreement": false,
			"base": "default_deny", "base_allows": false,
			"principal": {"type": "user", "uuid": "user-1"},
			"resource": {"type": "graph", "canonical_graph_uuid": "graph-1"}
		}`))
	}))
	defer srv.Close()

	result, err := newTestClient(srv).EvaluatePolicy(
		context.Background(),
		Principal{Type: PrincipalUser, UUID: "user-1"},
		"graph.search",
		&Resource{Type: ResourceGraph, CanonicalGraphUUID: "graph-1"},
	)
	require.NoError(t, err)
	assert.NotContains(t, gotBody, "api_key_uuid")
	assert.Equal(t, map[string]any{"type": "user", "uuid": "user-1"}, gotBody["principal"])
	assert.Equal(t, map[string]any{"type": "graph", "canonical_graph_uuid": "graph-1"}, gotBody["resource"])
	require.NotNil(t, result.Principal)
	assert.Equal(t, PrincipalUser, result.Principal.Type)
	require.NotNil(t, result.Resource)
	assert.Equal(t, "graph-1", result.Resource.CanonicalGraphUUID)
	assert.Equal(t, "default_deny", result.Base)
	require.NotNil(t, result.BaseAllows)
	assert.False(t, *result.BaseAllows)
}

// A user group principal omits the resource when none is requested.
func TestClient_EvaluatePolicy_UserGroupPrincipalWithoutResource(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"outcome":"deny"}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).EvaluatePolicy(
		context.Background(), Principal{Type: PrincipalUserGroup, UUID: "group-1"}, "graph.search", nil,
	)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"type": "user_group", "uuid": "group-1"}, gotBody["principal"])
	assert.NotContains(t, gotBody, "resource")
}

// EvaluatePolicy preserves server-added fields the typed shape doesn't
// model, for forward compatibility.
func TestClient_EvaluatePolicy_PreservesUnknownFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"outcome":"deny","abac":"NEUTRAL","abac_shadow":"NEUTRAL","role_allows":false,"would_log_disagreement":false,"future_field":"surprise"}`))
	}))
	defer srv.Close()

	result, err := newTestClient(srv).EvaluatePolicy(
		context.Background(), Principal{Type: PrincipalAPIKey, UUID: "key-1"}, "thread.delete", nil,
	)
	require.NoError(t, err)
	assert.Equal(t, "deny", result.Outcome)
	assert.Contains(t, string(result.RawJSON), `"future_field":"surprise"`)
}

func TestClient_ExplainPolicy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v4/abac/policies/explain", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"outcome": "allow",
			"abac": "ALLOW",
			"abac_shadow": "NEUTRAL",
			"role_allows": false,
			"would_log_disagreement": true,
			"trace": {
				"action": "thread.get",
				"registry_entry": {"read_only": true},
				"role_base_covers_action": false,
				"evaluated_sets": [
					{"uuid": "set-1", "name": "read_everything", "set_mode": "enforce",
					 "matched": [{"policy_id": "allow_reads", "effect": "allow", "matched_via": "readonly"}]}
				],
				"skipped_sets": []
			}
		}`))
	}))
	defer srv.Close()

	result, err := newTestClient(srv).ExplainPolicy(
		context.Background(), Principal{Type: PrincipalAPIKey, UUID: "key-1"}, "thread.get", nil,
	)
	require.NoError(t, err)
	assert.Equal(t, "ALLOW", result.ABAC)
	assert.Equal(t, "thread.get", result.Trace.Action)
	assert.True(t, result.Trace.RegistryEntry.ReadOnly)
	require.Len(t, result.Trace.EvaluatedSets, 1)
	assert.Equal(t, "read_everything", result.Trace.EvaluatedSets[0].Name)
	require.Len(t, result.Trace.EvaluatedSets[0].Matched, 1)
	assert.Equal(t, "readonly", result.Trace.EvaluatedSets[0].Matched[0].MatchedVia)
	assert.Empty(t, result.Trace.SkippedSets)
	// role_base_covers_action is intentionally not in the typed shape but is
	// retained in RawJSON.
	assert.Contains(t, string(result.RawJSON), `"role_base_covers_action": false`)
}

// A resource-aware explain carries effective groups, contributing
// groups, resource matches, skipped policies, object constraints, and
// dead graph references.
func TestClient_ExplainPolicy_ResourceAware(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v4/abac/policies/explain", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"outcome": "allow",
			"abac": "ALLOW",
			"abac_shadow": "NEUTRAL",
			"role_allows": false,
			"would_log_disagreement": false,
			"base": "default_deny",
			"base_allows": false,
			"principal": {"type": "user", "uuid": "user-1"},
			"resource": {"type": "graph", "canonical_graph_uuid": "graph-1"},
			"trace": {
				"action": "graph.search",
				"registry_entry": {"read_only": true},
				"resolved_group_uuids": ["group-all", "group-support"],
				"evaluated_sets": [
					{"uuid": "set-1", "name": "support_read", "set_mode": "enforce",
					 "contributing_group_uuids": ["group-support"],
					 "matched": [{"policy_id": "read_graph", "effect": "allow",
					   "matched_via": "literal", "resource_match": "explicit"}],
					 "skipped": [{"policy_id": "other_graph", "effect": "allow",
					   "resource_match": "resource_mismatch"}]}
				],
				"skipped_sets": [],
				"object_constraints": {
					"include": [{"attributes": {"source": ["email"]}, "subset_match": true}],
					"exclude": []
				},
				"dead_graph_uuids": ["graph-gone"]
			}
		}`))
	}))
	defer srv.Close()

	result, err := newTestClient(srv).ExplainPolicy(
		context.Background(),
		Principal{Type: PrincipalUser, UUID: "user-1"},
		"graph.search",
		&Resource{Type: ResourceGraph, CanonicalGraphUUID: "graph-1"},
	)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"type": "graph", "canonical_graph_uuid": "graph-1"}, gotBody["resource"])
	assert.Equal(t, "default_deny", result.Base)
	assert.Equal(t, []string{"group-all", "group-support"}, result.Trace.ResolvedGroupUUIDs)
	require.Len(t, result.Trace.EvaluatedSets, 1)
	set := result.Trace.EvaluatedSets[0]
	assert.Equal(t, []string{"group-support"}, set.ContributingGroupUUIDs)
	require.Len(t, set.Matched, 1)
	assert.Equal(t, "explicit", set.Matched[0].ResourceMatch)
	require.Len(t, set.Skipped, 1)
	assert.Equal(t, "resource_mismatch", set.Skipped[0].ResourceMatch)
	require.NotNil(t, result.Trace.ObjectConstraints)
	require.Len(t, result.Trace.ObjectConstraints.Include, 1)
	assert.True(t, result.Trace.ObjectConstraints.Include[0].SubsetMatch)
	assert.Equal(t, []string{"graph-gone"}, result.Trace.DeadGraphUUIDs)
}

// --- User Groups ---

func TestClient_ListUserGroupPolicySets(t *testing.T) {
	var requestCount int
	var cursors []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v4/user-groups/group-1/policy-sets", r.URL.Path)
		assert.Equal(t, "200", r.URL.Query().Get("page_size"))
		requestCount++
		cursors = append(cursors, r.URL.Query().Get("cursor"))
		w.Header().Set("Content-Type", "application/json")
		if requestCount == 1 {
			_, _ = w.Write([]byte(`{"items":[{"uuid":"ps-1","name":"support_read","mode":"enforce","version":1}],"next_cursor":"c1"}`))
			return
		}
		_, _ = w.Write([]byte(`{"items":[{"uuid":"ps-2","name":"support_write","mode":"report_only","version":2}]}`))
	}))
	defer srv.Close()

	result, err := newTestClient(srv).ListUserGroupPolicySets(context.Background(), "group-1")
	require.NoError(t, err)
	require.Len(t, result.PolicySets, 2)
	assert.Equal(t, "support_read", result.PolicySets[0].Name)
	assert.Equal(t, "support_write", result.PolicySets[1].Name)
	assert.Equal(t, []string{"", "c1"}, cursors)
	assert.Equal(t, 2, requestCount)
}

func TestClient_ListUserGroupPolicySets_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "200", r.URL.Query().Get("page_size"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	result, err := newTestClient(srv).ListUserGroupPolicySets(context.Background(), "group-1")
	require.NoError(t, err)
	require.NotNil(t, result.PolicySets)
	assert.Empty(t, result.PolicySets)
}

func TestClient_AttachUserGroupPolicySet(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v4/user-groups/group-1/policy-sets", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"uuid":"ps-1","name":"support_read","mode":"enforce","version":1}]}`))
	}))
	defer srv.Close()

	result, err := newTestClient(srv).AttachUserGroupPolicySet(context.Background(), "group-1", "ps-1")
	require.NoError(t, err)
	require.Len(t, result.PolicySets, 1)
	assert.Equal(t, "support_read", result.PolicySets[0].Name)
	assert.Equal(t, "ps-1", gotBody["policy_set_uuid"])
}

func TestClient_DetachUserGroupPolicySet(t *testing.T) {
	var requestCount int
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	err := newTestClient(srv).DetachUserGroupPolicySet(context.Background(), "group-1", "ps-1")
	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, gotMethod)
	assert.Equal(t, "/api/v4/user-groups/group-1/policy-sets/ps-1", gotPath)
	assert.Equal(t, 1, requestCount)
}

// A retention policy set cannot attach to a user group.
func TestClient_AttachUserGroupPolicySet_BadRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "retention policy sets cannot attach to a user group"})
	}))
	defer srv.Close()

	_, err := newTestClient(srv).AttachUserGroupPolicySet(context.Background(), "group-1", "ps-1")
	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Contains(t, apiErr.Message, "retention policy sets")
}

// --- Error Handling ---

func TestClient_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "policy set not found"})
	}))
	defer srv.Close()

	_, err := newTestClient(srv).GetPolicySet(context.Background(), "nonexistent")
	require.Error(t, err)
	assert.True(t, IsNotFound(err))
	assert.Contains(t, err.Error(), "policy set not found")
}

func TestClient_Conflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "duplicate name"})
	}))
	defer srv.Close()

	_, err := newTestClient(srv).CreatePolicySet(context.Background(), "yaml")
	require.Error(t, err)
	assert.True(t, IsConflict(err))
}

func TestClient_BadRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "invalid yaml"})
	}))
	defer srv.Close()

	_, err := newTestClient(srv).CreatePolicySet(context.Background(), "bad")
	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
}

func TestClient_ErrorWithoutBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := newTestClient(srv).ListPolicySets(context.Background())
	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	assert.Contains(t, apiErr.Message, "HTTP 500")
}

func TestDecodeError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantCode   string
		wantMsg    string
	}{
		{
			name:       "v4 error envelope",
			statusCode: http.StatusNotFound,
			body:       `{"error":{"code":"not_found","message":"graph not found","request_id":"request-1"}}`,
			wantCode:   "not_found",
			wantMsg:    "graph not found",
		},
		{
			name:       "top-level error",
			statusCode: http.StatusBadRequest,
			body:       `{"code":"bad_request","message":"invalid resource"}`,
			wantCode:   "bad_request",
			wantMsg:    "invalid resource",
		},
		{
			name:       "non-JSON body",
			statusCode: http.StatusInternalServerError,
			body:       "upstream error",
			wantMsg:    "API error (HTTP 500)",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := decodeError(test.statusCode, []byte(test.body))
			var apiErr *APIError
			require.ErrorAs(t, err, &apiErr)
			assert.Equal(t, test.statusCode, apiErr.StatusCode)
			assert.Equal(t, test.wantCode, apiErr.Code)
			assert.Equal(t, test.wantMsg, apiErr.Message)
		})
	}
}

func TestIsNotFound_NonAPIError(t *testing.T) {
	assert.False(t, IsNotFound(assert.AnError))
}

func TestIsConflict_NonAPIError(t *testing.T) {
	assert.False(t, IsConflict(assert.AnError))
}
