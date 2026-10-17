package cli

import (
	"context"
	"fmt"

	"github.com/getzep/zep-go/v4"
	zepgraph "github.com/getzep/zep-go/v4/graph"
	"github.com/getzep/zepctl/internal/client"
	"github.com/getzep/zepctl/internal/output"
	"github.com/spf13/cobra"
)

var threadSummaryCmd = &cobra.Command{
	Use:   "thread-summary",
	Short: "Manage thread summaries",
	Long:  `List incremental thread summaries for a user or graph.`,
}

var threadSummaryListCmd = &cobra.Command{
	Use:   listCmdUse,
	Short: "List thread summaries",
	RunE: func(cmd *cobra.Command, args []string) error {
		userID, _ := cmd.Flags().GetString("user")
		graphID, _ := cmd.Flags().GetString("graph")
		limit, _ := cmd.Flags().GetInt("limit")
		cursor, _ := cmd.Flags().GetString("cursor")

		if userID == "" && graphID == "" {
			return fmt.Errorf("either --user or --graph is required")
		}
		if userID != "" && graphID != "" {
			return fmt.Errorf("--user and --graph are mutually exclusive")
		}

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		req := &zepgraph.ThreadSummaryListRequest{}
		if limit > 0 {
			req.PageSize = zep.Int(limit)
		}
		if cursor != "" {
			req.Cursor = zep.String(cursor)
		}

		graphUUID := graphID
		if userID != "" {
			graphUUID = userID
		}
		page, err := c.Graph.ThreadSummary.List(context.Background(), graphUUID, req)
		if err != nil {
			return fmt.Errorf("listing thread summaries: %w", err)
		}
		summaries := page.Results

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("UUID", "THREAD UUID", "SUMMARY", "LAST SUMMARIZED AT")
			tbl.WriteHeader()
			for _, s := range summaries {
				uuid := ""
				if s.UUID != nil {
					uuid = *s.UUID
				}
				threadUUID := ""
				if s.ThreadUUID != nil {
					threadUUID = *s.ThreadUUID
				}
				summary := ""
				if s.Summary != nil {
					summary = *s.Summary
					if len(summary) > 50 {
						summary = summary[:50] + "..."
					}
				}
				lastSummarizedAt := ""
				if s.LastSummarizedAt != nil {
					lastSummarizedAt = *s.LastSummarizedAt
				}
				tbl.WriteRow(uuid, threadUUID, summary, lastSummarizedAt)
			}
			writeNextCursor(page.RawResponse.Next)
			return tbl.Flush()
		}

		return output.Print(map[string]any{itemsOutputKey: summaries, nextCursorOutputKey: page.RawResponse.Next})
	},
}

func init() {
	rootCmd.AddCommand(threadSummaryCmd)
	threadSummaryCmd.AddCommand(threadSummaryListCmd)

	threadSummaryListCmd.Flags().String("user", "", "User UUID for a user graph")
	threadSummaryListCmd.Flags().String("graph", "", "Graph UUID for a standalone graph")
	threadSummaryListCmd.Flags().Int("limit", 50, "Maximum number of results to return")
	threadSummaryListCmd.Flags().String("cursor", "", "UUID cursor for pagination (last UUID from previous page)")
}
