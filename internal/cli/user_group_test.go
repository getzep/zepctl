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

// testUUID3 is a third arbitrary valid UUID, used as a member UUID.
const testUUID3 = "b2c3d4e5-6f7a-8b9c-0d1e-2f3a4b5c6d7e"

// setFlags sets flags on cmd and restores their defaults, including the
// Changed bit, on cleanup. The command objects are package-level, so a
// leaked Changed flag would alter later tests.
func setFlags(t *testing.T, cmd *cobra.Command, values map[string]string) {
	t.Helper()
	for name, value := range values {
		flag := cmd.Flags().Lookup(name)
		require.NotNil(t, flag, "unknown flag %q", name)
		require.NoError(t, flag.Value.Set(value))
		flag.Changed = true
		t.Cleanup(func() {
			require.NoError(t, flag.Value.Set(flag.DefValue))
			flag.Changed = false
		})
	}
}

// --- Credential Type ---

func TestUserGroupCommands_CredentialType(t *testing.T) {
	cmds := map[string]*cobra.Command{
		"list":               userGroupListCmd,
		"get":                userGroupGetCmd,
		"create":             userGroupCreateCmd,
		"update":             userGroupUpdateCmd,
		"delete":             userGroupDeleteCmd,
		"members list":       userGroupMembersListCmd,
		"members candidates": userGroupMembersCandidatesCmd,
		"members add":        userGroupMembersAddCmd,
		"members remove":     userGroupMembersRemoveCmd,
		"policy-sets list":   userGroupPolicySetsListCmd,
		"policy-sets attach": userGroupPolicySetsAttachCmd,
		"policy-sets detach": userGroupPolicySetsDetachCmd,
		"evaluate":           userGroupEvaluateCmd,
		"explain":            userGroupExplainCmd,
		"user groups":        userMembershipsCmd,
		"user evaluate":      userEvaluateCmd,
		"user explain":       userExplainCmd,
	}
	for name, cmd := range cmds {
		t.Run(name, func(t *testing.T) {
			require.NotNil(t, cmd.Annotations, "command %q has no annotations", name)
			assert.Equal(t, "bearer", cmd.Annotations["zepctl_credential_type"],
				"command %q should declare CredentialBearer", name)
		})
	}
}

// --- List ---

func TestUserGroupList_TableOutput(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"user_groups": [
				{"uuid": "group-all", "name": "All Users", "kind": "virtual",
				 "member_count": 12, "attached_policy_set_count": 1, "version": null},
				{"uuid": "group-1", "name": "Support", "kind": "managed",
				 "member_count": 3, "attached_policy_set_count": 2, "version": 4}
			],
			"total_count": 2, "row_count": 2,
			"quota": {"active_managed": 1, "allocation": 10, "has_feature": true, "resolved": true}
		}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupListCmd)
	var runErr error
	out := captureStdout(t, func() {
		runErr = cmd.RunE(cmd, nil)
	})
	require.NoError(t, runErr)

	assert.Contains(t, out, "UUID")
	assert.Contains(t, out, "KIND")
	assert.Contains(t, out, "MEMBERS")
	assert.Contains(t, out, "POLICY SETS")
	assert.Contains(t, out, "All Users")
	assert.Contains(t, out, "virtual")
	assert.Contains(t, out, "Support")
	assert.Contains(t, out, "managed")
	// The whole row, because a bare "-" is also in every fixture UUID: a
	// null version renders as "-" and a real one as the number.
	assert.Regexp(t, `All Users\s+virtual\s+12\s+1\s+-\s*\n`, out)
	assert.Regexp(t, `Support\s+managed\s+3\s+2\s+4\s*\n`, out)
	// pageSize is required server-side, so it is always sent.
	assert.Contains(t, gotQuery, "pageSize=50")
	assert.Contains(t, gotQuery, "pageNumber=1")
}

// --- Get ---

