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

func TestSummaryInstructionsAddWritesFullSet(t *testing.T) {
	var putBody map[string][]map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v4/users/"+testUUID+"/summary-instructions", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `{"instructions":[{"name":"existing","text":"keep"}]}`)
			return
		}
		require.Equal(t, http.MethodPut, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&putBody))
		_, _ = io.WriteString(w, `{"instructions":[{"name":"existing","text":"keep"},{"name":"new","text":"add"}]}`)
	}))
	defer server.Close()
	viper.Set("api-key", "test-api-key")
	viper.Set("api-url", server.URL)
	t.Cleanup(viper.Reset)

	cmd := newTestCommand(summaryInstructionsAddCmd)
	require.NoError(t, cmd.Flags().Set("user", testUUID))
	require.NoError(t, cmd.Flags().Set("name", "new"))
	require.NoError(t, cmd.Flags().Set("instruction", "add"))
	require.NoError(t, cmd.RunE(cmd, nil))
	require.Equal(t, []map[string]string{
		{"name": "existing", "text": "keep"},
		{"name": "new", "text": "add"},
	}, putBody["instructions"])
}

func TestSummaryInstructionsDeleteWritesFullSetAndRejectsMissingName(t *testing.T) {
	putCount := 0
	var putBody map[string][]map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v4/users/"+testUUID+"/summary-instructions", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `{"instructions":[{"name":"keep","text":"keep text"},{"name":"remove","text":"remove text"}]}`)
			return
		}
		putCount++
		require.Equal(t, http.MethodPut, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&putBody))
		_, _ = io.WriteString(w, `{"instructions":[{"name":"keep","text":"keep text"}]}`)
	}))
	defer server.Close()
	viper.Set("api-key", "test-api-key")
	viper.Set("api-url", server.URL)
	t.Cleanup(viper.Reset)

	cmd := newTestCommand(summaryInstructionsDeleteCmd)
	require.NoError(t, cmd.Flags().Set("user", testUUID))
	require.NoError(t, cmd.Flags().Set("force", "true"))
	require.NoError(t, cmd.RunE(cmd, []string{"remove"}))
	require.Equal(t, []map[string]string{{"name": "keep", "text": "keep text"}}, putBody["instructions"])

	cmd = newTestCommand(summaryInstructionsDeleteCmd)
	require.NoError(t, cmd.Flags().Set("user", testUUID))
	require.NoError(t, cmd.Flags().Set("force", "true"))
	require.ErrorContains(t, cmd.RunE(cmd, []string{"missing"}), `summary instruction "missing" was not found`)
	require.Equal(t, 1, putCount)
}
