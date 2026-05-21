package abac

import (
	"encoding/json"
	"errors"
	"net/http"
)

// PolicySet is the full policy set response from the API.
type PolicySet struct {
	UUID        string         `json:"uuid"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Mode        string         `json:"mode"`
	Version     int            `json:"version"`
	Spec        map[string]any `json:"spec,omitempty"`
	ProjectUUID string         `json:"project_uuid"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
}

// PolicySetSummary is the abbreviated form returned by list endpoints.
type PolicySetSummary struct {
	UUID    string `json:"uuid"`
	Name    string `json:"name"`
	Mode    string `json:"mode"`
	Version int    `json:"version"`
}

// PolicySetList is the response for GET /api/v2/abac/policy-sets.
type PolicySetList struct {
	PolicySets []PolicySetSummary `json:"policy_sets"`
}

// AttachedPolicySets is the response for api-key policy-set list/attach/detach.
type AttachedPolicySets struct {
	PolicySets []PolicySetSummary `json:"policy_sets"`
}

// APIKeySettings is the response for api-key settings endpoints.
type APIKeySettings struct {
	ABACMode     string `json:"abac_mode"`
	Capabilities string `json:"capabilities"`
}

// ValidationResult is the response from the validate endpoint.
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors"`
}

// ValidationError is a single validation error from the server.
type ValidationError struct {
	PolicyID string `json:"policy_id,omitempty"`
	Message  string `json:"message"`
}

// ProjectKey represents an API key in the project keys list response.
type ProjectKey struct {
	UUID        string `json:"uuid"`
	ProjectUUID string `json:"project_uuid"`
	AccountUUID string `json:"account_uuid"`
	Name        string `json:"name"`
	FirstFour   string `json:"first_four"`
	LastFour    string `json:"last_four"`
	Role        string `json:"role"`
}

// ProjectKeysResponse is the response for the list API keys endpoint.
type ProjectKeysResponse struct {
	Keys []ProjectKey `json:"keys"`
}

// EvaluateResponse is the response from POST /api/v2/abac/policies/evaluate.
//
// RawJSON is the unmodified server response body. Table output uses the
// typed fields; JSON/YAML output prints RawJSON verbatim so any
// server-added fields are surfaced without a CLI release. Set by the
// client method, not by the JSON decoder.
type EvaluateResponse struct {
	Outcome              string          `json:"outcome"`
	ABAC                 string          `json:"abac"`
	ABACShadow           string          `json:"abac_shadow"`
	RoleAllows           bool            `json:"role_allows"`
	WouldLogDisagreement bool            `json:"would_log_disagreement"`
	RawJSON              json.RawMessage `json:"-"`
}

// ExplainResponse is the response from POST /api/v2/abac/policies/explain.
//
// The trace is documentation-shaped and forward-compatible: unknown fields
// inside the trace are preserved in RawJSON and surfaced in JSON/YAML
// output even if the typed shape does not yet render them.
type ExplainResponse struct {
	Outcome              string          `json:"outcome"`
	ABAC                 string          `json:"abac"`
	ABACShadow           string          `json:"abac_shadow"`
	RoleAllows           bool            `json:"role_allows"`
	WouldLogDisagreement bool            `json:"would_log_disagreement"`
	Trace                ExplainTrace    `json:"trace"`
	RawJSON              json.RawMessage `json:"-"`
}

// ExplainTrace carries the structured decision trace.
//
// role_base_covers_action is intentionally absent: the same boolean is
// already exposed at the top level as RoleAllows. The raw field is
// preserved in ExplainResponse.RawJSON for JSON callers.
type ExplainTrace struct {
	Action        string                `json:"action"`
	RegistryEntry ExplainRegistryEntry  `json:"registry_entry"`
	EvaluatedSets []ExplainEvaluatedSet `json:"evaluated_sets"`
	SkippedSets   []ExplainSkippedSet   `json:"skipped_sets"`
}

type ExplainRegistryEntry struct {
	ReadOnly bool `json:"read_only"`
}

type ExplainEvaluatedSet struct {
	UUID    string           `json:"uuid"`
	Name    string           `json:"name"`
	SetMode string           `json:"set_mode"`
	Matched []ExplainMatched `json:"matched"`
}

type ExplainMatched struct {
	PolicyID   string `json:"policy_id"`
	Effect     string `json:"effect"`
	MatchedVia string `json:"matched_via"`
}

type ExplainSkippedSet struct {
	UUID   string `json:"uuid"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// APIError is returned when the ABAC API returns a non-2xx response.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return e.Message
}

// IsNotFound reports whether err is a 404 API error.
func IsNotFound(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}

// IsConflict reports whether err is a 409 API error.
func IsConflict(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusConflict
	}
	return false
}