func TestUserGroupGet_TableOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"uuid": "9f1a2b3c-4d5e-6f7a-8b9c-0d1e2f3a4b5c",
			"project_uuid": "proj-uuid",
			"name": "Support", "description": "Support staff", "kind": "managed",
			"member_count": 3, "attached_policy_set_count": 2, "version": 4,
			"created_at": "2026-04-15T10:30:00Z", "updated_at": "2026-04-20T14:15:00Z"
		}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupGetCmd)
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	require.NoError(t, cmd.RunE(cmd, []string{testUUID}))
	out := buf.String()
	assert.Contains(t, out, "UUID:          "+testUUID)
	assert.Contains(t, out, "Name:          Support")
	assert.Contains(t, out, "Description:   Support staff")
	assert.Contains(t, out, "Kind:          managed")
	assert.Contains(t, out, "Members:       3")
	assert.Contains(t, out, "Policy sets:   2")
	assert.Contains(t, out, "Version:       4")
	assert.Contains(t, out, "Project:       proj-uuid")
	assert.Contains(t, out, "Created:       2026-04-15T10:30:00Z")
}

// The virtual All Users group has no version and no timestamps.
func TestUserGroupGet_VirtualGroup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"uuid": "9f1a2b3c-4d5e-6f7a-8b9c-0d1e2f3a4b5c", "project_uuid": "proj-uuid",
			"name": "All Users", "kind": "virtual", "member_count": 12,
			"attached_policy_set_count": 1, "version": null,
			"created_at": null, "updated_at": null
		}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupGetCmd)
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	require.NoError(t, cmd.RunE(cmd, []string{testUUID}))
	out := buf.String()
	assert.Contains(t, out, "Kind:          virtual")
	assert.Contains(t, out, "Version:       -")
	assert.Contains(t, out, "Created:       -")
	assert.Contains(t, out, "Updated:       -")
}

func TestUserGroupGet_InvalidUUID(t *testing.T) {
	err := userGroupGetCmd.RunE(userGroupGetCmd, []string{"not-valid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid user group UUID: "not-valid"`)
}

func TestUserGroupGet_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "not found"})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupGetCmd)
	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user group not found: "+testUUID)
}

// --- Create ---

func TestUserGroupCreate_TableOutput(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uuid": "9f1a2b3c-4d5e-6f7a-8b9c-0d1e2f3a4b5c", "name": "Support", "version": 1}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupCreateCmd)
	setFlags(t, cmd, map[string]string{"name": "Support", "description": "Support staff"})

	var runErr error
	out := captureStderr(t, func() {
		runErr = cmd.RunE(cmd, nil)
	})
	require.NoError(t, runErr)
	assert.Contains(t, out, `Created user group "Support"`)
	assert.Contains(t, out, "9f1a2b3c...")
	assert.Equal(t, "Support", gotBody["name"])
	assert.Equal(t, "Support staff", gotBody["description"])
}

func TestUserGroupCreate_NameFlagRequired(t *testing.T) {
	ann := userGroupCreateCmd.Flags().Lookup("name").Annotations[cobra.BashCompOneRequiredFlag]
	require.Len(t, ann, 1)
	assert.Equal(t, "true", ann[0])
}

// The server owns quota enforcement; the CLI surfaces its message.
func TestUserGroupCreate_QuotaExhausted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "user group allocation exhausted"})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupCreateCmd)
	setFlags(t, cmd, map[string]string{"name": "Support"})

	err := cmd.RunE(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating user group:")
	assert.Contains(t, err.Error(), "allocation exhausted")
}

// --- Update ---

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

