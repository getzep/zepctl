package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/getzep/zep-go/v3"
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

		req := &zep.GraphObservationsRequest{}
		if limit > 0 {
			req.Limit = zep.Int(limit)
		}
		if cursor != "" {
			req.UUIDCursor = zep.String(cursor)
		}

		var observations []*zep.DerivedNode

		if userID != "" {
			result, err := c.Graph.Observation.GetByUserID(context.Background(), userID, req)
			if err != nil {
				return fmt.Errorf("listing observations: %w", err)
			}
			observations = result
		} else {
			result, err := c.Graph.Observation.GetByGraphID(context.Background(), graphID, req)
			if err != nil {
				return fmt.Errorf("listing observations: %w", err)
			}
			observations = result
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("UUID", "NAME", "LABELS", "SUMMARY", "CREATED AT")
			tbl.WriteHeader()
			for _, o := range observations {
				summary := ""
				if o.Summary != nil {
					summary = *o.Summary
					if len(summary) > 40 {
						summary = summary[:40] + "..."
					}
				}
				tbl.WriteRow(o.UUID, o.Name, strings.Join(o.Labels, ","), summary, o.CreatedAt)
			}
			return tbl.Flush()
		}

		return output.Print(observations)
	},
}

var observationGetCmd = &cobra.Command{
	Use:   "get <uuid>",
	Short: "Get observation details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uuid := args[0]

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		obs, err := c.Graph.Observation.Get(context.Background(), uuid)
		if err != nil {
			return fmt.Errorf("getting observation: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("FIELD", "VALUE")
			tbl.WriteHeader()
			tbl.WriteRow("UUID", obs.UUID)
			tbl.WriteRow("Name", obs.Name)
			if len(obs.Labels) > 0 {
				tbl.WriteRow("Labels", strings.Join(obs.Labels, ","))
			}
			if obs.Summary != nil {
				tbl.WriteRow("Summary", *obs.Summary)
			}
			if len(obs.EpisodeIDs) > 0 {
				tbl.WriteRow("Episode IDs", strings.Join(obs.EpisodeIDs, ","))
			}
			tbl.WriteRow("Created At", obs.CreatedAt)
			return tbl.Flush()
		}

		return output.Print(obs)
	},
}

func init() {
	rootCmd.AddCommand(observationCmd)
	observationCmd.AddCommand(observationListCmd)
	observationCmd.AddCommand(observationGetCmd)

	observationListCmd.Flags().String("user", "", "List observations for user graph")
	observationListCmd.Flags().String("graph", "", "List observations for standalone graph")
	observationListCmd.Flags().Int("limit", 50, "Maximum number of results to return")
	observationListCmd.Flags().String("cursor", "", "UUID cursor for pagination (last UUID from previous page)")
}
