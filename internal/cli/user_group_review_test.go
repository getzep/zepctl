package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testUUID3 = "b2c3d4e5-6f70-8192-a3b4-c5d6e7f8091a"

type sdkRequestSnapshot struct {
	method        string
	path          string
	authorization string
	project       string
	account       string
	body          map[string]any
}

func newUserGroupSDKServer(t *testing.T, status int, response string) *sdkRequestSnapshot {
	t.Helper()
	request := &sdkRequestSnapshot{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request.method = r.Method
		request.path = r.URL.Path
		request.authorization = r.Header.Get("Authorization")
		request.project = r.Header.Get("X-Zep-Project")
		request.account = r.Header.Get("X-Zep-Account-UUID")
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&request.body); err != nil {
				t.Errorf("decode request body: %v", err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(server.Close)
	configureBearerSDKTestServer(t, server)
	return request
}

func requireBearerProjectHeaders(t *testing.T, request *sdkRequestSnapshot) {
	t.Helper()
	require.Equal(t, "Bearer test-access-token", request.authorization)
	require.Equal(t, "project-test", request.project)
	require.Equal(t, "account-test", request.account)
}

func TestUserGroupList_TableOutput(t *testing.T) {
	request := newUserGroupSDKServer(t, http.StatusOK,
		`{"items":[{"uuid":"`+testUUID+`","name":"Support","version":2}]}`)
	cmd := newTestCommand(userGroupListCmd)
	var runErr error
	out := captureStdout(t, func() { runErr = cmd.RunE(cmd, nil) })
	require.NoError(t, runErr)
	assert.Contains(t, out, "UUID")
	assert.Contains(t, out, "Support")
	assert.Contains(t, out, "managed")
	assert.Equal(t, http.MethodPost, request.method)
	assert.Equal(t, "/api/v4/user-groups/list", request.path)
	requireBearerProjectHeaders(t, request)
}

func TestUserGroupGet_TableOutput(t *testing.T) {
	request := newUserGroupSDKServer(t, http.StatusOK,
		`{"uuid":"`+testUUID+`","name":"Support","description":"Support staff","version":4}`)
	cmd := newTestCommand(userGroupGetCmd)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.RunE(cmd, []string{testUUID}))
	assert.Contains(t, buf.String(), "UUID:          "+testUUID)
	assert.Contains(t, buf.String(), "Name:          Support")
	assert.Contains(t, buf.String(), "Description:   Support staff")
	assert.Contains(t, buf.String(), "Kind:          managed")
	assert.Contains(t, buf.String(), "Version:       4")
	assert.Equal(t, http.MethodGet, request.method)
	assert.Equal(t, "/api/v4/user-groups/"+testUUID, request.path)
	requireBearerProjectHeaders(t, request)
}

func TestUserGroupGet_VirtualGroup(t *testing.T) {
	request := newUserGroupSDKServer(t, http.StatusOK,
		`{"uuid":"`+testUUID+`","name":"All Users","version":null}`)
	cmd := newTestCommand(userGroupGetCmd)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.RunE(cmd, []string{testUUID}))
	assert.Contains(t, buf.String(), "Kind:          all_users")
	assert.Contains(t, buf.String(), "Version:       -")
	assert.Contains(t, buf.String(), "Created:       ")
	requireBearerProjectHeaders(t, request)
}