// Without --expected-version, the command reads and sends the current
// version. A concurrent update between the read and the write causes a
// conflict.
func TestUserGroupUpdate_ResolvesCurrentVersion(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"uuid": "g", "name": "Support", "kind": "managed", "version": 7}`))
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"uuid": "g", "name": "Renamed", "kind": "managed", "version": 8}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupUpdateCmd)
	setFlags(t, cmd, map[string]string{"name": "Renamed"})

	var runErr error
	out := captureStderr(t, func() {
		runErr = cmd.RunE(cmd, []string{testUUID})
	})
	require.NoError(t, runErr)
	assert.Equal(t, float64(7), gotBody["expected_version"])
	assert.Equal(t, "Renamed", gotBody["name"])
	assert.Contains(t, out, `Updated user group "Renamed" (version 8)`)
}

// An explicit --expected-version is sent as given, with no extra read.
func TestUserGroupUpdate_ExplicitExpectedVersion(t *testing.T) {
	var methods []string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"uuid": "g", "name": "Support", "kind": "managed", "version": 4}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupUpdateCmd)
	setFlags(t, cmd, map[string]string{"description": "New text", "expected-version": "3"})

	captureStderr(t, func() {
		require.NoError(t, cmd.RunE(cmd, []string{testUUID}))
	})
	assert.Equal(t, []string{http.MethodPatch}, methods)
	assert.Equal(t, float64(3), gotBody["expected_version"])
	assert.Equal(t, "New text", gotBody["description"])
	assert.NotContains(t, gotBody, "name")
}

// An explicit zero is still sent as given, so the server rejects it
// instead of the CLI silently substituting the current version.
func TestUserGroupUpdate_ExplicitZeroExpectedVersion(t *testing.T) {
	var methods []string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "expected_version must be at least 1"})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupUpdateCmd)
	setFlags(t, cmd, map[string]string{"name": "Renamed", "expected-version": "0"})

	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Equal(t, []string{http.MethodPatch}, methods)
	assert.Equal(t, float64(0), gotBody["expected_version"])
	assert.Contains(t, err.Error(), "updating user group:")
}

// A stale --expected-version surfaces the server's conflict.
func TestUserGroupUpdate_VersionConflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "user group was modified by another request"})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupUpdateCmd)
	setFlags(t, cmd, map[string]string{"name": "Renamed", "expected-version": "3"})

	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating user group:")
	assert.Contains(t, err.Error(), "modified by another request")
}

// The virtual group has no version to resolve, so the CLI refuses the
// update rather than sending an invalid one.
func TestUserGroupUpdate_VirtualGroupRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"uuid": "g", "name": "All Users", "kind": "virtual", "version": null}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupUpdateCmd)
	setFlags(t, cmd, map[string]string{"name": "Renamed"})

	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "the All Users group is immutable")
}

// --- Delete ---

func TestUserGroupDelete_InvalidUUID(t *testing.T) {
	err := userGroupDeleteCmd.RunE(userGroupDeleteCmd, []string{"bad-uuid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid user group UUID: "bad-uuid"`)
}

func TestUserGroupDelete_NonInteractiveWithoutForce(t *testing.T) {
	stubStdin(t, false, "")

	err := userGroupDeleteCmd.RunE(userGroupDeleteCmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use --force to delete without confirmation")
}

func TestUserGroupDelete_Force(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupDeleteCmd)
	setFlags(t, cmd, map[string]string{"force": "true"})

	var runErr error
	out := captureStderr(t, func() {
		runErr = cmd.RunE(cmd, []string{testUUID})
	})
	require.NoError(t, runErr)
	assert.Equal(t, http.MethodDelete, gotMethod)
	assert.Equal(t, "/api/v2/user-groups/"+testUUID, gotPath)
	assert.Contains(t, out, "Deleted user group "+testUUID)
}

// Deleting the virtual group is a server-side conflict, not a client
// special case.
func TestUserGroupDelete_VirtualConflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "the All Users group cannot be deleted"})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupDeleteCmd)
	setFlags(t, cmd, map[string]string{"force": "true"})

	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deleting user group:")
	assert.Contains(t, err.Error(), "cannot be deleted")
}

// --- Members ---

func TestUserGroupMembersList_TableOutput(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"users": [
				{"uuid": "user-1", "user_id": "alice", "email": "alice@example.com",
				 "first_name": "Alice", "last_name": "Smith"}
			],
			"total_count": 1, "row_count": 1
		}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupMembersListCmd)
	setFlags(t, cmd, map[string]string{"search": "ali"})

	var runErr error
	out := captureStdout(t, func() {
		runErr = cmd.RunE(cmd, []string{testUUID})
	})
	require.NoError(t, runErr)
	assert.Contains(t, out, "USER ID")
	assert.Contains(t, out, "EMAIL")
	assert.Contains(t, out, "alice")
	assert.Contains(t, out, "alice@example.com")
	assert.Contains(t, out, "Alice")
	assert.Equal(t, "/api/v2/user-groups/"+testUUID+"/members", gotPath)
	assert.Contains(t, gotQuery, "search=ali")
}

