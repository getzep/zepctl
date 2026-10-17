package cli

import (
	"context"
	"fmt"

	"github.com/getzep/zep-go/v4"
	"github.com/getzep/zep-go/v4/graph"
	"github.com/getzep/zepctl/internal/client"
	"github.com/getzep/zepctl/internal/output"
	"github.com/spf13/cobra"
)

var episodeCmd = &cobra.Command{
	Use:   "episode",
	Short: "Manage graph episodes",
	Long:  `List, get, and delete episodes in a graph.`,
}

var episodeListCmd = &cobra.Command{
	Use:   listCmdUse,
	Short: "List episodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		userID, _ := cmd.Flags().GetString("user")
		graphID, _ := cmd.Flags().GetString("graph")
		lastN, _ := cmd.Flags().GetInt("last")

		if userID == "" && graphID == "" {
			return fmt.Errorf("either --user or --graph is required")
		}

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		graphUUID := graphID
		if userID != "" {
			graphUUID = userID
		}
		limit, _ := cmd.Flags().GetInt("limit")
		cursor, _ := cmd.Flags().GetString("cursor")
		if lastN > 0 {
			limit = lastN
		}
		req := &graph.EpisodeListRequest{}
		if limit > 0 {
			req.PageSize = zep.Int(limit)
		}
		if cursor != "" {
			req.Cursor = zep.String(cursor)
		}
		episodePage, err := c.Graph.Episode.List(context.Background(), graphUUID, req)
		if err != nil {
			return fmt.Errorf("listing episodes: %w", err)
		}
		episodes := episodePage.Results

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("UUID", "SOURCE", "ROLE", "CONTENT", "CREATED AT")
			tbl.WriteHeader()
			for _, ep := range episodes {
				source := ""
				if ep.Source != nil {
					source = string(*ep.Source)
				}
				role := ""
				if ep.Role != nil {
					role = string(*ep.Role)
				}
				content := valueOrEmpty(ep.Content)
				if len(content) > 40 {
					content = content[:40] + "..."
				}
				tbl.WriteRow(valueOrEmpty(ep.UUID), source, role, content, valueOrEmpty(ep.CreatedAt))
			}
			if episodePage.RawResponse.Next != nil && *episodePage.RawResponse.Next != "" {
				output.Info("Next cursor: %s", *episodePage.RawResponse.Next)
			}
			return tbl.Flush()
		}

		return output.Print(map[string]any{itemsOutputKey: episodes, nextCursorOutputKey: episodePage.RawResponse.Next})
	},
}

var episodeGetCmd = &cobra.Command{
	Use:   "get <episode-uuid>",
	Short: "Get episode details",
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

		episode, err := c.Graph.Episode.Get(context.Background(), graphUUID, uuid)
		if err != nil {
			return fmt.Errorf("getting episode: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("FIELD", "VALUE")
			tbl.WriteHeader()
			tbl.WriteRow("UUID", valueOrEmpty(episode.UUID))
			if episode.Source != nil {
				tbl.WriteRow("Source", string(*episode.Source))
			}
			if episode.SourceDescription != nil {
				tbl.WriteRow("Source Description", *episode.SourceDescription)
			}
			if episode.Role != nil {
				tbl.WriteRow("Role", string(*episode.Role))
			}
			tbl.WriteRow("Content", valueOrEmpty(episode.Content))
			tbl.WriteRow("Created At", valueOrEmpty(episode.CreatedAt))
			return tbl.Flush()
		}

		return output.Print(episode)
	},
}