func TestUserGroupGet_InvalidUUID(t *testing.T) {
	err := userGroupGetCmd.RunE(userGroupGetCmd, []string{"not-valid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid user group UUID: "not-valid"`)
}

func TestUserGroupGet_NotFound(t *testing.T) {
	newUserGroupSDKServer(t, http.StatusNotFound, `{"code":"not_found","message":"not found"}`)
	cmd := newTestCommand(userGroupGetCmd)
	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting user group "+testUUID)
	assert.Contains(t, err.Error(), "not found")
}

func TestUserGroupCreate_TableOutput(t *testing.T) {
	request := newUserGroupSDKServer(t, http.StatusCreated,
		`{"uuid":"`+testUUID+`","name":"Support","version":1}`)
	cmd := newTestCommand(userGroupCreateCmd)
	setFlags(t, cmd, map[string]string{"name": "Support", "description": "Support staff"})
	var runErr error
	out := captureStderr(t, func() { runErr = cmd.RunE(cmd, nil) })
	require.NoError(t, runErr)
	assert.Contains(t, out, `Created user group "Support"`)
	assert.Contains(t, out, "9f1a2b3c...")
	assert.Equal(t, http.MethodPost, request.method)
	assert.Equal(t, "/api/v4/user-groups", request.path)
	assert.Equal(t, "Support", request.body["name"])
	assert.Equal(t, "Support staff", request.body["description"])
	requireBearerProjectHeaders(t, request)
}

func TestUserGroupCreate_NameFlagRequired(t *testing.T) {
	annotations := userGroupCreateCmd.Flags().Lookup("name").Annotations[cobra.BashCompOneRequiredFlag]
	require.Len(t, annotations, 1)
	assert.Equal(t, "true", annotations[0])
}

func TestUserGroupCreate_QuotaExhausted(t *testing.T) {
	newUserGroupSDKServer(t, http.StatusForbidden,
		`{"code":"quota_exceeded","message":"user group allocation exhausted"}`)
	cmd := newTestCommand(userGroupCreateCmd)
	setFlags(t, cmd, map[string]string{"name": "Support"})
	err := cmd.RunE(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating user group:")
	assert.Contains(t, err.Error(), "allocation exhausted")
}

func TestUserGroupUpdate_InvalidUUID(t *testing.T) {
	err := userGroupUpdateCmd.RunE(userGroupUpdateCmd, []string{"bad-uuid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid user group UUID: "bad-uuid"`)
}

func TestUserGroupUpdate_NoFieldsRequested(t *testing.T) {
	cmd := newTestCommand(userGroupUpdateCmd)
	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one of --name or --description is required")
}

func TestUserGroupUpdate_ResolvesCurrentVersion(t *testing.T) {
	var methods, paths []string
	var updateBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"uuid":"` + testUUID + `","name":"Support","version":7}`))
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&updateBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{"uuid":"` + testUUID + `","name":"Renamed","version":8}`))
	}))
	t.Cleanup(server.Close)
	configureBearerSDKTestServer(t, server)
	cmd := newTestCommand(userGroupUpdateCmd)
	setFlags(t, cmd, map[string]string{"name": "Renamed"})
	var runErr error
	out := captureStderr(t, func() { runErr = cmd.RunE(cmd, []string{testUUID}) })
	require.NoError(t, runErr)
	assert.Equal(t, []string{http.MethodGet, http.MethodPatch}, methods)
	assert.Equal(t, []string{"/api/v4/user-groups/" + testUUID, "/api/v4/user-groups/" + testUUID}, paths)
	assert.Equal(t, float64(7), updateBody["expected_version"])
	assert.Equal(t, "Renamed", updateBody["name"])
	assert.Contains(t, out, `Updated user group "Renamed" (version 8)`)
}

func TestUserGroupUpdate_ExplicitExpectedVersion(t *testing.T) {
	request := newUserGroupSDKServer(t, http.StatusOK,
		`{"uuid":"`+testUUID+`","name":"Support","version":4}`)
	cmd := newTestCommand(userGroupUpdateCmd)
	setFlags(t, cmd, map[string]string{"description": "New text", "expected-version": "3"})
	captureStderr(t, func() { require.NoError(t, cmd.RunE(cmd, []string{testUUID})) })
	assert.Equal(t, http.MethodPatch, request.method)
	assert.Equal(t, "/api/v4/user-groups/"+testUUID, request.path)
	assert.Equal(t, float64(3), request.body["expected_version"])
	assert.Equal(t, "New text", request.body["description"])
	assert.NotContains(t, request.body, "name")
	requireBearerProjectHeaders(t, request)
}

func TestUserGroupUpdate_ExplicitZeroExpectedVersion(t *testing.T) {
	request := newUserGroupSDKServer(t, http.StatusBadRequest,
		`{"code":"invalid_argument","message":"expected_version must be at least 1"}`)
	cmd := newTestCommand(userGroupUpdateCmd)
	setFlags(t, cmd, map[string]string{"name": "Renamed", "expected-version": "0"})
	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Equal(t, http.MethodPatch, request.method)
	assert.Equal(t, float64(0), request.body["expected_version"])
	assert.Contains(t, err.Error(), "updating user group")
}