func TestUserGroupMembersList_InvalidUUID(t *testing.T) {
	err := userGroupMembersListCmd.RunE(userGroupMembersListCmd, []string{"bad"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid user group UUID: "bad"`)
}

func TestUserGroupMembersCandidates_TableOutput(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"users": [{"uuid": "user-2", "user_id": "bob"}], "total_count": 1, "row_count": 1}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupMembersCandidatesCmd)
	var runErr error
	out := captureStdout(t, func() {
		runErr = cmd.RunE(cmd, []string{testUUID})
	})
	require.NoError(t, runErr)
	assert.Contains(t, out, "bob")
	assert.Equal(t, "/api/v2/user-groups/"+testUUID+"/members/candidates", gotPath)
}

func TestUserGroupMembersAdd_TableOutput(t *testing.T) {
	var gotPath string
	var gotBody map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"added_count": 1, "no_op_count": 1})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupMembersAddCmd)
	var runErr error
	out := captureStderr(t, func() {
		runErr = cmd.RunE(cmd, []string{testUUID, testUUID2, testUUID3})
	})
	require.NoError(t, runErr)
	assert.Equal(t, "/api/v2/user-groups/"+testUUID+"/members", gotPath)
	assert.Equal(t, []string{testUUID2, testUUID3}, gotBody["user_uuids"])
	assert.Contains(t, out, "Added 1 member(s)")
	assert.Contains(t, out, "1 already a member")
}

// A single removal still uses the bulk endpoint, so one code path covers
// both cases.
func TestUserGroupMembersRemove_UsesBulkEndpoint(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"removed_count": 1, "no_op_count": 0})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupMembersRemoveCmd)
	var runErr error
	out := captureStderr(t, func() {
		runErr = cmd.RunE(cmd, []string{testUUID, testUUID2})
	})
	require.NoError(t, runErr)
	assert.Equal(t, "/api/v2/user-groups/"+testUUID+"/members/bulk-remove", gotPath)
	assert.Contains(t, out, "Removed 1 member(s)")
}

// An unknown user UUID is the same 404 as an unknown group, so the
// server's message decides which object was not found.
func TestUserGroupMembersAdd_NotFoundKeepsServerMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "User not found"})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupMembersAddCmd)
	err := cmd.RunE(cmd, []string{testUUID, testUUID2})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "adding members: User not found")
	assert.NotContains(t, err.Error(), "user group not found")
}

func TestUserGroupMembersRemove_NotFoundKeepsServerMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "User not found"})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupMembersRemoveCmd)
	err := cmd.RunE(cmd, []string{testUUID, testUUID2})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "removing members: User not found")
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

// --- Policy Set Attachments ---

func TestUserGroupPolicySetsList_TableOutput(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"policy_sets": [
			{"uuid": "ps-1", "name": "support_read", "mode": "enforce", "version": 2}
		]}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupPolicySetsListCmd)
	var runErr error
	out := captureStdout(t, func() {
		runErr = cmd.RunE(cmd, []string{testUUID})
	})
	require.NoError(t, runErr)
	assert.Contains(t, out, "MODE")
	assert.Contains(t, out, "support_read")
	assert.Contains(t, out, "enforce")
	assert.Equal(t, "/api/v2/abac/user-groups/"+testUUID+"/policy-sets", gotPath)
}

func TestUserGroupPolicySetsAttach_TableOutput(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"policy_sets": [
			{"uuid": "` + testUUID2 + `", "name": "support_read", "mode": "enforce", "version": 2}
		]}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupPolicySetsAttachCmd)
	var runErr error
	out := captureStderr(t, func() {
		runErr = cmd.RunE(cmd, []string{testUUID, testUUID2})
	})
	require.NoError(t, runErr)
	assert.Equal(t, testUUID2, gotBody["policy_set_uuid"])
	assert.Contains(t, out, `Attached policy set "support_read"`)
}

func TestUserGroupPolicySetsAttach_InvalidPolicySetUUID(t *testing.T) {
	cmd := newTestCommand(userGroupPolicySetsAttachCmd)
	err := cmd.RunE(cmd, []string{testUUID, "bad"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid policy set UUID: "bad"`)
}

func TestUserGroupPolicySetsDetach_TableOutput(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"policy_sets": []}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupPolicySetsDetachCmd)
	var runErr error
	out := captureStderr(t, func() {
		runErr = cmd.RunE(cmd, []string{testUUID, testUUID2})
	})
	require.NoError(t, runErr)
	assert.Equal(t, http.MethodDelete, gotMethod)
	assert.Equal(t, "/api/v2/abac/user-groups/"+testUUID+"/policy-sets/"+testUUID2, gotPath)
	assert.Contains(t, out, "Detached policy set from user group")
}

// --- Dry runs ---

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
			ann := cmd.Flags().Lookup("action").Annotations[cobra.BashCompOneRequiredFlag]
			require.Len(t, ann, 1)
			assert.Equal(t, "true", ann[0])
		})
	}
}

