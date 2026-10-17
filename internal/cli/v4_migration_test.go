package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func configureSDKTestServer(t *testing.T, server *httptest.Server) {
	t.Helper()
	viper.Set("api-key", "test-api-key")
	viper.Set("api-url", server.URL)
	t.Cleanup(viper.Reset)
}

func TestUserCreateUsesExternalIDAndRejectsPositionals(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v4/users", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"uuid":"`+testUUID+`","user_id":"external-123"}`)
	}))
	defer server.Close()
	configureSDKTestServer(t, server)

	cmd := newTestCommand(userCreateCmd)
	require.NoError(t, cmd.Flags().Set("user-id", "external-123"))
	require.NoError(t, cmd.Args(cmd, nil))
	require.Error(t, cmd.Args(cmd, []string{"legacy-id"}))
	require.NoError(t, cmd.RunE(cmd, nil))
	require.Equal(t, "external-123", body["user_id"])
}