func TestUserGroupUpdate_VersionConflict(t *testing.T) {
	newUserGroupSDKServer(t, http.StatusConflict,
		`{"code":"conflict","message":"user group was modified by another request"}`)
	cmd := newTestCommand(userGroupUpdateCmd)
	setFlags(t, cmd, map[string]string{"name": "Renamed", "expected-version": "3"})
	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating user group")
	assert.Contains(t, err.Error(), "modified by another request")
}

func TestUserGroupUpdate_VirtualGroupRejected(t *testing.T) {
	newUserGroupSDKServer(t, http.StatusOK,
		`{"uuid":"`+testUUID+`","name":"All Users","version":null}`)
	cmd := newTestCommand(userGroupUpdateCmd)
	setFlags(t, cmd, map[string]string{"name": "Renamed"})
	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "the All Users group is immutable")
}

func TestUserGroupDelete_InvalidUUID(t *testing.T) {
	err := userGroupDeleteCmd.RunE(userGroupDeleteCmd, []string{"bad-uuid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid user group UUID: "bad-uuid"`)
}

func TestUserGroupDelete_NonInteractiveWithoutForce(t *testing.T) {
	stubStdin(t, false, "")
	cmd := newTestCommand(userGroupDeleteCmd)
	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use --force to delete without confirmation")
}

func TestUserGroupDelete_Force(t *testing.T) {
	request := newUserGroupSDKServer(t, http.StatusNoContent, "")
	cmd := newTestCommand(userGroupDeleteCmd)
	setFlags(t, cmd, map[string]string{"force": "true"})
	var runErr error
	out := captureStderr(t, func() { runErr = cmd.RunE(cmd, []string{testUUID}) })
	require.NoError(t, runErr)
	assert.Equal(t, http.MethodDelete, request.method)
	assert.Equal(t, "/api/v4/user-groups/"+testUUID, request.path)
	assert.Contains(t, out, "Deleted user group "+testUUID)
	requireBearerProjectHeaders(t, request)
}

func TestUserGroupDelete_VirtualConflict(t *testing.T) {
	newUserGroupSDKServer(t, http.StatusConflict,
		`{"code":"conflict","message":"the All Users group cannot be deleted"}`)
	cmd := newTestCommand(userGroupDeleteCmd)
	setFlags(t, cmd, map[string]string{"force": "true"})
	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deleting user group")
	assert.Contains(t, err.Error(), "cannot be deleted")
}

func TestUserGroupMembersList_TableOutput(t *testing.T) {
	request := newUserGroupSDKServer(t, http.StatusOK,
		`{"items":[{"uuid":"`+testUUID2+`","user_id":"alice","email":"alice@example.com","first_name":"Alice","last_name":"Smith"}]}`)
	cmd := newTestCommand(userGroupMembersListCmd)
	setFlags(t, cmd, map[string]string{"search": "ali"})
	var runErr error
	out := captureStdout(t, func() { runErr = cmd.RunE(cmd, []string{testUUID}) })
	require.NoError(t, runErr)
	assert.Contains(t, out, "USER ID")
	assert.Contains(t, out, "alice")
	assert.Contains(t, out, "alice@example.com")
	assert.Contains(t, out, "Alice")
	assert.Equal(t, http.MethodPost, request.method)
	assert.Equal(t, "/api/v4/user-groups/"+testUUID+"/members/list", request.path)
	assert.Equal(t, "ali", request.body["search"])
	requireBearerProjectHeaders(t, request)
}

func TestUserGroupMembersList_InvalidUUID(t *testing.T) {
	err := userGroupMembersListCmd.RunE(userGroupMembersListCmd, []string{"bad"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid user group UUID: "bad"`)
}

func TestUserGroupMembersCandidates_TableOutput(t *testing.T) {
	request := newUserGroupSDKServer(t, http.StatusOK,
		`{"items":[{"uuid":"`+testUUID2+`","user_id":"bob"}]}`)
	cmd := newTestCommand(userGroupMembersCandidatesCmd)
	var runErr error
	out := captureStdout(t, func() { runErr = cmd.RunE(cmd, []string{testUUID}) })
	require.NoError(t, runErr)
	assert.Contains(t, out, "bob")
	assert.Equal(t, http.MethodPost, request.method)
	assert.Equal(t, "/api/v4/user-groups/"+testUUID+"/member-candidates/list", request.path)
	requireBearerProjectHeaders(t, request)
}

func TestUserGroupMembersAdd_TableOutput(t *testing.T) {
	request := newUserGroupSDKServer(t, http.StatusOK,
		`{"added_count":1,"no_op_count":1}`)
	cmd := newTestCommand(userGroupMembersAddCmd)
	var runErr error
	out := captureStderr(t, func() { runErr = cmd.RunE(cmd, []string{testUUID, testUUID2, testUUID3}) })
	require.NoError(t, runErr)
	assert.Equal(t, http.MethodPost, request.method)
	assert.Equal(t, "/api/v4/user-groups/"+testUUID+"/members", request.path)
	assert.Equal(t, []any{testUUID2, testUUID3}, request.body["user_uuids"])
	assert.Contains(t, out, "Added 1 member(s)")
	assert.Contains(t, out, "1 already a member")
	requireBearerProjectHeaders(t, request)
}

func TestUserGroupMembersRemove_UsesBulkEndpoint(t *testing.T) {
	request := newUserGroupSDKServer(t, http.StatusOK,
		`{"removed_count":1,"no_op_count":0}`)
	cmd := newTestCommand(userGroupMembersRemoveCmd)
	var runErr error
	out := captureStderr(t, func() { runErr = cmd.RunE(cmd, []string{testUUID, testUUID2}) })
	require.NoError(t, runErr)
	assert.Equal(t, http.MethodPost, request.method)
	assert.Equal(t, "/api/v4/user-groups/"+testUUID+"/members/remove", request.path)
	assert.Equal(t, []any{testUUID2}, request.body["user_uuids"])
	assert.Contains(t, out, "Removed 1 member(s)")
	requireBearerProjectHeaders(t, request)
}

func TestUserGroupMembersAdd_NotFoundKeepsServerMessage(t *testing.T) {
	newUserGroupSDKServer(t, http.StatusNotFound, `{"code":"not_found","message":"User not found"}`)
	cmd := newTestCommand(userGroupMembersAddCmd)
	err := cmd.RunE(cmd, []string{testUUID, testUUID2})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "adding members")
	assert.Contains(t, err.Error(), "User not found")
	assert.NotContains(t, err.Error(), "user group not found")
}

func TestUserGroupMembersRemove_NotFoundKeepsServerMessage(t *testing.T) {
	newUserGroupSDKServer(t, http.StatusNotFound, `{"code":"not_found","message":"User not found"}`)
	cmd := newTestCommand(userGroupMembersRemoveCmd)
	err := cmd.RunE(cmd, []string{testUUID, testUUID2})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "removing members")
	assert.Contains(t, err.Error(), "User not found")
	assert.NotContains(t, err.Error(), "user group not found")
}

func TestUserGroupMembersAdd_InvalidUserUUID(t *testing.T) {
	cmd := newTestCommand(userGroupMembersAddCmd)
	err := cmd.RunE(cmd, []string{testUUID, testUUID2, "not-a-uuid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid user UUID: "not-a-uuid"`)
}

func TestUserGroupMembersRemove_InvalidUserUUID(t *testing.T) {
	cmd := newTestCommand(userGroupMembersRemoveCmd)
	err := cmd.RunE(cmd, []string{testUUID, "not-a-uuid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid user UUID: "not-a-uuid"`)
}

func TestUserGroups_TableOutput(t *testing.T) {
	request := newUserGroupSDKServer(t, http.StatusOK,
		`{"items":[{"uuid":"`+testUUID2+`","name":"All Users","version":null},{"uuid":"`+testUUID3+`","name":"Support","version":2}]}`)
	cmd := newTestCommand(userMembershipsCmd)
	var runErr error
	out := captureStdout(t, func() { runErr = cmd.RunE(cmd, []string{testUUID}) })
	require.NoError(t, runErr)
	assert.Contains(t, out, "All Users")
	assert.Contains(t, out, "Support")
	assert.Equal(t, http.MethodGet, request.method)
	assert.Equal(t, "/api/v4/users/"+testUUID+"/user-groups", request.path)
	requireBearerProjectHeaders(t, request)
}

func TestUserGroups_InvalidUUID(t *testing.T) {
	err := userMembershipsCmd.RunE(userMembershipsCmd, []string{"bad"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid user UUID: "bad"`)
}

func TestUserGroups_NotFound(t *testing.T) {
	request := newUserGroupSDKServer(t, http.StatusNotFound,
		`{"code":"not_found","message":"user not found"}`)
	cmd := newTestCommand(userMembershipsCmd)
	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Equal(t, "/api/v4/users/"+testUUID+"/user-groups", request.path)
	assert.Contains(t, err.Error(), "listing user groups")
	assert.Contains(t, err.Error(), "user not found")
}

func TestUserGroupPolicySetsList_TableOutput(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"policy_sets":[{"uuid":"` + testUUID2 + `","name":"support_read","mode":"enforce","version":2}]}`))
	}))
	t.Cleanup(server.Close)
	withTestABACClient(t, server)
	cmd := newTestCommand(userGroupPolicySetsListCmd)
	var runErr error
	out := captureStdout(t, func() { runErr = cmd.RunE(cmd, []string{testUUID}) })
	require.NoError(t, runErr)
	assert.Contains(t, out, "MODE")
	assert.Contains(t, out, "support_read")
	assert.Equal(t, "/api/v4/user-groups/"+testUUID+"/policy-sets", gotPath)
}

func TestUserGroupPolicySetsAttach_TableOutput(t *testing.T) {
	var gotPath, gotMethod string
	var body map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"policy_sets":[{"uuid":"` + testUUID2 + `","name":"support_read","mode":"enforce","version":2}]}`))
	}))
	t.Cleanup(server.Close)
	withTestABACClient(t, server)
	cmd := newTestCommand(userGroupPolicySetsAttachCmd)
	var runErr error
	out := captureStderr(t, func() { runErr = cmd.RunE(cmd, []string{testUUID, testUUID2}) })
	require.NoError(t, runErr)
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/api/v4/user-groups/"+testUUID+"/policy-sets", gotPath)
	assert.Equal(t, testUUID2, body["policy_set_uuid"])
	assert.Contains(t, out, `Attached policy set "support_read"`)
}

func TestUserGroupPolicySetsAttach_InvalidPolicySetUUID(t *testing.T) {
	cmd := newTestCommand(userGroupPolicySetsAttachCmd)
	err := cmd.RunE(cmd, []string{testUUID, "bad"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid policy set UUID: "bad"`)
}

