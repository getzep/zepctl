package cli

import (
	"context"
	"fmt"

	"github.com/getzep/zep-go/v3"
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
	Use:   "list",
	Short: "List nodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		userID, _ := cmd.Flags().GetString("user")
		graphID, _ := cmd.Flags().GetString("graph")

		if userID == "" && graphID == "" {
			return fmt.Errorf("either --user or --graph is required")
		}

		c, err := client.New()
		if err != nil {
			return err
		}

		var nodes []*zep.EntityNode

		limit, _ := cmd.Flags().GetInt("limit")
		cursor, _ := cmd.Flags().GetString("cursor")

		req := &zep.GraphNodesRequest{}
		if limit > 0 {
			req.Limit = zep.Int(limit)
		}
		if cursor != "" {
			req.UUIDCursor = zep.String(cursor)
		}

		if userID != "" {
			result, err := c.Graph.Node.GetByUserID(context.Background(), userID, req)
			if err != nil {
				return fmt.Errorf("listing nodes: %w", err)
			}
			nodes = result
		} else {
			result, err := c.Graph.Node.GetByGraphID(context.Background(), graphID, req)
			if err != nil {
				return fmt.Errorf("listing nodes: %w", err)
			}
			nodes = result
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("UUID", "NAME", "LABEL", "SUMMARY")
			tbl.WriteHeader()
			for _, n := range nodes {
				label := ""
				if len(n.Labels) > 0 {
					label = n.Labels[0]
				}
				summary := n.Summary
				if len(summary) > 40 {
					summary = summary[:40] + "..."
				}
				tbl.WriteRow(n.UUID, n.Name, label, summary)
			}
			return tbl.Flush()
		}

		return output.Print(nodes)
	},
}

var nodeGetCmd = &cobra.Command{
	Use:   "get <uuid>",
	Short: "Get node details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uuid := args[0]

		c, err := client.New()
		if err != nil {
			return err
		}

		node, err := c.Graph.Node.Get(context.Background(), uuid)
		if err != nil {
			return fmt.Errorf("getting node: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("FIELD", "VALUE")
			tbl.WriteHeader()
			tbl.WriteRow("UUID", node.UUID)
			tbl.WriteRow("Name", node.Name)
			if len(node.Labels) > 0 {
				tbl.WriteRow("Labels", fmt.Sprintf("%v", node.Labels))
			}
			tbl.WriteRow("Summary", node.Summary)
			tbl.WriteRow("Created At", node.CreatedAt)
			return tbl.Flush()
		}

		return output.Print(node)
	},
}

var nodeEdgesCmd = &cobra.Command{
	Use:   "edges <uuid>",
	Short: "Get edges for a node",
	Long:  `Returns all entity edges connected to the specified node.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uuid := args[0]

		c, err := client.New()
		if err != nil {
			return err
		}

		edges, err := c.Graph.Node.GetEdges(context.Background(), uuid)
		if err != nil {
			return fmt.Errorf("getting node edges: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("UUID", "NAME", "FACT", "SOURCE", "TARGET")
			tbl.WriteHeader()
			for _, e := range edges {
				fact := e.Fact
				if len(fact) > 40 {
					fact = fact[:40] + "..."
				}
				tbl.WriteRow(e.UUID, e.Name, fact, e.SourceNodeUUID, e.TargetNodeUUID)
			}
			return tbl.Flush()
		}

		return output.Print(edges)
	},
}

var nodeEpisodesCmd = &cobra.Command{
	Use:   "episodes <uuid>",
	Short: "Get episodes that mention a node",
	Long:  `Returns all episodes that mention the specified node.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uuid := args[0]

		c, err := client.New()
		if err != nil {
			return err
		}

		episodes, err := c.Graph.Node.GetEpisodes(context.Background(), uuid)
		if err != nil {
			return fmt.Errorf("getting node episodes: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("UUID", "SOURCE", "CONTENT", "CREATED AT")
			tbl.WriteHeader()
			for _, ep := range episodes.Episodes {
				source := ""
				if ep.Source != nil {
					source = string(*ep.Source)
				}
				content := ep.Content
				if len(content) > 40 {
					content = content[:40] + "..."
				}
				tbl.WriteRow(ep.UUID, source, content, ep.CreatedAt)
			}
			return tbl.Flush()
		}

		return output.Print(episodes)
	},
}

func init() {
	rootCmd.AddCommand(nodeCmd)
	nodeCmd.AddCommand(nodeListCmd)
	nodeCmd.AddCommand(nodeGetCmd)
	nodeCmd.AddCommand(nodeEdgesCmd)
	nodeCmd.AddCommand(nodeEpisodesCmd)

	// List flags
	nodeListCmd.Flags().String("user", "", "List nodes for user graph")
	nodeListCmd.Flags().String("graph", "", "List nodes for standalone graph")
	nodeListCmd.Flags().Int("limit", 50, "Maximum number of results to return")
	nodeListCmd.Flags().String("cursor", "", "UUID cursor for pagination (last UUID from previous page)")
}
