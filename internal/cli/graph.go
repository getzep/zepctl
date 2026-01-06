package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/getzep/zep-go/v3"
	"github.com/getzep/zepctl/internal/client"
	"github.com/getzep/zepctl/internal/output"
	"github.com/spf13/cobra"
)

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Manage graphs",
	Long:  `Create, list, delete, clone graphs and add data.`,
}

var graphListCmd = &cobra.Command{
	Use:   "list",
	Short: "List graphs",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}

		page, _ := cmd.Flags().GetInt("page")
		pageSize, _ := cmd.Flags().GetInt("page-size")

		graphs, err := c.Graph.ListAll(context.Background(), &zep.GraphListAllRequest{
			PageNumber: zep.Int(page),
			PageSize:   zep.Int(pageSize),
		})
		if err != nil {
			return fmt.Errorf("listing graphs: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("UUID", "GRAPH ID", "NAME", "CREATED AT")
			tbl.WriteHeader()
			for _, g := range graphs.Graphs {
				uuid := ""
				if g.UUID != nil {
					uuid = *g.UUID
				}
				graphID := ""
				if g.GraphID != nil {
					graphID = *g.GraphID
				}
				name := ""
				if g.Name != nil {
					name = *g.Name
				}
				createdAt := ""
				if g.CreatedAt != nil {
					createdAt = *g.CreatedAt
				}
				tbl.WriteRow(uuid, graphID, name, createdAt)
			}
			return tbl.Flush()
		}

		return output.Print(graphs)
	},
}

var graphCreateCmd = &cobra.Command{
	Use:   "create <graph-id>",
	Short: "Create a new graph",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		graphID := args[0]

		c, err := client.New()
		if err != nil {
			return err
		}

		graph, err := c.Graph.Create(context.Background(), &zep.CreateGraphRequest{
			GraphID: graphID,
		})
		if err != nil {
			return fmt.Errorf("creating graph: %w", err)
		}

		output.Info("Created graph %q", graphID)
		return output.Print(graph)
	},
}

var graphDeleteCmd = &cobra.Command{
	Use:   "delete <graph-id>",
	Short: "Delete a graph",
	Long:  `Delete a graph. To delete a user graph, use 'zepctl user delete' instead.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		graphID := args[0]
		force, _ := cmd.Flags().GetBool("force")

		if !force {
			fmt.Printf("Delete graph %q? [y/N]: ", graphID)
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				output.Info("Aborted")
				return nil
			}
		}

		c, err := client.New()
		if err != nil {
			return err
		}

		if _, err := c.Graph.Delete(context.Background(), graphID); err != nil {
			return fmt.Errorf("deleting graph: %w", err)
		}

		output.Info("Deleted graph %q", graphID)
		return nil
	},
}

var graphCloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone a graph",
	Long:  `Clone a user graph or standalone graph to a new ID.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sourceUser, _ := cmd.Flags().GetString("source-user")
		targetUser, _ := cmd.Flags().GetString("target-user")
		sourceGraph, _ := cmd.Flags().GetString("source-graph")
		targetGraph, _ := cmd.Flags().GetString("target-graph")

		if sourceUser == "" && sourceGraph == "" {
			return fmt.Errorf("either --source-user or --source-graph is required")
		}

		c, err := client.New()
		if err != nil {
			return err
		}

		req := &zep.CloneGraphRequest{}

		if sourceUser != "" {
			req.SourceUserID = zep.String(sourceUser)
			if targetUser != "" {
				req.TargetUserID = zep.String(targetUser)
			}
		} else {
			req.SourceGraphID = zep.String(sourceGraph)
			if targetGraph != "" {
				req.TargetGraphID = zep.String(targetGraph)
			}
		}

		resp, err := c.Graph.Clone(context.Background(), req)
		if err != nil {
			return fmt.Errorf("cloning graph: %w", err)
		}

		if resp.GraphID != nil {
			output.Info("Cloned to graph: %s", *resp.GraphID)
		} else if resp.UserID != nil {
			output.Info("Cloned to user: %s", *resp.UserID)
		}

		return output.Print(resp)
	},
}

// EpisodeInput represents the input format for batch episodes.
type EpisodeInput struct {
	Episodes []EpisodeData `json:"episodes"`
}

// EpisodeData represents a single episode for batch import.
type EpisodeData struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