func TestUserGroupPolicySetsDetach_TableOutput(t *testing.T) {
	var gotPath, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"policy_sets":[]}`))
	}))
	t.Cleanup(server.Close)
	withTestABACClient(t, server)
	cmd := newTestCommand(userGroupPolicySetsDetachCmd)
	var runErr error
	out := captureStderr(t, func() { runErr = cmd.RunE(cmd, []string{testUUID, testUUID2}) })
	require.NoError(t, runErr)
	assert.Equal(t, http.MethodDelete, gotMethod)
	assert.Equal(t, "/api/v4/user-groups/"+testUUID+"/policy-sets/"+testUUID2, gotPath)
	assert.Contains(t, out, "Detached policy set from user group")
}

func TestUserGroupEvaluate_InvalidUUID(t *testing.T) {
	err := userGroupEvaluateCmd.RunE(userGroupEvaluateCmd, []string{"bad"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid user group UUID: "bad"`)
}

func TestUserGroupDryRun_ActionFlagRequired(t *testing.T) {
	for name, cmd := range map[string]*cobra.Command{
		"user-group evaluate": userGroupEvaluateCmd,
		"user-group explain":  userGroupExplainCmd,
		"user evaluate":       userEvaluateCmd,
		"user explain":        userExplainCmd,
	} {
		t.Run(name, func(t *testing.T) {
			annotation := cmd.Flags().Lookup("action").Annotations[cobra.BashCompOneRequiredFlag]
			require.Len(t, annotation, 1)
			assert.Equal(t, "true", annotation[0])
		})
	}
}

