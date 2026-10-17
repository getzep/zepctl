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

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Manage graph nodes",
	Long:  `List, get, and inspect nodes in a graph.`,
}

var nodeListCmd = &cobra.Command{
	Use:   listCmdUse,
	Short: "List nodes",
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

		req := &zepgraph.NodeListRequest{}
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
		nodes, err := c.Graph.Node.List(context.Background(), graphUUID, req)
		if err != nil {
			return fmt.Errorf("listing nodes: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("UUID", "NAME", "LABEL", "SUMMARY")
			tbl.WriteHeader()
			for _, n := range nodes.Results {
				label := ""
				if len(n.Labels) > 0 {
					label = n.Labels[0]
				}
				summary := ""
				if n.Summary != nil {
					summary = *n.Summary
				}
				if len(summary) > 40 {
					summary = summary[:40] + "..."
				}
				tbl.WriteRow(valueOrEmpty(n.UUID), valueOrEmpty(n.Name), label, summary)
			}
			writeNextCursor(nodes.RawResponse.Next)
			return tbl.Flush()
		}

		return output.Print(map[string]any{itemsOutputKey: nodes.Results, nextCursorOutputKey: nodes.RawResponse.Next})
	},
}

var nodeGetCmd = &cobra.Command{
	Use:   "get <node-uuid>",
	Short: "Get node details",
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

		node, err := c.Graph.Node.Get(context.Background(), graphUUID, uuid)
		if err != nil {
			return fmt.Errorf("getting node: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("FIELD", "VALUE")
			tbl.WriteHeader()
			tbl.WriteRow("UUID", valueOrEmpty(node.UUID))
			tbl.WriteRow("Name", valueOrEmpty(node.Name))
			if len(node.Labels) > 0 {
				tbl.WriteRow("Labels", fmt.Sprintf("%v", node.Labels))
			}
			tbl.WriteRow("Summary", valueOrEmpty(node.Summary))
			tbl.WriteRow("Created At", valueOrEmpty(node.CreatedAt))
			return tbl.Flush()
		}

		return output.Print(node)
	},
}

var nodeEdgesCmd = &cobra.Command{
	Use:   "edges <node-uuid>",
	Short: "Get edges for a node",
	Long:  `Returns all entity edges connected to the specified node.`,
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

		limit, _ := cmd.Flags().GetInt("limit")
		cursor, _ := cmd.Flags().GetString("cursor")
		request := &zepgraph.EdgeListRequest{Filters: &zep.EdgeListFilters{ConnectedNodeUUIDs: []string{uuid}}}
		if limit > 0 {
			request.PageSize = zep.Int(limit)
		}
		if cursor != "" {
			request.Cursor = zep.String(cursor)
		}
		edges, err := c.Graph.Edge.List(context.Background(), graphUUID, request)
		if err != nil {
			return fmt.Errorf("getting node edges: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("UUID", "NAME", "FACT", "SOURCE", "TARGET")
			tbl.WriteHeader()
			for _, e := range edges.Results {
				fact := valueOrEmpty(e.Fact)
				if len(fact) > 40 {
					fact = fact[:40] + "..."
				}
				tbl.WriteRow(valueOrEmpty(e.UUID), valueOrEmpty(e.Name), fact, valueOrEmpty(e.SourceNodeUUID), valueOrEmpty(e.TargetNodeUUID))
			}
			writeNextCursor(edges.RawResponse.Next)
			return tbl.Flush()
		}

		return output.Print(map[string]any{itemsOutputKey: edges.Results, nextCursorOutputKey: edges.RawResponse.Next})
	},
}

var nodeEpisodesCmd = &cobra.Command{
	Use:   "episodes <node-uuid>",
	Short: "Get episodes that mention a node",
	Long:  `Returns all episodes that mention the specified node.`,
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

		limit, _ := cmd.Flags().GetInt("limit")
		cursor, _ := cmd.Flags().GetString("cursor")
		request := &zepgraph.EpisodeListRequest{Filters: &zep.EpisodeListFilters{MentionedNodeUUIDs: []string{uuid}}}
		if limit > 0 {
			request.PageSize = zep.Int(limit)
		}
		if cursor != "" {
			request.Cursor = zep.String(cursor)
		}
		episodes, err := c.Graph.Episode.List(context.Background(), graphUUID, request)
		if err != nil {
			return fmt.Errorf("getting node episodes: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("UUID", "SOURCE", "CONTENT", "CREATED AT")
			tbl.WriteHeader()
			for _, ep := range episodes.Results {
				source := ""
				if ep.Source != nil {
					source = string(*ep.Source)
				}
				content := valueOrEmpty(ep.Content)
				if len(content) > 40 {
					content = content[:40] + "..."
				}
				tbl.WriteRow(valueOrEmpty(ep.UUID), source, content, valueOrEmpty(ep.CreatedAt))
			}
			writeNextCursor(episodes.RawResponse.Next)
			return tbl.Flush()
		}

		return output.Print(map[string]any{itemsOutputKey: episodes.Results, nextCursorOutputKey: episodes.RawResponse.Next})
	},
}