var graphAddCmd = &cobra.Command{
	Use:   "add [graph-id]",
	Short: "Add data to a graph",
	Long:  `Add text, JSON, or message data to a graph or user graph.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		userID, _ := cmd.Flags().GetString("user")
		dataType, _ := cmd.Flags().GetString("type")
		dataStr, _ := cmd.Flags().GetString("data")
		file, _ := cmd.Flags().GetString("file")
		useStdin, _ := cmd.Flags().GetBool("stdin")
		batch, _ := cmd.Flags().GetBool("batch")
		wait, _ := cmd.Flags().GetBool("wait")

		var graphID string
		if len(args) > 0 {
			graphID = args[0]
		}

		if userID == "" && graphID == "" {
			return fmt.Errorf("either graph-id argument or --user flag is required")
		}

		c, err := client.New()
		if err != nil {
			return err
		}

		// Handle batch mode
		if batch {
			var data []byte
			if file != "" {
				data, err = os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
			} else if useStdin {
				data, err = io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
			} else {
				return fmt.Errorf("--file or --stdin is required for batch mode")
			}

			var input EpisodeInput
			if err := json.Unmarshal(data, &input); err != nil {
				return fmt.Errorf("parsing episodes: %w", err)
			}

			var episodes []*zep.EpisodeData
			for _, e := range input.Episodes {
				episodeType := zep.GraphDataType(e.Type)
				episodes = append(episodes, &zep.EpisodeData{
					Data: e.Data,
					Type: episodeType,
				})
			}

			req := &zep.AddDataBatchRequest{
				Episodes: episodes,
			}
			if userID != "" {
				req.UserID = zep.String(userID)
			} else {
				req.GraphID = zep.String(graphID)
			}

			resp, err := c.Graph.AddBatch(context.Background(), req)
			if err != nil {
				return fmt.Errorf("adding batch data: %w", err)
			}

			output.Info("Added %d episodes to graph", len(resp))
			if !wait {
				return output.Print(resp)
			}

			// If wait is requested, we just report completion since batch returns episodes directly
			return output.Print(resp)
		}

		// Single episode mode
		var dataContent string
		if dataStr != "" {
			dataContent = dataStr
		} else if file != "" {
			data, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}
			dataContent = string(data)
		} else if useStdin {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}
			dataContent = string(data)
		} else {
			return fmt.Errorf("--data, --file, or --stdin is required")
		}

		episodeType := zep.GraphDataType(dataType)
		req := &zep.AddDataRequest{
			Data: dataContent,
			Type: episodeType,
		}
		if userID != "" {
			req.UserID = zep.String(userID)
		} else {
			req.GraphID = zep.String(graphID)
		}

		resp, err := c.Graph.Add(context.Background(), req)
		if err != nil {
			return fmt.Errorf("adding data: %w", err)
		}

		output.Info("Added data to graph")
		return output.Print(resp)
	},
}

var graphSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search a graph",
	Long:  `Search a user graph or standalone graph for edges, nodes, or episodes.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		userID, _ := cmd.Flags().GetString("user")
		graphID, _ := cmd.Flags().GetString("graph")
		scope, _ := cmd.Flags().GetString("scope")
		limit, _ := cmd.Flags().GetInt("limit")
		reranker, _ := cmd.Flags().GetString("reranker")
		mmrLambda, _ := cmd.Flags().GetFloat64("mmr-lambda")
		minScore, _ := cmd.Flags().GetFloat64("min-score")
		excludeNodeLabels, _ := cmd.Flags().GetString("exclude-node-labels")
		excludeEdgeTypes, _ := cmd.Flags().GetString("exclude-edge-types")

		if userID == "" && graphID == "" {
			return fmt.Errorf("either --user or --graph is required")
		}

		c, err := client.New()
		if err != nil {
			return err
		}

		req := &zep.GraphSearchQuery{
			Query: query,
			Limit: zep.Int(limit),
		}

		if userID != "" {
			req.UserID = zep.String(userID)
		} else {
			req.GraphID = zep.String(graphID)
		}

		if scope != "" {
			s := zep.GraphSearchScope(scope)
			req.Scope = &s
		}

		if reranker != "" {
			r := zep.Reranker(reranker)
			req.Reranker = &r
		}

		if mmrLambda > 0 {
			req.MmrLambda = zep.Float64(mmrLambda)
		}

		if minScore > 0 {
			req.MinScore = zep.Float64(minScore)
		}

		if excludeNodeLabels != "" || excludeEdgeTypes != "" {
			req.SearchFilters = &zep.SearchFilters{}
			if excludeNodeLabels != "" {
				req.SearchFilters.NodeLabels = strings.Split(excludeNodeLabels, ",")
			}
			if excludeEdgeTypes != "" {
				req.SearchFilters.EdgeTypes = strings.Split(excludeEdgeTypes, ",")
			}
		}

		resp, err := c.Graph.Search(context.Background(), req)
		if err != nil {
			return fmt.Errorf("searching graph: %w", err)
		}

		if output.GetFormat() == output.FormatTable && scope == "edges" {
			tbl := output.NewTable("UUID", "FACT", "VALID AT", "INVALID AT")
			tbl.WriteHeader()
			for _, e := range resp.Edges {
				fact := e.Fact
				if len(fact) > 60 {
					fact = fact[:60] + "..."
				}
				validAt := ""
				if e.ValidAt != nil {
					validAt = *e.ValidAt
				}
				invalidAt := ""
				if e.InvalidAt != nil {
					invalidAt = *e.InvalidAt
				}
				tbl.WriteRow(e.UUID, fact, validAt, invalidAt)
			}
			return tbl.Flush()
		}

		if output.GetFormat() == output.FormatTable && scope == "nodes" {
			tbl := output.NewTable("UUID", "NAME", "SUMMARY")
			tbl.WriteHeader()
			for _, n := range resp.Nodes {
				summary := n.Summary
				if len(summary) > 50 {
					summary = summary[:50] + "..."
				}
				tbl.WriteRow(n.UUID, n.Name, summary)
			}
			return tbl.Flush()
		}

		return output.Print(resp)
	},
}