func TestUserGroupEvaluate_TableOutputWithGraph(t *testing.T) {
	var gotPath string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"outcome":"allow","abac":"ALLOW","abac_shadow":"NEUTRAL","role_allows":false,"would_log_disagreement":false,"base":"default_deny","base_allows":false,"principal":{"type":"user_group","uuid":"` + testUUID + `"},"resource":{"type":"graph","canonical_graph_uuid":"` + testUUID2 + `"}}`))
	}))
	t.Cleanup(server.Close)
	withTestABACClient(t, server)
	cmd := newTestCommand(userGroupEvaluateCmd)
	setFlags(t, cmd, map[string]string{"action": "graph.search", "graph": testUUID2})
	var out bytes.Buffer
	cmd.SetOut(&out)
	require.NoError(t, cmd.RunE(cmd, []string{testUUID}))
	assert.Equal(t, "/api/v4/abac/policies/evaluate", gotPath)
	assert.Equal(t, map[string]any{"type": "user_group", "uuid": testUUID}, body["principal"])
	assert.Equal(t, map[string]any{"type": "graph", "canonical_graph_uuid": testUUID2}, body["resource"])
	assert.Contains(t, out.String(), "Outcome:                allow")
}

func TestUserGroupEvaluate_DenyOutcomeExitsZero(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"outcome":"deny","abac":"DENY","abac_shadow":"NEUTRAL","role_allows":false,"would_log_disagreement":false,"base":"default_deny","base_allows":false}`))
	}))
	t.Cleanup(server.Close)
	withTestABACClient(t, server)
	cmd := newTestCommand(userGroupEvaluateCmd)
	setFlags(t, cmd, map[string]string{"action": "graph.search"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	require.NoError(t, cmd.RunE(cmd, []string{testUUID}))
	assert.Equal(t, "/api/v4/abac/policies/evaluate", gotPath)
	assert.Contains(t, out.String(), "Outcome:                deny")
}

func TestUserGroupEvaluate_InvalidGraphUUID(t *testing.T) {
	cmd := newTestCommand(userGroupEvaluateCmd)
	setFlags(t, cmd, map[string]string{"action": "graph.search", "graph": "not-a-uuid"})
	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid graph UUID: "not-a-uuid"`)
}

func TestUserGroupEvaluate_NotFound(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"not_found","message":"not found"}`))
	}))
	t.Cleanup(server.Close)
	withTestABACClient(t, server)
	cmd := newTestCommand(userGroupEvaluateCmd)
	setFlags(t, cmd, map[string]string{"action": "graph.search"})
	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Equal(t, "/api/v4/abac/policies/evaluate", gotPath)
	assert.Contains(t, err.Error(), "user group not found: "+testUUID)
}

