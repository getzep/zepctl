package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThreadLastNRequestsDescendingAndRendersChronologically(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
	}{
		{name: "get", cmd: threadGetCmd},
		{name: "messages", cmd: threadMessagesCmd},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var gotOrder, gotPageSize, gotPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotOrder = r.URL.Query().Get("order")
				gotPageSize = r.URL.Query().Get("page_size")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"items":[{"role":"assistant","content":"newest"},{"role":"assistant","content":"oldest"}]}`))
			}))
			t.Cleanup(server.Close)
			configureSDKTestServer(t, server)

			cmd := newTestCommand(test.cmd)
			setFlags(t, cmd, map[string]string{"last": "2"})
			var runErr error
			out := captureStdout(t, func() { runErr = cmd.RunE(cmd, []string{testUUID}) })
			require.NoError(t, runErr)
			assert.Equal(t, "/api/v4/threads/"+testUUID+"/messages", gotPath)
			assert.Equal(t, "desc", gotOrder)
			assert.Equal(t, "2", gotPageSize)
			require.Contains(t, out, "oldest")
			require.Contains(t, out, "newest")
			assert.Less(t, strings.Index(out, "oldest"), strings.Index(out, "newest"))
		})
	}
}

func TestThreadGetJSONPrintsMessagesAsItems(t *testing.T) {
	tests := []struct {
		name        string
		last        string
		response    string
		wantContent []string
		wantOrder   string
		wantPage    string
	}{
		{
			name:        "all messages",
			response:    `{"items":[{"role":"assistant","content":"oldest"},{"role":"assistant","content":"newest"}]}`,
			wantContent: []string{"oldest", "newest"},
		},
		{
			name:        "last two messages are chronological",
			last:        "2",
			response:    `{"items":[{"role":"assistant","content":"newest"},{"role":"assistant","content":"oldest"}]}`,
			wantContent: []string{"oldest", "newest"},
			wantOrder:   "desc",
			wantPage:    "2",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var gotOrder, gotPageSize string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotOrder = r.URL.Query().Get("order")
				gotPageSize = r.URL.Query().Get("page_size")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(test.response))
			}))
			t.Cleanup(server.Close)
			configureSDKTestServer(t, server)
			viper.Set("output", "json")

			cmd := newTestCommand(threadGetCmd)
			if test.last != "" {
				setFlags(t, cmd, map[string]string{"last": test.last})
			}
			var runErr error
			out := captureStdout(t, func() { runErr = cmd.RunE(cmd, []string{testUUID}) })
			require.NoError(t, runErr)

			var response struct {
				Items []struct {
					Content string `json:"content"`
				} `json:"items"`
			}
			require.NoError(t, json.Unmarshal([]byte(out), &response))
			require.Len(t, response.Items, len(test.wantContent))
			for i, content := range test.wantContent {
				assert.Equal(t, content, response.Items[i].Content)
			}
			assert.Equal(t, test.wantOrder, gotOrder)
			assert.Equal(t, test.wantPage, gotPageSize)
		})
	}
}