func TestUserGroupEvaluate_TableOutputWithGraph(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"outcome": "allow", "abac": "ALLOW", "abac_shadow": "NEUTRAL",
			"role_allows": false, "would_log_disagreement": false,
			"base": "default_deny", "base_allows": false,
			"principal": {"type": "user_group", "uuid": "` + testUUID + `"},
			"resource": {"type": "graph", "uuid": "` + testUUID2 + `"}
		}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupEvaluateCmd)
	setFlags(t, cmd, map[string]string{"action": "graph.search", "graph": testUUID2})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	require.NoError(t, cmd.RunE(cmd, []string{testUUID}))
	out := buf.String()
	assert.Equal(t, map[string]any{"type": "user_group", "uuid": testUUID}, gotBody["principal"])
	assert.Equal(t, map[string]any{"type": "graph", "uuid": testUUID2}, gotBody["resource"])
	assert.Contains(t, out, "Principal:              user_group "+testUUID)
	assert.Contains(t, out, "Resource:               graph "+testUUID2)
	assert.Contains(t, out, "Outcome:                allow")
	assert.Contains(t, out, "Base:                   default_deny")
	assert.Contains(t, out, "Base allows:            false")
}

// An outcome of deny is an answer, not an error.
func TestUserGroupEvaluate_DenyOutcomeExitsZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"outcome":"deny","abac":"DENY","abac_shadow":"NEUTRAL",
			"role_allows":false,"would_log_disagreement":false,"base":"default_deny","base_allows":false}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupEvaluateCmd)
	setFlags(t, cmd, map[string]string{"action": "graph.search"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	require.NoError(t, cmd.RunE(cmd, []string{testUUID}))
	assert.Contains(t, buf.String(), "Outcome:                deny")
}

func TestUserGroupEvaluate_InvalidGraphUUID(t *testing.T) {
	cmd := newTestCommand(userGroupEvaluateCmd)
	setFlags(t, cmd, map[string]string{"action": "graph.search", "graph": "not-a-uuid"})

	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid graph UUID: "not-a-uuid"`)
}

func TestUserGroupEvaluate_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "not found"})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupEvaluateCmd)
	setFlags(t, cmd, map[string]string{"action": "graph.search"})

	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user group not found: "+testUUID)
}

// With --graph the same 404 covers a missing graph, so the CLI passes the
// server's message through instead of blaming the group.
func TestUserGroupEvaluate_NotFoundWithGraphKeepsServerMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "graph not found"})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userGroupEvaluateCmd)
	setFlags(t, cmd, map[string]string{"action": "graph.search", "graph": testUUID2})

	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "evaluating policy: graph not found")
	assert.NotContains(t, err.Error(), "user group not found")
}

// The explain trace renders the user-group-specific sections: effective
// groups, per-set contributing groups, resource matches, skipped
// policies, object constraints, and dead graph references.
func TestUserExplain_TableOutput_ResourceAwareSections(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"outcome": "allow", "abac": "ALLOW", "abac_shadow": "NEUTRAL",
			"role_allows": false, "would_log_disagreement": false,
			"base": "default_deny", "base_allows": false,
			"principal": {"type": "user", "uuid": "` + testUUID + `"},
			"resource": {"type": "graph", "uuid": "` + testUUID2 + `"},
			"trace": {
				"action": "graph.search",
				"registry_entry": {"read_only": true},
				"resolved_group_uuids": ["` + testUUID2 + `", "` + testUUID3 + `"],
				"evaluated_sets": [
					{"uuid": "9f1a2b3c-set-uuid", "name": "support_read", "set_mode": "enforce",
					 "contributing_group_uuids": ["` + testUUID3 + `"],
					 "matched": [{"policy_id": "read_graph", "effect": "allow",
					   "matched_via": "literal", "resource_match": "explicit"}],
					 "skipped": [{"policy_id": "other_graph", "effect": "allow",
					   "resource_match": "resource_mismatch"}]}
				],
				"skipped_sets": [],
				"object_constraints": {
					"include": [{"attributes": {"source": ["email", "slack"]}, "subset_match": true}],
					"exclude": [{"attributes": {"source": ["private"]}, "subset_match": false}]
				},
				"dead_graph_uuids": ["` + testUUID3 + `"],
				"future_top_level": "surprise"
			}
		}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userExplainCmd)
	setFlags(t, cmd, map[string]string{"action": "graph.search", "graph": testUUID2})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	require.NoError(t, cmd.RunE(cmd, []string{testUUID}))
	out := buf.String()

	assert.Contains(t, out, "Action:                 graph.search")
	assert.Contains(t, out, "Principal:              user "+testUUID)
	assert.Contains(t, out, "Resource:               graph "+testUUID2)
	assert.Contains(t, out, "Base:                   default_deny")
	assert.Contains(t, out, "Effective UserGroups:")
	assert.Contains(t, out, "  "+testUUID3)
	assert.Contains(t, out, "support_read (9f1a2b3c..., set mode: enforce, groups: b2c3d4e5...)")
	assert.Contains(t, out, "[allow] read_graph -- matched via literal -- resource explicit")
	assert.Contains(t, out, "(skipped) [allow] other_graph -- resource resource_mismatch")
	assert.Contains(t, out, "Skipped policy sets: (none)")
	assert.Contains(t, out, "Object constraints:")
	assert.Contains(t, out, "include: source=[email slack] (subset match: true)")
	assert.Contains(t, out, "exclude: source=[private] (subset match: false)")
	assert.Contains(t, out, "Dead graph references:")
}

// An API key explain must be unchanged by the user-group additions: no
// principal, resource, base, or trailing sections.
func TestAPIKeyExplain_TableOutput_OmitsUserGroupSections(t *testing.T) {
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
	setFlags(t, cmd, map[string]string{"action": "thread.get"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	require.NoError(t, cmd.RunE(cmd, []string{testUUID}))
	out := buf.String()
	assert.NotContains(t, out, "Principal:")
	assert.NotContains(t, out, "Resource:")
	assert.NotContains(t, out, "Base:")
	assert.NotContains(t, out, "Effective UserGroups:")
	assert.NotContains(t, out, "Object constraints:")
	assert.NotContains(t, out, "Dead graph references:")
	assert.Contains(t, out, "    [allow] allow_reads -- matched via readonly\n")
}

// --- User ABAC subcommands ---

func TestUserGroups_TableOutput(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user_groups": [
			{"uuid": "group-all", "name": "All Users", "kind": "virtual", "member_count": 12, "version": null},
			{"uuid": "group-1", "name": "Support", "kind": "managed", "member_count": 3, "version": 2}
		]}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userMembershipsCmd)
	var runErr error
	out := captureStdout(t, func() {
		runErr = cmd.RunE(cmd, []string{testUUID})
	})
	require.NoError(t, runErr)
	assert.Contains(t, out, "All Users")
	assert.Contains(t, out, "Support")
	assert.Equal(t, "/api/v2/user-groups/users/"+testUUID, gotPath)
}

func TestUserGroups_InvalidUUID(t *testing.T) {
	err := userMembershipsCmd.RunE(userMembershipsCmd, []string{"bad"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid user UUID: "bad"`)
}