func TestUserGroupEvaluate_NotFoundWithGraphKeepsServerMessage(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"not_found","message":"graph not found"}`))
	}))
	t.Cleanup(server.Close)
	withTestABACClient(t, server)
	cmd := newTestCommand(userGroupEvaluateCmd)
	setFlags(t, cmd, map[string]string{"action": "graph.search", "graph": testUUID2})
	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Equal(t, "/api/v4/abac/policies/evaluate", gotPath)
	assert.Contains(t, err.Error(), "graph not found")
	assert.NotContains(t, err.Error(), "user group not found")
}

func TestUserExplain_TableOutput_ResourceAwareSections(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"outcome":"allow","abac":"ALLOW","abac_shadow":"NEUTRAL","role_allows":false,"would_log_disagreement":false,"base":"default_deny","base_allows":false,"principal":{"type":"user","uuid":"` + testUUID + `"},"resource":{"type":"graph","canonical_graph_uuid":"` + testUUID2 + `"},"trace":{"action":"graph.search","registry_entry":{"read_only":true},"resolved_group_uuids":["` + testUUID3 + `"],"evaluated_sets":[{"uuid":"9f1a2b3c-set-uuid","name":"support_read","set_mode":"enforce","version":1,"contributing_group_uuids":["` + testUUID3 + `"],"matched":[{"policy_id":"read_graph","effect":"allow","matched_via":"literal","resource_match":"explicit"}],"skipped":[{"policy_id":"other_graph","effect":"allow","resource_match":"resource_mismatch"}]}],"skipped_sets":[],"object_constraints":{"include":[{"attributes":{"source":["email","slack"]},"subset_match":true}],"exclude":[{"attributes":{"source":["private"]},"subset_match":false}]},"dead_graph_uuids":["` + testUUID3 + `"],"future_top_level":"surprise"}}`))
	}))
	t.Cleanup(server.Close)
	withTestABACClient(t, server)
	cmd := newTestCommand(userExplainCmd)
	setFlags(t, cmd, map[string]string{"action": "graph.search", "graph": testUUID2})
	var out bytes.Buffer
	cmd.SetOut(&out)
	require.NoError(t, cmd.RunE(cmd, []string{testUUID}))
	assert.Equal(t, "/api/v4/abac/policies/explain", gotPath)
	assert.Contains(t, out.String(), "Effective UserGroups:")
	assert.Contains(t, out.String(), "support_read")
	assert.Contains(t, out.String(), "Object constraints:")
	assert.Contains(t, out.String(), "Dead graph references:")
}

