package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/getzep/zepctl/internal/config"
	"github.com/getzep/zepctl/internal/keyring"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserGroupCommands_CredentialType(t *testing.T) {
	commands := map[string]*cobra.Command{
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
	for name, command := range commands {
		t.Run(name, func(t *testing.T) {
			require.NotNil(t, command.Annotations)
			assert.Equal(t, "bearer", command.Annotations["zepctl_credential_type"])
		})
	}
}

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

func configureBearerSDKTestServer(t *testing.T, server *httptest.Server) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Cleanup(func() { _, _ = config.Reload() })
	configDir := filepath.Join(home, ".zepctl")
	require.NoError(t, os.MkdirAll(configDir, 0o700))
	configText := fmt.Sprintf(
		"current-profile: user-group-sdk-test\nprofiles:\n  - name: user-group-sdk-test\n    api-url: %s\n    project-uuid: project-test\n    account-uuid: account-test\n",
		server.URL,
	)
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configText), 0o600))
	_, err := config.Reload()
	require.NoError(t, err)
	require.NoError(t, keyring.SetCredentials("user-group-sdk-test", &keyring.Credentials{
		AccessToken: "test-access-token",
		ExpiresAt:   time.Now().Add(time.Hour).Format(time.RFC3339),
	}))
	t.Cleanup(func() { _ = keyring.Delete("user-group-sdk-test") })
}

func TestUserGroupGet_UsesBearerSDKClient(t *testing.T) {
	var gotMethod, gotPath, gotAuthorization, gotProject, gotAccount string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuthorization = r.Header.Get("Authorization")
		gotProject = r.Header.Get("X-Zep-Project")
		gotAccount = r.Header.Get("X-Zep-Account-UUID")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"uuid":"9f1a2b3c-4d5e-6f7a-8b9c-0d1e2f3a4b5c","name":"Support","version":1}`))
	}))
	defer srv.Close()

	home := t.TempDir()
	t.Cleanup(func() { _, _ = config.Reload() })
	t.Setenv("HOME", home)
	viper.Reset()
	t.Cleanup(viper.Reset)
	configDir := filepath.Join(home, ".zepctl")
	require.NoError(t, os.MkdirAll(configDir, 0o700))
	configText := fmt.Sprintf(
		"current-profile: user-group-sdk-test\nprofiles:\n  - name: user-group-sdk-test\n    api-url: %s\n    project-uuid: project-test\n    account-uuid: account-test\n",
		srv.URL,
	)
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configText), 0o600))
	_, err := config.Reload()
	require.NoError(t, err)
	require.NoError(t, keyring.SetCredentials("user-group-sdk-test", &keyring.Credentials{
		AccessToken: "test-access-token",
		ExpiresAt:   time.Now().Add(time.Hour).Format(time.RFC3339),
	}))
	t.Cleanup(func() { _ = keyring.Delete("user-group-sdk-test") })

	cmd := newTestCommand(userGroupGetCmd)
	err = cmd.RunE(cmd, []string{testUUID})
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, gotMethod)
	require.Equal(t, "/api/v4/user-groups/"+testUUID, gotPath)
	require.Equal(t, "Bearer test-access-token", gotAuthorization)
	require.Equal(t, "project-test", gotProject)
	require.Equal(t, "account-test", gotAccount)
}

func TestUserGroupPolicySetsListUsesV4Path(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"policy_sets":[]}`))
	}))
	defer server.Close()
	withTestABACClient(t, server)

	cmd := newTestCommand(userGroupPolicySetsListCmd)
	require.NoError(t, cmd.RunE(cmd, []string{testUUID}))
	assert.Equal(t, "/api/v4/user-groups/"+testUUID+"/policy-sets", gotPath)
}
