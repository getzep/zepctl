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

		if sourceUser != "" && sourceGraph != "" {
			return fmt.Errorf("--source-user and --source-graph are mutually exclusive")
		}

		if sourceUser != "" && targetGraph != "" {
			return fmt.Errorf("--target-graph cannot be used with --source-user; use --target-user instead")
		}

		if sourceGraph != "" && targetUser != "" {
			return fmt.Errorf("--target-user cannot be used with --source-graph; use --target-graph instead")
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

var graphAddFactCmd = &cobra.Command{
	Use:   "add-fact",
	Short: "Add a fact triple to a graph",
	Long: `Add a fact triple (source node -> edge -> target node) to a graph.

Attributes can be specified as JSON objects for source node, edge, and target node.
Example: --source-attrs '{"type": "Person", "age": 30}'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		userID, _ := cmd.Flags().GetString("user")
		graphID, _ := cmd.Flags().GetString("graph")
		fact, _ := cmd.Flags().GetString("fact")
		factName, _ := cmd.Flags().GetString("fact-name")
		sourceNodeName, _ := cmd.Flags().GetString("source-node")
		targetNodeName, _ := cmd.Flags().GetString("target-node")
		validAt, _ := cmd.Flags().GetString("valid-at")
		invalidAt, _ := cmd.Flags().GetString("invalid-at")
		sourceAttrsStr, _ := cmd.Flags().GetString("source-attrs")
		edgeAttrsStr, _ := cmd.Flags().GetString("edge-attrs")
		targetAttrsStr, _ := cmd.Flags().GetString("target-attrs")

		if userID == "" && graphID == "" {
			return fmt.Errorf("either --user or --graph is required")
		}

		if fact == "" {
			return fmt.Errorf("--fact is required")
		}
		if factName == "" {
			return fmt.Errorf("--fact-name is required")
		}
		if sourceNodeName == "" {
			return fmt.Errorf("--source-node is required")
		}
		if targetNodeName == "" {
			return fmt.Errorf("--target-node is required")
		}

		c, err := client.New()
		if err != nil {
			return err
		}

		req := &zep.AddTripleRequest{
			Fact:           fact,
			FactName:       factName,
			SourceNodeName: zep.String(sourceNodeName),
			TargetNodeName: zep.String(targetNodeName),
		}

		if userID != "" {
			req.UserID = zep.String(userID)
		} else {
			req.GraphID = zep.String(graphID)
		}

		if validAt != "" {
			req.ValidAt = zep.String(validAt)
		}

		if invalidAt != "" {
			req.InvalidAt = zep.String(invalidAt)
		}

		// Parse source node attributes
		if sourceAttrsStr != "" {
			var sourceAttrs map[string]interface{}
			if err := json.Unmarshal([]byte(sourceAttrsStr), &sourceAttrs); err != nil {
				return fmt.Errorf("parsing source-attrs: %w", err)
			}
			req.SourceNodeAttributes = sourceAttrs
		}

		// Parse edge attributes
		if edgeAttrsStr != "" {
			var edgeAttrs map[string]interface{}
			if err := json.Unmarshal([]byte(edgeAttrsStr), &edgeAttrs); err != nil {
				return fmt.Errorf("parsing edge-attrs: %w", err)
			}
			req.EdgeAttributes = edgeAttrs
		}

		// Parse target node attributes
		if targetAttrsStr != "" {
			var targetAttrs map[string]interface{}
			if err := json.Unmarshal([]byte(targetAttrsStr), &targetAttrs); err != nil {
				return fmt.Errorf("parsing target-attrs: %w", err)
			}
			req.TargetNodeAttributes = targetAttrs
		}

		resp, err := c.Graph.AddFactTriple(context.Background(), req)
		if err != nil {
			return fmt.Errorf("adding fact triple: %w", err)
		}

		output.Info("Added fact triple")
		return output.Print(resp)
	},
}

var graphSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search a graph",
	Long: `Search a user graph or standalone graph for edges, nodes, or episodes.

Property filters allow filtering by node/edge attributes:
  --property-filter "property_name:operator:value"

  Operators: =, <>, >, <, >=, <=, IS NULL, IS NOT NULL

  Examples:
    --property-filter "age:>:30"
    --property-filter "status:=:active"
    --property-filter "deleted_at:IS NULL"
    --property-filter "verified:IS NOT NULL"

Date filters allow filtering by date fields (created_at, valid_at, invalid_at, expired_at):
  --date-filter "field:operator:date"

  Examples:
    --date-filter "created_at:>:2024-01-01"
    --date-filter "valid_at:IS NULL"
    --date-filter "invalid_at:IS NOT NULL"`,
	Args: cobra.ExactArgs(1),
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
		nodeLabels, _ := cmd.Flags().GetString("node-labels")
		edgeTypes, _ := cmd.Flags().GetString("edge-types")
		propertyFilters, _ := cmd.Flags().GetStringArray("property-filter")
		dateFilters, _ := cmd.Flags().GetStringArray("date-filter")

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

		if cmd.Flags().Changed("mmr-lambda") {
			req.MmrLambda = zep.Float64(mmrLambda)
		}

		if cmd.Flags().Changed("min-score") {
			req.MinScore = zep.Float64(minScore)
		}

		// Build search filters
		hasFilters := excludeNodeLabels != "" || excludeEdgeTypes != "" ||
			nodeLabels != "" || edgeTypes != "" ||
			len(propertyFilters) > 0 || len(dateFilters) > 0

		if hasFilters {
			if req.SearchFilters == nil {
				req.SearchFilters = &zep.SearchFilters{}
			}

			if excludeNodeLabels != "" {
				req.SearchFilters.ExcludeNodeLabels = strings.Split(excludeNodeLabels, ",")
			}
			if excludeEdgeTypes != "" {
				req.SearchFilters.ExcludeEdgeTypes = strings.Split(excludeEdgeTypes, ",")
			}
			if nodeLabels != "" {
				req.SearchFilters.NodeLabels = strings.Split(nodeLabels, ",")
			}
			if edgeTypes != "" {
				req.SearchFilters.EdgeTypes = strings.Split(edgeTypes, ",")
			}

			// Parse property filters
			if len(propertyFilters) > 0 {
				parsedFilters, err := parsePropertyFilters(propertyFilters)
				if err != nil {
					return err
				}
				req.SearchFilters.PropertyFilters = parsedFilters
			}

			// Parse date filters
			if len(dateFilters) > 0 {
				if err := parseDateFilters(dateFilters, req.SearchFilters); err != nil {
					return err
				}
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

// parsePropertyFilters parses property filter strings into PropertyFilter objects.
// Format: "property_name:operator:value" or "property_name:IS NULL" / "property_name:IS NOT NULL".
func parsePropertyFilters(filters []string) ([]*zep.PropertyFilter, error) {
	var result []*zep.PropertyFilter

	for _, f := range filters {
		pf, err := parsePropertyFilter(f)
		if err != nil {
			return nil, err
		}
		result = append(result, pf)
	}

	return result, nil
}

func parsePropertyFilter(filter string) (*zep.PropertyFilter, error) {
	// Check for IS NULL / IS NOT NULL operators first
	if strings.Contains(filter, ":IS NOT NULL") {
		parts := strings.SplitN(filter, ":IS NOT NULL", 2)
		if len(parts) < 1 || parts[0] == "" {
			return nil, fmt.Errorf("invalid property filter format: %q", filter)
		}
		return &zep.PropertyFilter{
			PropertyName:       parts[0],
			ComparisonOperator: zep.ComparisonOperatorIsNotNull,
		}, nil
	}

	if strings.Contains(filter, ":IS NULL") {
		parts := strings.SplitN(filter, ":IS NULL", 2)
		if len(parts) < 1 || parts[0] == "" {
			return nil, fmt.Errorf("invalid property filter format: %q", filter)
		}
		return &zep.PropertyFilter{
			PropertyName:       parts[0],
			ComparisonOperator: zep.ComparisonOperatorIsNull,
		}, nil
	}

	// Parse format: "property_name:operator:value"
	parts := strings.SplitN(filter, ":", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid property filter format: %q (expected property_name:operator:value)", filter)
	}

	propName := parts[0]
	opStr := parts[1]
	valueStr := parts[2]

	op, err := parseComparisonOperator(opStr)
	if err != nil {
		return nil, fmt.Errorf("invalid operator in property filter %q: %w", filter, err)
	}

	// Parse the value - try to detect type
	var value interface{}
	if valueStr == "true" {
		value = true
	} else if valueStr == "false" {
		value = false
	} else if valueStr == "null" || valueStr == "" {
		value = nil
	} else if i, err := json.Number(valueStr).Int64(); err == nil {
		value = i
	} else if f, err := json.Number(valueStr).Float64(); err == nil {
		value = f
	} else {
		value = valueStr
	}

	return &zep.PropertyFilter{
		PropertyName:       propName,
		ComparisonOperator: op,
		PropertyValue:      value,
	}, nil
}

func parseComparisonOperator(op string) (zep.ComparisonOperator, error) {
	switch op {
	case "=", "==":
		return zep.ComparisonOperatorEquals, nil
	case "<>", "!=":
		return zep.ComparisonOperatorNotEquals, nil
	case ">":
		return zep.ComparisonOperatorGreaterThan, nil
	case "<":
		return zep.ComparisonOperatorLessThan, nil
	case ">=":
		return zep.ComparisonOperatorGreaterThanEqual, nil
	case "<=":
		return zep.ComparisonOperatorLessThanEqual, nil
	case "IS NULL":
		return zep.ComparisonOperatorIsNull, nil
	case "IS NOT NULL":
		return zep.ComparisonOperatorIsNotNull, nil
	default:
		return "", fmt.Errorf("unknown operator: %s", op)
	}
}

// parseDateFilters parses date filter strings and adds them to SearchFilters.
// Format: "field:operator:date" or "field:IS NULL" / "field:IS NOT NULL".
// Fields: created_at, valid_at, invalid_at, expired_at.
func parseDateFilters(filters []string, sf *zep.SearchFilters) error {
	for _, f := range filters {
		if err := parseDateFilter(f, sf); err != nil {
			return err
		}
	}
	return nil
}

func parseDateFilter(filter string, sf *zep.SearchFilters) error {
	// Check for IS NULL / IS NOT NULL operators first
	if strings.Contains(filter, ":IS NOT NULL") {
		parts := strings.SplitN(filter, ":IS NOT NULL", 2)
		if len(parts) < 1 || parts[0] == "" {
			return fmt.Errorf("invalid date filter format: %q", filter)
		}
		return addDateFilter(parts[0], zep.ComparisonOperatorIsNotNull, nil, sf)
	}

	if strings.Contains(filter, ":IS NULL") {
		parts := strings.SplitN(filter, ":IS NULL", 2)
		if len(parts) < 1 || parts[0] == "" {
			return fmt.Errorf("invalid date filter format: %q", filter)
		}
		return addDateFilter(parts[0], zep.ComparisonOperatorIsNull, nil, sf)
	}

	// Parse format: "field:operator:date"
	parts := strings.SplitN(filter, ":", 3)
	if len(parts) != 3 {
		return fmt.Errorf("invalid date filter format: %q (expected field:operator:date)", filter)
	}

	field := parts[0]
	opStr := parts[1]
	dateStr := parts[2]

	op, err := parseComparisonOperator(opStr)
	if err != nil {
		return fmt.Errorf("invalid operator in date filter %q: %w", filter, err)
	}

	return addDateFilter(field, op, &dateStr, sf)
}

func addDateFilter(field string, op zep.ComparisonOperator, date *string, sf *zep.SearchFilters) error {
	df := &zep.DateFilter{
		ComparisonOperator: op,
		Date:               date,
	}

	// Date filters use a 2D array where outer = OR, inner = AND
	// For simplicity, each filter creates a new OR group with single element
	switch field {
	case "created_at":
		sf.CreatedAt = append(sf.CreatedAt, []*zep.DateFilter{df})
	case "valid_at":
		sf.ValidAt = append(sf.ValidAt, []*zep.DateFilter{df})
	case "invalid_at":
		sf.InvalidAt = append(sf.InvalidAt, []*zep.DateFilter{df})
	case "expired_at":
		sf.ExpiredAt = append(sf.ExpiredAt, []*zep.DateFilter{df})
	default:
		return fmt.Errorf("unknown date field: %s (valid: created_at, valid_at, invalid_at, expired_at)", field)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(graphCmd)
	graphCmd.AddCommand(graphListCmd)
	graphCmd.AddCommand(graphCreateCmd)
	graphCmd.AddCommand(graphDeleteCmd)
	graphCmd.AddCommand(graphCloneCmd)
	graphCmd.AddCommand(graphAddCmd)
	graphCmd.AddCommand(graphAddFactCmd)
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

	// Add-fact flags
	graphAddFactCmd.Flags().String("user", "", "Add to user graph")
	graphAddFactCmd.Flags().String("graph", "", "Add to standalone graph")
	graphAddFactCmd.Flags().String("fact", "", "The fact relating the two nodes (required)")
	graphAddFactCmd.Flags().String("fact-name", "", "Edge name, should be UPPER_SNAKE_CASE (required)")
	graphAddFactCmd.Flags().String("source-node", "", "Source node name (required)")
	graphAddFactCmd.Flags().String("target-node", "", "Target node name (required)")
	graphAddFactCmd.Flags().String("valid-at", "", "When the fact becomes true (ISO 8601)")
	graphAddFactCmd.Flags().String("invalid-at", "", "When the fact stops being true (ISO 8601)")
	graphAddFactCmd.Flags().String("source-attrs", "", "Source node attributes as JSON")
	graphAddFactCmd.Flags().String("edge-attrs", "", "Edge attributes as JSON")
	graphAddFactCmd.Flags().String("target-attrs", "", "Target node attributes as JSON")

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
	graphSearchCmd.Flags().String("node-labels", "", "Comma-separated node labels to include")
	graphSearchCmd.Flags().String("edge-types", "", "Comma-separated edge types to include")
	graphSearchCmd.Flags().StringArray("property-filter", nil, "Property filter (can be repeated): property:op:value or property:IS NULL")
	graphSearchCmd.Flags().StringArray("date-filter", nil, "Date filter (can be repeated): field:op:date or field:IS NULL")
}