func TestAPIKeyExplain_TableOutput_OmitsUserGroupSections(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"outcome":"allow","abac":"ALLOW","abac_shadow":"NEUTRAL","role_allows":false,"would_log_disagreement":true,"trace":{"action":"thread.get","registry_entry":{"read_only":true},"evaluated_sets":[{"uuid":"9f1a2b3c-set-uuid","name":"read_everything","set_mode":"enforce","matched":[{"policy_id":"allow_reads","effect":"allow","matched_via":"readonly"}]}],"skipped_sets":[]}}`))
	}))
	t.Cleanup(server.Close)
	withTestABACClient(t, server)
	cmd := newTestCommand(apiKeyExplainCmd)
	setFlags(t, cmd, map[string]string{"action": "thread.get"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	require.NoError(t, cmd.RunE(cmd, []string{testUUID}))
	assert.Equal(t, "/api/v4/abac/policies/explain", gotPath)
	assert.NotContains(t, out.String(), "Principal:")
	assert.NotContains(t, out.String(), "Effective UserGroups:")
	assert.Contains(t, out.String(), "[allow] allow_reads")
}

func TestUserEvaluate_InvalidUUID(t *testing.T) {
	err := userEvaluateCmd.RunE(userEvaluateCmd, []string{"bad"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid user UUID: "bad"`)
}

func TestUserEvaluate_NotFound(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"not_found","message":"not found"}`))
	}))
	t.Cleanup(server.Close)
	withTestABACClient(t, server)
	cmd := newTestCommand(userEvaluateCmd)
	setFlags(t, cmd, map[string]string{"action": "graph.search"})
	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Equal(t, "/api/v4/abac/policies/evaluate", gotPath)
	assert.Contains(t, err.Error(), "user not found: "+testUUID)
}

func TestUserGroupExplain_JSONOutput_PreservesUnknownFields(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"outcome":"allow","trace":{"action":"graph.search","registry_entry":{"read_only":true},"evaluated_sets":[],"skipped_sets":[],"future_group_field":"surprise"}}`))
	}))
	t.Cleanup(server.Close)
	withTestABACClient(t, server)
	viper.Set("output", "json")
	t.Cleanup(func() { viper.Set("output", "") })
	cmd := newTestCommand(userGroupExplainCmd)
	setFlags(t, cmd, map[string]string{"action": "graph.search"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	require.NoError(t, cmd.RunE(cmd, []string{testUUID}))
	assert.Equal(t, "/api/v4/abac/policies/explain", gotPath)
	assert.Contains(t, out.String(), `"future_group_field": "surprise"`)
}
