package abac

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client provides methods for ABAC API endpoints. The caller obtains
// httpClient from client.NewBearerHTTPClient (which handles token
// refresh), projectUUID from config.GetProjectUUID, and accountUUID from
// config.GetAccountUUID.
type Client struct {
	HTTP        *http.Client
	BaseURL     string
	ProjectUUID string
	AccountUUID string
}

// NewClient creates an ABAC client. accountUUID may be empty, in which case
// no X-Zep-Account-UUID header is sent and the server resolves the caller's
// default membership.
func NewClient(httpClient *http.Client, baseURL, projectUUID, accountUUID string) *Client {
	return &Client{
		HTTP:        httpClient,
		BaseURL:     strings.TrimRight(baseURL, "/"),
		ProjectUUID: projectUUID,
		AccountUUID: accountUUID,
	}
}

// --- Policy Set CRUD ---

func (c *Client) ListPolicySets(ctx context.Context) (*PolicySetList, error) {
	var result PolicySetList
	if err := c.doRequest(ctx, http.MethodGet, "/api/v2/abac/policy-sets", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetPolicySet(ctx context.Context, uuid string) (*PolicySet, error) {
	var result PolicySet
	if err := c.doRequest(ctx, http.MethodGet, "/api/v2/abac/policy-sets/"+uuid, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) CreatePolicySet(ctx context.Context, yamlContent string) (*PolicySet, error) {
	body := map[string]string{"yaml": yamlContent}
	var result PolicySet
	if err := c.doRequest(ctx, http.MethodPost, "/api/v2/abac/policy-sets", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) UpdatePolicySet(ctx context.Context, uuid, yamlContent string) (*PolicySet, error) {
	body := map[string]string{"yaml": yamlContent}
	var result PolicySet
	if err := c.doRequest(ctx, http.MethodPatch, "/api/v2/abac/policy-sets/"+uuid, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeletePolicySet(ctx context.Context, uuid string) error {
	return c.doRequest(ctx, http.MethodDelete, "/api/v2/abac/policy-sets/"+uuid, nil, nil)
}

func (c *Client) ValidatePolicySet(ctx context.Context, yamlContent string) (*ValidationResult, error) {
	body := map[string]string{"yaml": yamlContent}
	var result ValidationResult
	if err := c.doRequest(ctx, http.MethodPost, "/api/v2/abac/policy-sets/validate", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// --- API Key Settings ---

func (c *Client) GetAPIKeySettings(ctx context.Context, keyUUID string) (*APIKeySettings, error) {
	var result APIKeySettings
	path := fmt.Sprintf("/api/v2/abac/api-keys/%s/settings", keyUUID)
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) SetAPIKeySettings(ctx context.Context, keyUUID, mode string) (*APIKeySettings, error) {
	body := map[string]string{"abac_mode": mode}
	var result APIKeySettings
	path := fmt.Sprintf("/api/v2/abac/api-keys/%s/settings", keyUUID)
	if err := c.doRequest(ctx, http.MethodPatch, path, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// --- API Key List ---

// ListAPIKeys returns the API keys for the current project.
func (c *Client) ListAPIKeys(ctx context.Context) (*ProjectKeysResponse, error) {
	var result ProjectKeysResponse
	if err := c.doRequest(ctx, http.MethodGet, "/api/v2/abac/api-keys", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// --- API Key Policy Attachments ---

func (c *Client) ListAPIKeyPolicySets(ctx context.Context, keyUUID string) (*AttachedPolicySets, error) {
	var result AttachedPolicySets
	path := fmt.Sprintf("/api/v2/abac/api-keys/%s/policy-sets", keyUUID)
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) AttachPolicySet(ctx context.Context, keyUUID, policySetUUID string) (*AttachedPolicySets, error) {
	body := map[string]string{"policy_set_uuid": policySetUUID}
	var result AttachedPolicySets
	path := fmt.Sprintf("/api/v2/abac/api-keys/%s/policy-sets", keyUUID)
	if err := c.doRequest(ctx, http.MethodPost, path, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DetachPolicySet(ctx context.Context, keyUUID, policySetUUID string) (*AttachedPolicySets, error) {
	var result AttachedPolicySets
	path := fmt.Sprintf("/api/v2/abac/api-keys/%s/policy-sets/%s", keyUUID, policySetUUID)
	if err := c.doRequest(ctx, http.MethodDelete, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// --- Policy Evaluation (Dry-Run) ---

// EvaluatePolicy posts to /api/v2/abac/policies/evaluate and returns the
// concise decision plus the unmodified server body in RawJSON.
func (c *Client) EvaluatePolicy(ctx context.Context, keyUUID, action string) (*EvaluateResponse, error) {
	body := map[string]string{"api_key_uuid": keyUUID, "action": action}
	raw, err := c.doRequestRaw(ctx, http.MethodPost, "/api/v2/abac/policies/evaluate", body)
	if err != nil {
		return nil, err
	}
	var result EvaluateResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	result.RawJSON = raw
	return &result, nil
}

// ExplainPolicy posts to /api/v2/abac/policies/explain and returns the
// decision plus a structured trace. RawJSON preserves any
// trace fields the typed shape does not yet render.
func (c *Client) ExplainPolicy(ctx context.Context, keyUUID, action string) (*ExplainResponse, error) {
	body := map[string]string{"api_key_uuid": keyUUID, "action": action}
	raw, err := c.doRequestRaw(ctx, http.MethodPost, "/api/v2/abac/policies/explain", body)
	if err != nil {
		return nil, err
	}
	var result ExplainResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	result.RawJSON = raw
	return &result, nil
}

// --- Internal ---

func (c *Client) doRequest(ctx context.Context, method, path string, body, result any) error {
	raw, err := c.doRequestRaw(ctx, method, path, body)
	if err != nil {
		return err
	}
	if len(raw) == 0 || result == nil {
		return nil
	}
	if err := json.Unmarshal(raw, result); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

func (c *Client) doRequestRaw(ctx context.Context, method, path string, body any) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.ProjectUUID != "" {
		req.Header.Set("X-Zep-Project", c.ProjectUUID)
	}
	if c.AccountUUID != "" {
		req.Header.Set("X-Zep-Account-UUID", c.AccountUUID)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, decodeError(resp.StatusCode, raw)
	}
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	return raw, nil
}

func decodeError(statusCode int, raw []byte) error {
	var errResp struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &errResp); err != nil || errResp.Message == "" {
		return &APIError{
			StatusCode: statusCode,
			Message:    fmt.Sprintf("API error (HTTP %d)", statusCode),
		}
	}
	return &APIError{
		StatusCode: statusCode,
		Message:    errResp.Message,
	}
}