var nodeUpdateCmd = &cobra.Command{
	Use:   "update <node-uuid>",
	Short: "Update a node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uuid := args[0]
		graphUUID, err := graphUUIDFromFlags(cmd)
		if err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")
		summary, _ := cmd.Flags().GetString("summary")
		attrsStr, _ := cmd.Flags().GetString("attrs")

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		req := &zepgraph.PatchNodeRequest{}

		if name != "" {
			req.Name = zep.String(name)
		}
		if summary != "" {
			req.Summary = zep.String(summary)
		}
		if attrsStr != "" {
			var attrs map[string]any
			if err := json.Unmarshal([]byte(attrsStr), &attrs); err != nil {
				return fmt.Errorf("parsing attrs: %w", err)
			}
			req.Attributes = attrs
		}

		node, err := c.Graph.Node.Update(context.Background(), graphUUID, uuid, req)
		if err != nil {
			return fmt.Errorf("updating node: %w", err)
		}

		output.Info("Updated node %q", uuid)
		return output.Print(node)
	},
}

var nodeDeleteCmd = &cobra.Command{
	Use:   "delete <node-uuid>",
	Short: "Delete a node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uuid := args[0]
		graphUUID, err := graphUUIDFromFlags(cmd)
		if err != nil {
			return err
		}
		force, _ := cmd.Flags().GetBool("force")

		if !force {
			fmt.Printf("Delete node %q? [y/N]: ", uuid)
			if !confirmYesNo() {
				output.Info("Aborted")
				return nil
			}
		}

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		result, err := c.Graph.Node.Delete(context.Background(), graphUUID, uuid)
		if err != nil {
			return fmt.Errorf("deleting node: %w", err)
		}

		wait, _ := cmd.Flags().GetBool("wait")
		timeout, _ := cmd.Flags().GetDuration("timeout")
		return reportDeletion(c, "node", uuid, result.Task, wait, timeout)
	},
}

func init() {
	rootCmd.AddCommand(nodeCmd)
	nodeCmd.AddCommand(nodeListCmd)
	nodeCmd.AddCommand(nodeGetCmd)
	nodeCmd.AddCommand(nodeUpdateCmd)
	nodeCmd.AddCommand(nodeEdgesCmd)
	nodeCmd.AddCommand(nodeEpisodesCmd)
	nodeCmd.AddCommand(nodeDeleteCmd)

	// List flags
	nodeListCmd.Flags().String("user", "", "User UUID for a user graph")
	nodeListCmd.Flags().String("graph", "", "Graph UUID for a standalone graph")
	nodeListCmd.Flags().Int("limit", 50, "Maximum number of results to return")
	nodeListCmd.Flags().String("cursor", "", "UUID cursor for pagination (last UUID from previous page)")
	for _, command := range []*cobra.Command{nodeEdgesCmd, nodeEpisodesCmd} {
		command.Flags().Int("limit", 50, "Maximum number of results")
		command.Flags().String("cursor", "", "Cursor from the previous page")
	}
	for _, command := range []*cobra.Command{nodeGetCmd, nodeEdgesCmd, nodeEpisodesCmd, nodeUpdateCmd, nodeDeleteCmd} {
		command.Flags().String("user", "", "User UUID for a user graph")
		command.Flags().String("graph", "", "Graph UUID for a standalone graph")
	}

	// Update flags
	nodeUpdateCmd.Flags().String("name", "", "Updated name")
	nodeUpdateCmd.Flags().String("summary", "", "Updated summary")
	nodeUpdateCmd.Flags().String("attrs", "", "Attributes as JSON (merged with existing, set key to null to delete)")

	// Delete flags
	nodeDeleteCmd.Flags().Bool("force", false, "Skip confirmation prompt")
	nodeDeleteCmd.Flags().Bool("wait", false, "Wait for asynchronous deletion")
	nodeDeleteCmd.Flags().Duration("timeout", defaultTaskTimeout, "Maximum time to wait")
}
