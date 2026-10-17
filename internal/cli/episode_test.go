package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestEpisodeMentionsUsesSeparateCursors(t *testing.T) {
	var nodesCursor, edgesCursor string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v4/graphs/" + testUUID + "/episodes/" + testUUID:
			require.Equal(t, http.MethodGet, r.Method)
			_, _ = io.WriteString(w, `{"uuid":"`+testUUID+`"}`)
		case "/api/v4/graphs/" + testUUID + "/nodes/list":
			require.Equal(t, http.MethodPost, r.Method)
			nodesCursor = r.URL.Query().Get("cursor")
			_, _ = io.WriteString(w, `{"items":[]}`)
		case "/api/v4/graphs/" + testUUID + "/edges/list":
			require.Equal(t, http.MethodPost, r.Method)
			edgesCursor = r.URL.Query().Get("cursor")
			_, _ = io.WriteString(w, `{"items":[]}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	configureSDKTestServer(t, server)
	viper.Set("output", "json")

	cmd := newTestCommand(episodeMentionsCmd)
	resetCmdFlags(t, cmd, "user", "graph", "limit", "nodes-cursor", "edges-cursor")
	require.NoError(t, cmd.Flags().Set("graph", testUUID))
	require.NoError(t, cmd.Flags().Set("nodes-cursor", "nodes-page"))
	require.NoError(t, cmd.Flags().Set("edges-cursor", "edges-page"))

	captureStdout(t, func() {
		require.NoError(t, cmd.RunE(cmd, []string{testUUID}))
	})

	require.Equal(t, "nodes-page", nodesCursor)
	require.Equal(t, "edges-page", edgesCursor)
}