var episodeMentionsCmd = &cobra.Command{
	Use:   "mentions <episode-uuid>",
	Short: "Get nodes and edges mentioned in an episode",
	Long:  `Returns nodes and edges mentioned in the specified episode. Use a separate cursor for each list.`,
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
		nodesCursor, _ := cmd.Flags().GetString("nodes-cursor")
		edgesCursor, _ := cmd.Flags().GetString("edges-cursor")
		_, err = c.Graph.Episode.Get(context.Background(), graphUUID, uuid)
		if err != nil {
			return fmt.Errorf("getting episode: %w", err)
		}

		nodeRequest := &graph.NodeListRequest{Filters: &zep.NodeListFilters{EpisodeUUIDs: []string{uuid}}}
		if limit > 0 {
			nodeRequest.PageSize = zep.Int(limit)
		}
		if nodesCursor != "" {
			nodeRequest.Cursor = zep.String(nodesCursor)
		}
		nodes, err := c.Graph.Node.List(context.Background(), graphUUID, nodeRequest)
		if err != nil {
			return fmt.Errorf("listing episode nodes: %w", err)
		}
		edgeRequest := &graph.EdgeListRequest{Filters: &zep.EdgeListFilters{EpisodeUUIDs: []string{uuid}}}
		if limit > 0 {
			edgeRequest.PageSize = zep.Int(limit)
		}
		if edgesCursor != "" {
			edgeRequest.Cursor = zep.String(edgesCursor)
		}
		edges, err := c.Graph.Edge.List(context.Background(), graphUUID, edgeRequest)
		if err != nil {
			return fmt.Errorf("listing episode edges: %w", err)
		}
		return output.Print(map[string]any{
			"nodes": nodes.Results, "nodes_next_cursor": nodes.RawResponse.Next,
			"edges": edges.Results, "edges_next_cursor": edges.RawResponse.Next,
		})
	},
}

var episodeDeleteCmd = &cobra.Command{
	Use:   "delete <episode-uuid>",
	Short: "Delete an episode",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uuid := args[0]
		force, _ := cmd.Flags().GetBool("force")

		if !force {
			fmt.Printf("Delete episode %q? [y/N]: ", uuid)
			if !confirmYesNo() {
				output.Info("Aborted")
				return nil
			}
		}

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		graphUUID, err := graphUUIDFromFlags(cmd)
		if err != nil {
			return err
		}
		result, err := c.Graph.Episode.Delete(context.Background(), graphUUID, uuid)
		if err != nil {
			return fmt.Errorf("deleting episode: %w", err)
		}

		wait, _ := cmd.Flags().GetBool("wait")
		timeout, _ := cmd.Flags().GetDuration("timeout")
		return reportDeletion(c, "episode", uuid, result.Task, wait, timeout)
	},
}

func init() {
	rootCmd.AddCommand(episodeCmd)
	episodeCmd.AddCommand(episodeListCmd)
	episodeCmd.AddCommand(episodeGetCmd)
	episodeCmd.AddCommand(episodeMentionsCmd)
	episodeCmd.AddCommand(episodeDeleteCmd)

	// List flags
	episodeListCmd.Flags().String("user", "", "User UUID for a user graph")
	episodeListCmd.Flags().String("graph", "", "Graph UUID for a standalone graph")
	episodeListCmd.Flags().Int("last", 0, "Get last N episodes")
	episodeListCmd.Flags().Int("limit", 50, "Maximum number of results")
	episodeListCmd.Flags().String("cursor", "", "Cursor from the previous page")
	for _, command := range []*cobra.Command{episodeGetCmd, episodeMentionsCmd, episodeDeleteCmd} {
		command.Flags().String("user", "", "User UUID for a user graph")
		command.Flags().String("graph", "", "Graph UUID for a standalone graph")
	}
	episodeMentionsCmd.Flags().Int("limit", 50, "Maximum number of results")
	episodeMentionsCmd.Flags().String("nodes-cursor", "", "Cursor for the node list")
	episodeMentionsCmd.Flags().String("edges-cursor", "", "Cursor for the edge list")

	// Delete flags
	episodeDeleteCmd.Flags().Bool("force", false, "Skip confirmation prompt")
	episodeDeleteCmd.Flags().Bool("wait", false, "Wait for asynchronous deletion")
	episodeDeleteCmd.Flags().Duration("timeout", defaultTaskTimeout, "Maximum time to wait")
}
