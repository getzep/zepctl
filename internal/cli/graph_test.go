package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func TestGraphSearchUsesV4ScopeEndpoints(t *testing.T) {
	tests := []struct {
		scope string
		path  string
		body  string
	}{
		{scope: "edges", path: "/api/v4/graphs/" + testUUID + "/search/edges", body: `{"items":[]}`},
		{scope: "nodes", path: "/api/v4/graphs/" + testUUID + "/search/nodes", body: `{"items":[]}`},
		{scope: "episodes", path: "/api/v4/graphs/" + testUUID + "/search/episodes", body: `{"items":[]}`},
		{scope: "observations", path: "/api/v4/graphs/" + testUUID + "/search/observations", body: `{"items":[]}`},
		{scope: "thread_summaries", path: "/api/v4/graphs/" + testUUID + "/search/thread-summaries", body: `{"items":[]}`},
		{scope: "auto", path: "/api/v4/graphs/" + testUUID + "/context", body: `{"context":"context"}`},
	}

	for _, test := range tests {
		t.Run(test.scope, func(t *testing.T) {
			var gotPath, gotMethod string
			var gotBody map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath, gotMethod = r.URL.Path, r.Method
				require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()
			configureSDKTestServer(t, server)

			cmd := newTestCommand(graphSearchCmd)
			setGraphSearchFlags(t, cmd, map[string]string{"user": testUUID, "scope": test.scope})
			require.NoError(t, cmd.RunE(cmd, []string{"query"}))
			require.Equal(t, test.path, gotPath)
			require.Equal(t, http.MethodPost, gotMethod)
			require.Equal(t, "query", gotBody["query"])
		})
	}
}

func TestGraphSearchRejectsFlagsUnsupportedByScope(t *testing.T) {
	cmd := newTestCommand(graphSearchCmd)
	setGraphSearchFlags(t, cmd, map[string]string{
		"user":           testUUID,
		"scope":          "edges",
		"max-characters": "100",
	})
	require.ErrorContains(t, cmd.RunE(cmd, []string{"query"}), "--max-characters is not supported with scope edges")

	cmd = newTestCommand(graphSearchCmd)
	setGraphSearchFlags(t, cmd, map[string]string{
		"user":  testUUID,
		"scope": "auto",
		"limit": "5",
	})
	require.ErrorContains(t, cmd.RunE(cmd, []string{"query"}), "--limit is not supported with scope auto")

	cmd = newTestCommand(graphSearchCmd)
	setGraphSearchFlags(t, cmd, map[string]string{
		"user":        testUUID,
		"scope":       "nodes",
		"date-filter": "created_at:>=:2024-01-01",
	})
	require.ErrorContains(t, cmd.RunE(cmd, []string{"query"}), "--date-filter is not supported with scope nodes")
}

func TestGraphSearchMapsFiltersToV4ScopeRequests(t *testing.T) {
	tests := []struct {
		name    string
		scope   string
		flags   map[string]string
		filters map[string]any
	}{
		{
			name:  "edges",
			scope: "edges",
			flags: map[string]string{
				"edge-types":         "WORKS_AT",
				"exclude-edge-types": "KNOWS",
				"property-filter":    "status:=:active",
				"date-filter":        "created_at:>=:2024-01-01T00:00:00Z",
			},
			filters: map[string]any{
				"edge_types":         []any{"WORKS_AT"},
				"exclude_edge_types": []any{"KNOWS"},
				"property_filters": []any{
					map[string]any{"operator": "eq", "property_name": "status", "value": "active"},
				},
				"date_filters": map[string]any{
					"any_of": []any{
						map[string]any{
							"all_of": []any{
								map[string]any{"field": "created_at", "operator": "gte", "value": "2024-01-01T00:00:00Z"},
							},
						},
					},
				},
			},
		},
		{
			name:  "nodes",
			scope: "nodes",
			flags: map[string]string{
				"node-labels":         "Person,Organization",
				"exclude-node-labels": "Document",
				"property-filter":     "status:=:active",
			},
			filters: map[string]any{
				"node_labels":         []any{"Person", "Organization"},
				"exclude_node_labels": []any{"Document"},
				"property_filters": []any{
					map[string]any{"operator": "eq", "property_name": "status", "value": "active"},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var gotFilters map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				filters, ok := body["filters"].(map[string]any)
				require.True(t, ok)
				gotFilters = filters
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"items":[]}`)
			}))
			defer server.Close()
			configureSDKTestServer(t, server)

			cmd := newTestCommand(graphSearchCmd)
			flags := map[string]string{"user": testUUID, "scope": test.scope}
			for name, value := range test.flags {
				flags[name] = value
			}
			setGraphSearchFlags(t, cmd, flags)
			require.NoError(t, cmd.RunE(cmd, []string{"query"}))
			require.Equal(t, test.filters, gotFilters)
		})
	}
}

func setGraphSearchFlags(t *testing.T, cmd *cobra.Command, values map[string]string) {
	t.Helper()
	for _, name := range graphSearchFlags {
		flag := cmd.Flags().Lookup(name)
		require.NotNil(t, flag)
		originalValue, originalChanged := flag.Value, flag.Changed
		if name == "property-filter" || name == "date-filter" {
			resetFlags := pflag.NewFlagSet("test", pflag.ContinueOnError)
			resetFlags.StringArray(name, nil, "")
			flag.Value = resetFlags.Lookup(name).Value
		} else {
			require.NoError(t, flag.Value.Set(flag.DefValue))
		}
		flag.Changed = false
		t.Cleanup(func() {
			flag.Value = originalValue
			flag.Changed = originalChanged
		})
	}
	for name, value := range values {
		flag := cmd.Flags().Lookup(name)
		require.NotNil(t, flag)
		require.NoError(t, cmd.Flags().Set(name, value))
	}
}