func init() {
	rootCmd.AddCommand(graphCmd)
	graphCmd.AddCommand(graphListCmd)
	graphCmd.AddCommand(graphCreateCmd)
	graphCmd.AddCommand(graphDeleteCmd)
	graphCmd.AddCommand(graphCloneCmd)
	graphCmd.AddCommand(graphAddCmd)
	graphCmd.AddCommand(graphSearchCmd)

	// List flags
	graphListCmd.Flags().Int("page", 1, "Page number")
	graphListCmd.Flags().Int("page-size", 50, "Results per page")

	// Delete flags
	graphDeleteCmd.Flags().Bool("force", false, "Skip confirmation prompt")

	// Clone flags
	graphCloneCmd.Flags().String("source-user", "", "Source user ID (for user graphs)")
	graphCloneCmd.Flags().String("target-user", "", "Target user ID (for user graphs)")
	graphCloneCmd.Flags().String("source-graph", "", "Source graph ID (for standalone graphs)")
	graphCloneCmd.Flags().String("target-graph", "", "Target graph ID (for standalone graphs)")
	graphCloneCmd.Flags().Bool("wait", false, "Wait for clone operation to complete")

	// Add flags
	graphAddCmd.Flags().String("type", "text", "Data type: text, json, message")
	graphAddCmd.Flags().String("data", "", "Inline data string")
	graphAddCmd.Flags().String("file", "", "Path to data file")
	graphAddCmd.Flags().Bool("stdin", false, "Read data from stdin")
	graphAddCmd.Flags().String("user", "", "Add to user graph instead of standalone graph")
	graphAddCmd.Flags().Bool("batch", false, "Enable batch processing (up to 20 episodes)")
	graphAddCmd.Flags().Bool("wait", false, "Wait for ingestion to complete")

	// Search flags
	graphSearchCmd.Flags().String("user", "", "Search user graph")
	graphSearchCmd.Flags().String("graph", "", "Search standalone graph")
	graphSearchCmd.Flags().String("scope", "edges", "Search scope: edges, nodes, episodes")
	graphSearchCmd.Flags().Int("limit", 10, "Maximum results")
	graphSearchCmd.Flags().String("reranker", "", "Reranker: rrf, mmr, cross_encoder")
	graphSearchCmd.Flags().Float64("mmr-lambda", 0, "MMR diversity/relevance balance (0-1)")
	graphSearchCmd.Flags().Float64("min-score", 0, "Minimum relevance score")
	graphSearchCmd.Flags().String("exclude-node-labels", "", "Comma-separated node labels to exclude")
	graphSearchCmd.Flags().String("exclude-edge-types", "", "Comma-separated edge types to exclude")
}
