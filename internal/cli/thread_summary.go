package cli

import (
	"context"
	"fmt"

	"github.com/getzep/zep-go/v3"
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
	Use:   "list",
	Short: "List thread summaries",
	RunE: func(cmd *cobra.Command, args []string) error {
		userID, _ := cmd.Flags().GetString("user")
		graphID, _ := cmd.Flags().GetString("graph")
		limit, _ := cmd.Flags().GetInt("limit")
		cursor, _ := cmd.Flags().GetString("cursor")

		if userID == "" && graphID == "" {
			return fmt.Errorf("either --user or --graph is required")
		}

		c, err := client.New()
		if err != nil {
			return err
		}

		req := &zep.GraphThreadSummariesRequest{}
		if limit > 0 {
			req.Limit = zep.Int(limit)
		}
		if cursor != "" {
			req.UUIDCursor = zep.String(cursor)
		}

		var summaries []*zep.ThreadSummary

		if userID != "" {
			result, err := c.Graph.ThreadSummary.GetByUserID(context.Background(), userID, req)
			if err != nil {
				return fmt.Errorf("listing thread summaries: %w", err)
			}
			summaries = result
		} else {
			result, err := c.Graph.ThreadSummary.GetByGraphID(context.Background(), graphID, req)
			if err != nil {
				return fmt.Errorf("listing thread summaries: %w", err)
			}
			summaries = result
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("UUID", "THREAD ID", "SUMMARY", "LAST SUMMARIZED AT")
			tbl.WriteHeader()
			for _, s := range summaries {
				uuid := ""
				if s.UUID != nil {
					uuid = *s.UUID
				}
				threadID := ""
				if s.ThreadID != nil {
					threadID = *s.ThreadID
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
				tbl.WriteRow(uuid, threadID, summary, lastSummarizedAt)
			}
			return tbl.Flush()
		}

		return output.Print(summaries)
	},
}

func init() {
	rootCmd.AddCommand(threadSummaryCmd)
	threadSummaryCmd.AddCommand(threadSummaryListCmd)

	threadSummaryListCmd.Flags().String("user", "", "List thread summaries for user graph")
	threadSummaryListCmd.Flags().String("graph", "", "List thread summaries for standalone graph")
	threadSummaryListCmd.Flags().Int("limit", 50, "Maximum number of results to return")
	threadSummaryListCmd.Flags().String("cursor", "", "UUID cursor for pagination (last UUID from previous page)")
}
