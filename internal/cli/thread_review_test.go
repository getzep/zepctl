package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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