func TestUserGroups_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "not found"})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userMembershipsCmd)
	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user not found: "+testUUID)
}

func TestUserEvaluate_InvalidUUID(t *testing.T) {
	err := userEvaluateCmd.RunE(userEvaluateCmd, []string{"bad"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid user UUID: "bad"`)
}

func TestUserEvaluate_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "not found"})
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	cmd := newTestCommand(userEvaluateCmd)
	setFlags(t, cmd, map[string]string{"action": "graph.search"})

	err := cmd.RunE(cmd, []string{testUUID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user not found: "+testUUID)
}

// JSON output keeps the whole server body, including fields the typed
// shape does not model.
func TestUserGroupExplain_JSONOutput_PreservesUnknownFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"outcome": "allow", "trace": {"action": "graph.search",
			"registry_entry": {"read_only": true}, "evaluated_sets": [], "skipped_sets": [],
			"future_group_field": "surprise"}
		}`))
	}))
	defer srv.Close()
	withTestABACClient(t, srv)

	viper.Set("output", "json")
	defer viper.Set("output", "")

	cmd := newTestCommand(userGroupExplainCmd)
	setFlags(t, cmd, map[string]string{"action": "graph.search"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	require.NoError(t, cmd.RunE(cmd, []string{testUUID}))
	assert.Contains(t, buf.String(), `"future_group_field": "surprise"`)
}
