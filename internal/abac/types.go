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

// PolicySetList is the response for GET /api/v4/abac/policy-sets.
type PolicySetList struct {
	PolicySets []PolicySetSummary `json:"policy_sets"`
}

// RetentionTarget is a graph ID pattern that a retention policy set is attached to.
type RetentionTarget struct {
	ProjectUUID string `json:"project_uuid" yaml:"project_uuid"`
	Pattern     string `json:"pattern" yaml:"pattern"`
}

// RetentionTargets is the target list that the retention-target endpoints return.
type RetentionTargets struct {
	Targets []RetentionTarget `json:"targets" yaml:"targets"`
}

// AttachedPolicySets is the public response for policy-set attachments.
type AttachedPolicySets struct {
	PolicySets []PolicySetSummary `json:"policy_sets"`
}

type userGroupPolicySetPage struct {
	Items      []PolicySetSummary `json:"items"`
	NextCursor string             `json:"next_cursor"`
}

type userGroupPolicySetList struct {
	Items []PolicySetSummary `json:"items"`
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

// Principal types accepted by the evaluate and explain endpoints.
const (
	PrincipalAPIKey    = "api_key"
	PrincipalUserGroup = "user_group"
	PrincipalUser      = "user"
)

// ResourceGraph is the only resource type accepted by resource-aware
// dry runs.
const ResourceGraph = "graph"

// Principal identifies the subject of a dry-run evaluation.
type Principal struct {
	Type string `json:"type"`
	UUID string `json:"uuid"`
}

// Resource identifies the object of a resource-aware dry-run evaluation.
// CanonicalGraphUUID is a standalone graph's V4 UUID, not its groups-row UUID.
type Resource struct {
	Type               string `json:"type"`
	CanonicalGraphUUID string `json:"canonical_graph_uuid"`
}

// EvaluateResponse is the response from POST /api/v4/abac/policies/evaluate.
//
// RawJSON is the unmodified server response body. Table output uses the
// typed fields; JSON/YAML output prints RawJSON verbatim so any
// server-added fields are surfaced without a CLI release. Set by the
// client method, not by the JSON decoder.
//
// Principal, Resource, Base, and BaseAllows are absent for API key
// principals and populated for user and user group principals, whose
// base posture is default-deny.
type EvaluateResponse struct {
	Outcome              string          `json:"outcome"`
	ABAC                 string          `json:"abac"`
	ABACShadow           string          `json:"abac_shadow"`
	RoleAllows           bool            `json:"role_allows"`
	WouldLogDisagreement bool            `json:"would_log_disagreement"`
	Principal            *Principal      `json:"principal,omitempty"`
	Resource             *Resource       `json:"resource,omitempty"`
	Base                 string          `json:"base,omitempty"`
	BaseAllows           *bool           `json:"base_allows,omitempty"`
	RawJSON              json.RawMessage `json:"-"`
}

// ExplainResponse is the response from POST /api/v4/abac/policies/explain.
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
	Principal            *Principal      `json:"principal,omitempty"`
	Resource             *Resource       `json:"resource,omitempty"`
	Base                 string          `json:"base,omitempty"`
	BaseAllows           *bool           `json:"base_allows,omitempty"`
	Trace                ExplainTrace    `json:"trace"`
	RawJSON              json.RawMessage `json:"-"`
}

// ExplainTrace carries the structured decision trace.
//
// role_base_covers_action is intentionally absent: the same boolean is
// already exposed at the top level as RoleAllows. The raw field is
// preserved in ExplainResponse.RawJSON for JSON callers.
type ExplainTrace struct {
	Action             string                    `json:"action"`
	RegistryEntry      ExplainRegistryEntry      `json:"registry_entry"`
	EvaluatedSets      []ExplainEvaluatedSet     `json:"evaluated_sets"`
	SkippedSets        []ExplainSkippedSet       `json:"skipped_sets"`
	Principal          *Principal                `json:"principal,omitempty"`
	Resource           *Resource                 `json:"resource,omitempty"`
	ResolvedGroupUUIDs []string                  `json:"resolved_group_uuids,omitempty"`
	ObjectConstraints  *ExplainObjectConstraints `json:"object_constraints,omitempty"`
	DeadGraphUUIDs     []string                  `json:"dead_graph_uuids,omitempty"`
}

type ExplainRegistryEntry struct {
	ReadOnly bool `json:"read_only"`
}

type ExplainEvaluatedSet struct {
	UUID                   string                 `json:"uuid"`
	Name                   string                 `json:"name"`
	SetMode                string                 `json:"set_mode"`
	Matched                []ExplainMatched       `json:"matched"`
	Skipped                []ExplainSkippedPolicy `json:"skipped,omitempty"`
	ContributingGroupUUIDs []string               `json:"contributing_group_uuids,omitempty"`
}

type ExplainMatched struct {
	PolicyID   string `json:"policy_id"`
	Effect     string `json:"effect"`
	MatchedVia string `json:"matched_via"`
	// ResourceMatch is "explicit" when the policy named the resolved
	// graph UUID and "action_wide" when it carried no selector. It is
	// empty for action-only dry runs.
	ResourceMatch string `json:"resource_match,omitempty"`
}

// ExplainSkippedPolicy is a policy that did not contribute because its
// action, resource, or set mode did not match.
type ExplainSkippedPolicy struct {
	PolicyID      string `json:"policy_id"`
	Effect        string `json:"effect"`
	ResourceMatch string `json:"resource_match"`
}

type ExplainSkippedSet struct {
	UUID   string `json:"uuid"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// ExplainObjectConstraints is the include/exclude object filter resolved
// for a resource-aware dry run.
type ExplainObjectConstraints struct {
	Include []ExplainConstraintGroup `json:"include"`
	Exclude []ExplainConstraintGroup `json:"exclude"`
}

type ExplainConstraintGroup struct {
	Attributes  map[string][]any `json:"attributes"`
	SubsetMatch bool             `json:"subset_match"`
}

// APIError is returned when the ABAC API returns a non-2xx response.
type APIError struct {
	StatusCode int
	Code       string
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
