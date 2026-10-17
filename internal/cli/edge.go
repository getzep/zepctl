package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/getzep/zep-go/v4"
	zepgraph "github.com/getzep/zep-go/v4/graph"
	"github.com/getzep/zepctl/internal/client"
	"github.com/getzep/zepctl/internal/output"
	"github.com/spf13/cobra"
)

var edgeCmd = &cobra.Command{
	Use:   "edge",
	Short: "Manage graph edges",
	Long:  `List, get, and delete edges in a graph.`,
}

var edgeListCmd = &cobra.Command{
	Use:   listCmdUse,
	Short: "List edges",
	RunE: func(cmd *cobra.Command, args []string) error {
		userID, _ := cmd.Flags().GetString("user")
		graphID, _ := cmd.Flags().GetString("graph")

		if userID == "" && graphID == "" {
			return fmt.Errorf("either --user or --graph is required")
		}

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		limit, _ := cmd.Flags().GetInt("limit")
		cursor, _ := cmd.Flags().GetString("cursor")

		req := &zepgraph.EdgeListRequest{}
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
		edges, err := c.Graph.Edge.List(context.Background(), graphUUID, req)
		if err != nil {
			return fmt.Errorf("listing edges: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("UUID", "NAME", "FACT", "VALID AT", "INVALID AT")
			tbl.WriteHeader()
			for _, e := range edges.Results {
				fact := ""
				if e.Fact != nil {
					fact = *e.Fact
				}
				if len(fact) > 40 {
					fact = fact[:40] + "..."
				}
				validAt := ""
				if e.ValidAt != nil {
					validAt = *e.ValidAt
				}
				invalidAt := ""
				if e.InvalidAt != nil {
					invalidAt = *e.InvalidAt
				}
				tbl.WriteRow(valueOrEmpty(e.UUID), valueOrEmpty(e.Name), fact, validAt, invalidAt)
			}
			if edges.RawResponse.Next != nil && *edges.RawResponse.Next != "" {
				output.Info("Next cursor: %s", *edges.RawResponse.Next)
			}
			return tbl.Flush()
		}

		return output.Print(map[string]any{itemsOutputKey: edges.Results, nextCursorOutputKey: edges.RawResponse.Next})
	},
}

var edgeGetCmd = &cobra.Command{
	Use:   "get <edge-uuid>",
	Short: "Get edge details",
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

		edge, err := c.Graph.Edge.Get(context.Background(), graphUUID, uuid)
		if err != nil {
			return fmt.Errorf("getting edge: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("FIELD", "VALUE")
			tbl.WriteHeader()
			tbl.WriteRow("UUID", valueOrEmpty(edge.UUID))
			tbl.WriteRow("Name", valueOrEmpty(edge.Name))
			tbl.WriteRow("Fact", valueOrEmpty(edge.Fact))
			tbl.WriteRow("Source Node", valueOrEmpty(edge.SourceNodeUUID))
			tbl.WriteRow("Target Node", valueOrEmpty(edge.TargetNodeUUID))
			if edge.ValidAt != nil {
				tbl.WriteRow("Valid At", *edge.ValidAt)
			}
			if edge.InvalidAt != nil {
				tbl.WriteRow("Invalid At", *edge.InvalidAt)
			}
			tbl.WriteRow("Created At", valueOrEmpty(edge.CreatedAt))
			return tbl.Flush()
		}

		return output.Print(edge)
	},
}

var edgeUpdateCmd = &cobra.Command{
	Use:   "update <edge-uuid>",
	Short: "Update an edge",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uuid := args[0]
		graphUUID, err := graphUUIDFromFlags(cmd)
		if err != nil {
			return err
		}
		fact, _ := cmd.Flags().GetString("fact")
		attrsStr, _ := cmd.Flags().GetString("attrs")

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		req := &zepgraph.PatchEdgeRequest{}

		if fact != "" {
			req.Fact = zep.String(fact)
		}
		if attrsStr != "" {
			var attrs map[string]any
			if err := json.Unmarshal([]byte(attrsStr), &attrs); err != nil {
				return fmt.Errorf("parsing attrs: %w", err)
			}
			req.Attributes = attrs
		}

		edge, err := c.Graph.Edge.Update(context.Background(), graphUUID, uuid, req)
		if err != nil {
			return fmt.Errorf("updating edge: %w", err)
		}

		output.Info("Updated edge %q", uuid)
		return output.Print(edge)
	},
}

var edgeDeleteCmd = &cobra.Command{
	Use:   "delete <edge-uuid>",
	Short: "Delete an edge",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uuid := args[0]
		graphUUID, err := graphUUIDFromFlags(cmd)
		if err != nil {
			return err
		}
		force, _ := cmd.Flags().GetBool("force")

		if !force {
			fmt.Printf("Delete edge %q? [y/N]: ", uuid)
			if !confirmYesNo() {
				output.Info("Aborted")
				return nil
			}
		}

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		result, err := c.Graph.Edge.Delete(context.Background(), graphUUID, uuid)
		if err != nil {
			return fmt.Errorf("deleting edge: %w", err)
		}

		wait, _ := cmd.Flags().GetBool("wait")
		timeout, _ := cmd.Flags().GetDuration("timeout")
		return reportDeletion(c, "edge", uuid, result.Task, wait, timeout)
	},
}

func init() {
	rootCmd.AddCommand(edgeCmd)
	edgeCmd.AddCommand(edgeListCmd)
	edgeCmd.AddCommand(edgeGetCmd)
	edgeCmd.AddCommand(edgeUpdateCmd)
	edgeCmd.AddCommand(edgeDeleteCmd)

	// List flags
	edgeListCmd.Flags().String("user", "", "User UUID for a user graph")
	edgeListCmd.Flags().String("graph", "", "Graph UUID for a standalone graph")
	edgeListCmd.Flags().Int("limit", 50, "Maximum number of results to return")
	edgeListCmd.Flags().String("cursor", "", "UUID cursor for pagination (last UUID from previous page)")
	for _, command := range []*cobra.Command{edgeGetCmd, edgeUpdateCmd, edgeDeleteCmd} {
		command.Flags().String("user", "", "User UUID for a user graph")
		command.Flags().String("graph", "", "Graph UUID for a standalone graph")
	}

	// Update flags
	edgeUpdateCmd.Flags().String("fact", "", "Updated fact")
	edgeUpdateCmd.Flags().String("attrs", "", "Attributes as JSON (merged with existing, set key to null to delete)")

	// Delete flags
	edgeDeleteCmd.Flags().Bool("force", false, "Skip confirmation prompt")
	edgeDeleteCmd.Flags().Bool("wait", false, "Wait for asynchronous deletion")
	edgeDeleteCmd.Flags().Duration("timeout", defaultTaskTimeout, "Maximum time to wait")
}
