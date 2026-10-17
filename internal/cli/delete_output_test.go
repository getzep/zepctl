package cli

import (
	"net/http"
	"net/http/httptest"
	"testing"

	zep "github.com/getzep/zep-go/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportDeletionWithoutTask(t *testing.T) {
	for _, wait := range []bool{false, true} {
		out := captureStderr(t, func() {
			require.NoError(t, reportDeletion(nil, "graph", testUUID, nil, wait, defaultTaskTimeout))
		})
		assert.Equal(t, "Deleted graph "+testUUID+"\n", out)
	}
}

func TestReportDeletionStarted(t *testing.T) {
	task := &zep.Task{UUID: zep.String("task-uuid")}
	out := captureStderr(t, func() {
		require.NoError(t, reportDeletion(nil, "graph", testUUID, task, false, defaultTaskTimeout))
	})
	assert.Equal(t, "Deletion of graph "+testUUID+" started; task task-uuid\n", out)
}

func TestThreadDelete_WaitedReportsSuccess(t *testing.T) {
	var gotDeletePath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodDelete:
			gotDeletePath = r.URL.Path
			_, _ = w.Write([]byte(`{"task":{"uuid":"task-uuid"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/tasks/task-uuid":
			_, _ = w.Write([]byte(`{"uuid":"task-uuid","status":"succeeded"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	configureSDKTestServer(t, server)

	cmd := newTestCommand(threadDeleteCmd)
	setFlags(t, cmd, map[string]string{"force": "true", "wait": "true", "timeout": "3s"})
	var runErr error
	out := captureStderr(t, func() { runErr = cmd.RunE(cmd, []string{testUUID}) })
	require.NoError(t, runErr)
	assert.Equal(t, "/api/v4/threads/"+testUUID, gotDeletePath)
	assert.Contains(t, out, "Deletion task: task-uuid")
	assert.Contains(t, out, "Deleted thread "+testUUID)
}
