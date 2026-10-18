package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/getzep/zep-go/v4"
	zepgraph "github.com/getzep/zep-go/v4/graph"
	"github.com/getzep/zepctl/internal/client"
	"github.com/getzep/zepctl/internal/output"
	"github.com/spf13/cobra"
)

var observationCmd = &cobra.Command{
	Use:   "observation",
	Short: "Manage graph observations",
	Long:  `List and get derived observation nodes (read-only) for a user or graph.`,
}

var observationListCmd = &cobra.Command{
	Use:   listCmdUse,
	Short: "List observations",
	RunE: func(cmd *cobra.Command, args []string) error {
		userID, _ := cmd.Flags().GetString("user")
		graphID, _ := cmd.Flags().GetString("graph")
		limit, _ := cmd.Flags().GetInt("limit")
		cursor, _ := cmd.Flags().GetString("cursor")

		if userID == "" && graphID == "" {
			return fmt.Errorf("either --user or --graph is required")
		}

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		req := &zepgraph.ObservationListRequest{}
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
		page, err := c.Graph.Observation.List(context.Background(), graphUUID, req)
		if err != nil {
			return fmt.Errorf("listing observations: %w", err)
		}
		observations := page.Results

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("UUID", "NAME", "LABELS", "SUMMARY", "CREATED AT")
			tbl.WriteHeader()
			for _, o := range observations {
				summary := ""
				if o.Summary != nil {
					summary = *o.Summary
					summary = truncateText(summary, 40)
				}
				tbl.WriteRow(valueOrEmpty(o.UUID), valueOrEmpty(o.Name), strings.Join(o.Labels, ","), summary, valueOrEmpty(o.CreatedAt))
			}
			if page.RawResponse.Next != nil && *page.RawResponse.Next != "" {
				output.Info("Next cursor: %s", *page.RawResponse.Next)
			}
			return tbl.Flush()
		}

		return output.Print(map[string]any{itemsOutputKey: observations, nextCursorOutputKey: page.RawResponse.Next})
	},
}

var observationGetCmd = &cobra.Command{
	Use:   "get <observation-uuid>",
	Short: "Get observation details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uuid := args[0]
		graphUUID, err := graphUUIDFromFlags(cmd)
		if err != nil {
			return err
		}

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		obs, err := c.Graph.Observation.Get(context.Background(), graphUUID, uuid)
		if err != nil {
			return fmt.Errorf("getting observation: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("FIELD", "VALUE")
			tbl.WriteHeader()
			tbl.WriteRow("UUID", valueOrEmpty(obs.UUID))
			tbl.WriteRow("Name", valueOrEmpty(obs.Name))
			if len(obs.Labels) > 0 {
				tbl.WriteRow("Labels", strings.Join(obs.Labels, ","))
			}
			if obs.Summary != nil {
				tbl.WriteRow("Summary", *obs.Summary)
			}
			if len(obs.EpisodeUUIDs) > 0 {
				tbl.WriteRow("Episode UUIDs", strings.Join(obs.EpisodeUUIDs, ","))
			}
			tbl.WriteRow("Created At", valueOrEmpty(obs.CreatedAt))
			return tbl.Flush()
		}

		return output.Print(obs)
	},
}

func init() {
	rootCmd.AddCommand(observationCmd)
	observationCmd.AddCommand(observationListCmd)
	observationCmd.AddCommand(observationGetCmd)

	observationListCmd.Flags().String("user", "", "User UUID for a user graph")
	observationListCmd.Flags().String("graph", "", "Graph UUID for a standalone graph")
	observationListCmd.Flags().Int("limit", 50, "Maximum number of results to return")
	observationListCmd.Flags().String("cursor", "", "UUID cursor for pagination (last UUID from previous page)")
	observationGetCmd.Flags().String("user", "", "User UUID for a user graph")
	observationGetCmd.Flags().String("graph", "", "Graph UUID for a standalone graph")
}
