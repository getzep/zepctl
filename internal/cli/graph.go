package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/getzep/zep-go/v4"
	graph "github.com/getzep/zep-go/v4/graph"
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
	Use:   listCmdUse,
	Short: "List graphs",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		limit, _ := cmd.Flags().GetInt("limit")
		cursor, _ := cmd.Flags().GetString("cursor")
		orderBy, _ := cmd.Flags().GetString("order-by")
		asc, _ := cmd.Flags().GetBool("asc")

		req := &zep.GraphListRequest{}
		if limit > 0 {
			req.PageSize = zep.Int(limit)
		}
		if cursor != "" {
			req.Cursor = zep.String(cursor)
		}
		if orderBy != "" {
			value := zep.GraphListRequestOrderBy(orderBy)
			req.OrderBy = &value
		}
		if cmd.Flags().Changed("asc") {
			order := zep.GraphListRequestOrder("desc")
			if asc {
				order = "asc"
			}
			req.Order = &order
		}

		graphs, err := c.Graph.List(context.Background(), req)
		if err != nil {
			return fmt.Errorf("listing graphs: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("UUID", "GRAPH ID", "NAME", "CREATED AT")
			tbl.WriteHeader()
			for _, g := range graphs.Results {
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
			writeNextCursor(graphs.RawResponse.Next)
			return tbl.Flush()
		}

		return output.Print(map[string]any{"items": graphs.Results, "next_cursor": graphs.RawResponse.Next})
	},
}

var graphCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new graph",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")
		description, _ := cmd.Flags().GetString("description")

		graph, err := c.Graph.Create(context.Background(), &zep.CreateGraphRequest{
			Name:        zep.String(name),
			Description: zep.String(description),
		})
		if err != nil {
			return fmt.Errorf("creating graph: %w", err)
		}

		if graph.UUID != nil {
			output.Info("Created graph %s", *graph.UUID)
		}
		return output.Print(graph)
	},
}

var graphDeleteCmd = &cobra.Command{
	Use:   "delete <graph-uuid>",
	Short: "Delete a graph",
	Long:  `Delete a graph. To delete a user graph, use 'zepctl user delete' instead.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		graphUUID := args[0]
		force, _ := cmd.Flags().GetBool("force")

		if !force {
			fmt.Printf("Delete graph %q? [y/N]: ", graphUUID)
			if !confirmYesNo() {
				output.Info("Aborted")
				return nil
			}
		}

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		result, err := c.Graph.Delete(context.Background(), graphUUID)
		if err != nil {
			return fmt.Errorf("deleting graph: %w", err)
		}

		wait, _ := cmd.Flags().GetBool("wait")
		timeout, _ := cmd.Flags().GetDuration("timeout")
		if result.Task != nil && result.Task.UUID != nil {
			output.Info("Deletion task: %s", *result.Task.UUID)
			if wait {
				return waitForTask(c, *result.Task.UUID, timeout, defaultTaskPollInterval)
			}
		}
		return nil
	},
}

var graphCloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone a graph",
	Long:  `Clone a user graph or standalone graph to a new graph UUID.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sourceUser, _ := cmd.Flags().GetString("source-user")
		sourceGraph, _ := cmd.Flags().GetString("source-graph")

		if sourceUser == "" && sourceGraph == "" {
			return fmt.Errorf("either --source-user or --source-graph is required")
		}

		if sourceUser != "" && sourceGraph != "" {
			return fmt.Errorf("--source-user and --source-graph are mutually exclusive")
		}

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		sourceUUID := sourceGraph
		if sourceUser != "" {
			sourceUUID = sourceUser
		}

		resp, err := c.Graph.Clone(context.Background(), sourceUUID, zep.CloneGraphRequest{})
		if err != nil {
			return fmt.Errorf("cloning graph: %w", err)
		}

		if resp.Graph != nil && resp.Graph.UUID != nil {
			output.Info("Cloned to graph: %s", *resp.Graph.UUID)
		}
		wait, _ := cmd.Flags().GetBool("wait")
		timeout, _ := cmd.Flags().GetDuration("timeout")
		if resp.Task != nil && resp.Task.UUID != nil {
			output.Info("Clone task: %s", *resp.Task.UUID)
			if wait {
				return waitForTask(c, *resp.Task.UUID, timeout, defaultTaskPollInterval)
			}
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
	Type          string `json:"type"`
	Data          string `json:"data"`
	ReferenceTime string `json:"reference_time,omitempty"`
}

var graphAddCmd = &cobra.Command{
	Use:   "add <graph-uuid>",
	Short: "Add data to a graph",
	Long:  `Add text, JSON, or message data to a graph or user graph.`,
	Args: func(cmd *cobra.Command, args []string) error {
		userUUID, _ := cmd.Flags().GetString("user")
		if len(args) > 1 || (userUUID == "" && len(args) != 1) || (userUUID != "" && len(args) != 0) {
			return fmt.Errorf("provide either a graph UUID argument or --user <user-uuid>")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		userID, _ := cmd.Flags().GetString("user")
		dataType, _ := cmd.Flags().GetString("type")
		dataStr, _ := cmd.Flags().GetString("data")
		file, _ := cmd.Flags().GetString("file")
		useStdin, _ := cmd.Flags().GetBool("stdin")
		batch, _ := cmd.Flags().GetBool("batch")
		wait, _ := cmd.Flags().GetBool("wait")

		graphUUID := ""
		if len(args) == 1 {
			graphUUID = args[0]
		}

		c, err := client.NewForCommand(cmd)
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
				data, err = readStdinAll()
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

			var items []*zep.BatchItemInput
			for _, e := range input.Episodes {
				dataType := zep.BatchItemInputDataType(e.Type)
				item := &zep.BatchItemInput{
					Type:     zep.BatchItemInputTypeGraphEpisode,
					Data:     zep.String(e.Data),
					DataType: &dataType,
				}
				if e.ReferenceTime != "" {
					item.ReferenceTime = zep.String(e.ReferenceTime)
				}
				if userID != "" {
					item.UserUUID = zep.String(userID)
				} else {
					item.GraphUUID = zep.String(graphUUID)
				}
				items = append(items, item)
			}

			createdBatch, err := c.Batch.Create(context.Background(), &zep.CreateBatchRequest{})
			if err != nil {
				return fmt.Errorf("creating graph batch: %w", err)
			}
			if createdBatch.UUID == nil {
				return fmt.Errorf("creating graph batch: response did not include a batch UUID")
			}
			if _, err := c.Batch.AddItems(context.Background(), *createdBatch.UUID, &zep.AddBatchItemsRequest{Items: items}); err != nil {
				return fmt.Errorf("adding graph batch items: %w", err)
			}
			resp, err := c.Batch.Process(context.Background(), *createdBatch.UUID)
			if err != nil {
				return fmt.Errorf("processing graph batch: %w", err)
			}

			if resp.Task != nil && resp.Task.UUID != nil {
				output.Info("Batch task: %s", *resp.Task.UUID)
				if wait {
					if err := waitForTask(c, *resp.Task.UUID, defaultTaskTimeout, defaultTaskPollInterval); err != nil {
						return err
					}
				}
			} else if wait {
				if err := waitForBatch(c, *createdBatch.UUID, defaultTaskTimeout); err != nil {
					return err
				}
			}
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
			data, err := readStdinAll()
			if err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}
			dataContent = string(data)
		} else {
			return fmt.Errorf("--data, --file, or --stdin is required")
		}

		episodeType := graph.AddEpisodeRequestType(dataType)
		req := &graph.AddEpisodeRequest{
			Data: dataContent,
			Type: &episodeType,
		}
		graphUUIDForAdd := graphUUID
		if userID != "" {
			graphUUIDForAdd = userID
		}
		resp, err := c.Graph.Episode.Add(context.Background(), graphUUIDForAdd, req)
		if err != nil {
			return fmt.Errorf("adding data: %w", err)
		}

		if resp.Task != nil && resp.Task.UUID != nil {
			output.Info("Ingestion task: %s", *resp.Task.UUID)
			if wait {
				if err := waitForTask(c, *resp.Task.UUID, defaultTaskTimeout, defaultTaskPollInterval); err != nil {
					return err
				}
			}
		}
		output.Info("Added data to graph")
		return output.Print(resp.Episode)
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
		sourceLabelsStr, _ := cmd.Flags().GetString("source-labels")
		targetLabelsStr, _ := cmd.Flags().GetString("target-labels")

		if userID == "" && graphID == "" {
			return fmt.Errorf("either --user or --graph is required")
		}
		if userID != "" && graphID != "" {
			return fmt.Errorf("--user and --graph are mutually exclusive")
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

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		graphUUID := graphID
		if userID != "" {
			graphUUID = userID
		}

		edge := &zep.EdgeInput{
			Fact:       fact,
			FactName:   factName,
			SourceNode: &zep.EdgeNodeRef{Name: zep.String(sourceNodeName)},
			TargetNode: &zep.EdgeNodeRef{Name: zep.String(targetNodeName)},
		}
		if validAt != "" {
			edge.ValidAt = zep.String(validAt)
		}
		if invalidAt != "" {
			edge.InvalidAt = zep.String(invalidAt)
		}

		if sourceLabelsStr != "" {
			labels := strings.Split(sourceLabelsStr, ",")
			if len(labels) > 1 {
				return fmt.Errorf("--source-labels accepts a single value; got %d", len(labels))
			}
			edge.SourceNode.Labels = labels
		}
		if targetLabelsStr != "" {
			labels := strings.Split(targetLabelsStr, ",")
			if len(labels) > 1 {
				return fmt.Errorf("--target-labels accepts a single value; got %d", len(labels))
			}
			edge.TargetNode.Labels = labels
		}

		// Parse source node attributes
		if sourceAttrsStr != "" {
			var sourceAttrs map[string]interface{}
			if err := json.Unmarshal([]byte(sourceAttrsStr), &sourceAttrs); err != nil {
				return fmt.Errorf("parsing source-attrs: %w", err)
			}
			edge.SourceNode.Attributes = sourceAttrs
		}

		// Parse edge attributes
		if edgeAttrsStr != "" {
			var edgeAttrs map[string]interface{}
			if err := json.Unmarshal([]byte(edgeAttrsStr), &edgeAttrs); err != nil {
				return fmt.Errorf("parsing edge-attrs: %w", err)
			}
			edge.Attributes = edgeAttrs
		}

		// Parse target node attributes
		if targetAttrsStr != "" {
			var targetAttrs map[string]interface{}
			if err := json.Unmarshal([]byte(targetAttrsStr), &targetAttrs); err != nil {
				return fmt.Errorf("parsing target-attrs: %w", err)
			}
			edge.TargetNode.Attributes = targetAttrs
		}

		resp, err := c.Graph.Edge.Add(context.Background(), graphUUID, &graph.AddEdgesRequest{
			Deduplicate: zep.Bool(true),
			Edges:       []*zep.EdgeInput{edge},
		})
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
	Long:  `Search a graph by scope. Use --user or --graph with a UUID.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]
		userID, _ := cmd.Flags().GetString("user")
		graphUUID, _ := cmd.Flags().GetString("graph")
		scope, _ := cmd.Flags().GetString("scope")
		limit, _ := cmd.Flags().GetInt("limit")
		cursor, _ := cmd.Flags().GetString("cursor")
		reranker, _ := cmd.Flags().GetString("reranker")
		mmrLambda, _ := cmd.Flags().GetFloat64("mmr-lambda")
		maxCharacters, _ := cmd.Flags().GetInt("max-characters")
		includeResults, _ := cmd.Flags().GetBool("return-raw-results")
		excludeNodeLabels, _ := cmd.Flags().GetString("exclude-node-labels")
		excludeEdgeTypes, _ := cmd.Flags().GetString("exclude-edge-types")
		nodeLabels, _ := cmd.Flags().GetString("node-labels")
		edgeTypes, _ := cmd.Flags().GetString("edge-types")
		propertyFilterArgs, _ := cmd.Flags().GetStringArray("property-filter")
		dateFilterArgs, _ := cmd.Flags().GetStringArray("date-filter")

		if userID == "" && graphUUID == "" {
			return fmt.Errorf("either --user or --graph is required")
		}
		if userID != "" && graphUUID != "" {
			return fmt.Errorf("--user and --graph are mutually exclusive")
		}
		if userID != "" {
			graphUUID = userID
		}
		scopeFlags, ok := graphSearchScopeFlags[scope]
		if !ok {
			return fmt.Errorf("invalid scope %q", scope)
		}
		for _, name := range graphSearchFlags {
			if cmd.Flags().Changed(name) && !scopeFlags[name] {
				return fmt.Errorf("--%s is not supported with scope %s", name, scope)
			}
		}

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}
		if scope == "auto" {
			req := &zep.GraphContextRequest{Query: query}
			if cmd.Flags().Changed("max-characters") {
				req.MaxCharacters = zep.Int(maxCharacters)
			}
			if cmd.Flags().Changed("return-raw-results") {
				req.IncludeResults = zep.Bool(includeResults)
			}
			resp, err := c.Graph.GetContext(context.Background(), graphUUID, req)
			if err != nil {
				return fmt.Errorf("getting graph context: %w", err)
			}
			return output.Print(resp)
		}

		if scope == "edges" {
			req := &zep.SearchEdgesRequest{Query: query, PageSize: zep.Int(limit)}
			filters := &zep.EdgeSearchFilters{}
			if cmd.Flags().Changed("edge-types") {
				filters.EdgeTypes = splitFlagValues(edgeTypes)
			}
			if cmd.Flags().Changed("exclude-edge-types") {
				filters.ExcludeEdgeTypes = splitFlagValues(excludeEdgeTypes)
			}
			if len(propertyFilterArgs) > 0 {
				filters.PropertyFilters, err = parsePropertyFilters(propertyFilterArgs)
				if err != nil {
					return err
				}
			}
			if len(dateFilterArgs) > 0 {
				filters.DateFilters, err = parseDateFilters(dateFilterArgs)
				if err != nil {
					return err
				}
			}
			if cmd.Flags().Changed("edge-types") || cmd.Flags().Changed("exclude-edge-types") || len(propertyFilterArgs) > 0 || len(dateFilterArgs) > 0 {
				req.Filters = filters
			}
			if cursor != "" {
				req.Cursor = zep.String(cursor)
			}
			if reranker != "" {
				value := zep.SearchEdgesRequestReranker(reranker)
				req.Reranker = &value
			}
			if cmd.Flags().Changed("mmr-lambda") {
				req.MmrLambda = zep.Float64(mmrLambda)
			}
			page, err := c.Graph.SearchEdges(context.Background(), graphUUID, req)
			if err != nil {
				return fmt.Errorf("searching edges: %w", err)
			}
			return output.Print(map[string]any{"items": page.Results, "next_cursor": page.RawResponse.Next})
		}
		if scope == "nodes" {
			req := &zep.SearchNodesRequest{Query: query, PageSize: zep.Int(limit)}
			filters := &zep.NodeSearchFilters{}
			if cmd.Flags().Changed("node-labels") {
				filters.NodeLabels = splitFlagValues(nodeLabels)
			}
			if cmd.Flags().Changed("exclude-node-labels") {
				filters.ExcludeNodeLabels = splitFlagValues(excludeNodeLabels)
			}
			if len(propertyFilterArgs) > 0 {
				filters.PropertyFilters, err = parsePropertyFilters(propertyFilterArgs)
				if err != nil {
					return err
				}
			}
			if cmd.Flags().Changed("node-labels") || cmd.Flags().Changed("exclude-node-labels") || len(propertyFilterArgs) > 0 {
				req.Filters = filters
			}
			if cursor != "" {
				req.Cursor = zep.String(cursor)
			}
			if reranker != "" {
				value := zep.SearchNodesRequestReranker(reranker)
				req.Reranker = &value
			}
			if cmd.Flags().Changed("mmr-lambda") {
				req.MmrLambda = zep.Float64(mmrLambda)
			}
			page, err := c.Graph.SearchNodes(context.Background(), graphUUID, req)
			if err != nil {
				return fmt.Errorf("searching nodes: %w", err)
			}
			return output.Print(map[string]any{"items": page.Results, "next_cursor": page.RawResponse.Next})
		}
		if scope == "episodes" {
			req := &zep.SearchEpisodesRequest{Query: query, PageSize: zep.Int(limit)}
			if cursor != "" {
				req.Cursor = zep.String(cursor)
			}
			if reranker != "" {
				value := zep.SearchEpisodesRequestReranker(reranker)
				req.Reranker = &value
			}
			if cmd.Flags().Changed("mmr-lambda") {
				req.MmrLambda = zep.Float64(mmrLambda)
			}
			page, err := c.Graph.SearchEpisodes(context.Background(), graphUUID, req)
			if err != nil {
				return fmt.Errorf("searching episodes: %w", err)
			}
			return output.Print(map[string]any{"items": page.Results, "next_cursor": page.RawResponse.Next})
		}
		if scope == "observations" {
			req := &zep.SearchObservationsRequest{Query: query, PageSize: zep.Int(limit)}
			if cursor != "" {
				req.Cursor = zep.String(cursor)
			}
			if reranker != "" {
				value := zep.SearchObservationsRequestReranker(reranker)
				req.Reranker = &value
			}
			if cmd.Flags().Changed("mmr-lambda") {
				req.MmrLambda = zep.Float64(mmrLambda)
			}
			page, err := c.Graph.SearchObservations(context.Background(), graphUUID, req)
			if err != nil {
				return fmt.Errorf("searching observations: %w", err)
			}
			return output.Print(map[string]any{"items": page.Results, "next_cursor": page.RawResponse.Next})
		}
		req := &zep.SearchThreadSummariesRequest{Query: query, PageSize: zep.Int(limit)}
		if cursor != "" {
			req.Cursor = zep.String(cursor)
		}
		if reranker != "" {
			value := zep.SearchThreadSummariesRequestReranker(reranker)
			req.Reranker = &value
		}
		if cmd.Flags().Changed("mmr-lambda") {
			req.MmrLambda = zep.Float64(mmrLambda)
		}
		page, err := c.Graph.SearchThreadSummaries(context.Background(), graphUUID, req)
		if err != nil {
			return fmt.Errorf("searching thread summaries: %w", err)
		}
		return output.Print(map[string]any{"items": page.Results, "next_cursor": page.RawResponse.Next})
	},
}

func graphUUIDFromFlags(cmd *cobra.Command) (string, error) {
	graphUUID, _ := cmd.Flags().GetString("graph")
	userUUID, _ := cmd.Flags().GetString("user")
	if graphUUID != "" && userUUID != "" {
		return "", fmt.Errorf("--graph and --user are mutually exclusive")
	}
	if userUUID != "" {
		graphUUID = userUUID
	}
	if graphUUID == "" {
		return "", fmt.Errorf("either --graph or --user is required")
	}
	return graphUUID, nil
}

func valueOrEmpty[T ~string](value *T) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

func writeNextCursor(cursor *string) {
	if cursor != nil && *cursor != "" {
		output.Info("Next cursor: %s", *cursor)
	}
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
	graphListCmd.Flags().Int("limit", 50, "Maximum number of results")
	graphListCmd.Flags().String("cursor", "", "Cursor from the previous page")
	graphListCmd.Flags().String("order-by", "", "Sort by UUID or created time")
	graphListCmd.Flags().Bool("asc", false, "Sort in ascending order")

	graphCreateCmd.Flags().String("name", "", "Graph name")
	graphCreateCmd.Flags().String("description", "", "Graph description")

	// Delete flags
	graphDeleteCmd.Flags().Bool("force", false, "Skip confirmation prompt")
	graphDeleteCmd.Flags().Bool("wait", false, "Wait for asynchronous deletion")
	graphDeleteCmd.Flags().Duration("timeout", defaultTaskTimeout, "Maximum time to wait")

	// Clone flags
	graphCloneCmd.Flags().String("source-user", "", "Source user UUID")
	graphCloneCmd.Flags().String("source-graph", "", "Source graph UUID")
	graphCloneCmd.Flags().Bool("wait", false, "Wait for clone operation to complete")
	graphCloneCmd.Flags().Duration("timeout", defaultTaskTimeout, "Maximum time to wait")

	// Add flags
	graphAddCmd.Flags().String("type", "text", "Data type: text, json, message")
	graphAddCmd.Flags().String("data", "", "Inline data string")
	graphAddCmd.Flags().String("file", "", "Path to data file")
	graphAddCmd.Flags().Bool("stdin", false, "Read data from stdin")
	graphAddCmd.Flags().String("user", "", "User UUID for a user graph")
	graphAddCmd.Flags().Bool("batch", false, "Enable batch processing (up to 20 episodes)")
	graphAddCmd.Flags().Bool("wait", false, "Wait for ingestion to complete")

	// Add-fact flags
	graphAddFactCmd.Flags().String("user", "", "User UUID for a user graph")
	graphAddFactCmd.Flags().String("graph", "", "Graph UUID for a standalone graph")
	graphAddFactCmd.Flags().String("fact", "", "The fact relating the two nodes (required)")
	graphAddFactCmd.Flags().String("fact-name", "", "Edge name, should be UPPER_SNAKE_CASE (required)")
	graphAddFactCmd.Flags().String("source-node", "", "Source node name (required)")
	graphAddFactCmd.Flags().String("target-node", "", "Target node name (required)")
	graphAddFactCmd.Flags().String("valid-at", "", "When the fact becomes true (ISO 8601)")
	graphAddFactCmd.Flags().String("invalid-at", "", "When the fact stops being true (ISO 8601)")
	graphAddFactCmd.Flags().String("source-attrs", "", "Source node attributes as JSON")
	graphAddFactCmd.Flags().String("edge-attrs", "", "Edge attributes as JSON")
	graphAddFactCmd.Flags().String("target-attrs", "", "Target node attributes as JSON")
	graphAddFactCmd.Flags().String("source-labels", "", "Label for the source node (single value; API accepts at most one)")
	graphAddFactCmd.Flags().String("target-labels", "", "Label for the target node (single value; API accepts at most one)")

	// Search flags
	graphSearchCmd.Flags().String("user", "", "User UUID for a user graph")
	graphSearchCmd.Flags().String("graph", "", "Graph UUID for a standalone graph")
	graphSearchCmd.Flags().String("scope", "edges", "Search scope: edges, nodes, episodes, observations, thread_summaries, auto")
	graphSearchCmd.Flags().Int("limit", 10, "Maximum results")
	graphSearchCmd.Flags().String("cursor", "", "Cursor from the previous page")
	graphSearchCmd.Flags().String("reranker", "", "Reranker: rrf, mmr, cross_encoder")
	graphSearchCmd.Flags().Float64("mmr-lambda", 0, "MMR diversity/relevance balance from 0 to 1")
	graphSearchCmd.Flags().Int("max-characters", 0, "Max total characters across selected results (scope=auto, max 50000)")
	graphSearchCmd.Flags().Bool("return-raw-results", false, "Include raw results when scope is auto")
	graphSearchCmd.Flags().String("exclude-node-labels", "", "Comma-separated node labels to exclude (scope=nodes)")
	graphSearchCmd.Flags().String("exclude-edge-types", "", "Comma-separated edge types to exclude (scope=edges)")
	graphSearchCmd.Flags().String("node-labels", "", "Comma-separated node labels to include (scope=nodes)")
	graphSearchCmd.Flags().String("edge-types", "", "Comma-separated edge types to include (scope=edges)")
	graphSearchCmd.Flags().StringArray("property-filter", nil, "Property filter (scope=edges or nodes): property:op:value or property:IS NULL")
	graphSearchCmd.Flags().StringArray("date-filter", nil, "Date filter (scope=edges): field:op:date or field:IS NULL")
}

var graphSearchFlags = []string{
	"limit", "cursor", "reranker", "mmr-lambda", "max-characters", "return-raw-results",
	"exclude-node-labels", "exclude-edge-types", "node-labels", "edge-types", "property-filter", "date-filter",
}

var graphSearchScopeFlags = map[string]map[string]bool{
	"edges": {
		"limit": true, "cursor": true, "reranker": true, "mmr-lambda": true,
		"exclude-edge-types": true, "edge-types": true, "property-filter": true, "date-filter": true,
	},
	"nodes": {
		"limit": true, "cursor": true, "reranker": true, "mmr-lambda": true,
		"exclude-node-labels": true, "node-labels": true, "property-filter": true,
	},
	"episodes": {
		"limit": true, "cursor": true, "reranker": true, "mmr-lambda": true,
	},
	"observations": {
		"limit": true, "cursor": true, "reranker": true, "mmr-lambda": true,
	},
	"thread_summaries": {
		"limit": true, "cursor": true, "reranker": true, "mmr-lambda": true,
	},
	"auto": {
		"max-characters": true, "return-raw-results": true,
	},
}

func splitFlagValues(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	var values []string
	for _, part := range strings.Split(value, ",") {
		if value := strings.TrimSpace(part); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func parsePropertyFilters(filters []string) ([]*zep.PropertyFilter, error) {
	result := make([]*zep.PropertyFilter, 0, len(filters))
	for _, filter := range filters {
		parsed, err := parsePropertyFilter(filter)
		if err != nil {
			return nil, err
		}
		result = append(result, parsed)
	}
	return result, nil
}

func parsePropertyFilter(filter string) (*zep.PropertyFilter, error) {
	if parsed, matched, err := parseNullPropertyFilter(filter); matched {
		return parsed, err
	}

	parts := strings.SplitN(filter, ":", 3)
	if len(parts) != 3 || strings.TrimSpace(parts[0]) == "" {
		return nil, fmt.Errorf("invalid property filter format: %q (expected property:operator:value)", filter)
	}
	operator, err := parseSearchComparisonOperator(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid operator in property filter %q: %w", filter, err)
	}
	name := strings.TrimSpace(parts[0])
	rawValue := strings.TrimSpace(parts[2])
	if rawValue == "" || rawValue == "null" {
		if operator != zep.PropertyFilterOperatorEq {
			return nil, fmt.Errorf("property filter %q requires a non-null value", filter)
		}
		operator = zep.PropertyFilterOperatorIsNull
		return &zep.PropertyFilter{PropertyName: zep.String(name), Operator: operator.Ptr()}, nil
	}

	value, err := parsePropertyFilterValue(rawValue)
	if err != nil {
		return nil, err
	}
	return &zep.PropertyFilter{
		PropertyName: zep.String(name),
		Operator:     operator.Ptr(),
		Value:        value,
	}, nil
}

func parseNullPropertyFilter(filter string) (*zep.PropertyFilter, bool, error) {
	for _, operator := range []string{"IS NOT NULL", "IS NULL"} {
		if !strings.Contains(filter, ":"+operator) {
			continue
		}
		parts := strings.SplitN(filter, ":"+operator, 2)
		name := strings.TrimSpace(parts[0])
		if name == "" || strings.TrimSpace(parts[1]) != "" {
			return nil, true, fmt.Errorf("invalid property filter format: %q", filter)
		}
		value, err := zep.NewPropertyFilterOperatorFromString(strings.ToLower(strings.ReplaceAll(operator, " ", "_")))
		if err != nil {
			return nil, true, err
		}
		return &zep.PropertyFilter{PropertyName: zep.String(name), Operator: value.Ptr()}, true, nil
	}
	return nil, false, nil
}

func parsePropertyFilterValue(rawValue string) (*zep.PropertyFilterValue, error) {
	var value any
	if rawValue == "true" || rawValue == "false" {
		value = rawValue == "true"
	} else {
		var number json.Number
		if err := json.Unmarshal([]byte(rawValue), &number); err == nil {
			if integer, err := number.Int64(); err == nil {
				value = integer
			} else if decimal, err := number.Float64(); err == nil {
				value = decimal
			} else {
				value = rawValue
			}
		} else {
			value = rawValue
		}
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encoding property filter value: %w", err)
	}
	var propertyValue zep.PropertyFilterValue
	if err := json.Unmarshal(encoded, &propertyValue); err != nil {
		return nil, fmt.Errorf("parsing property filter value: %w", err)
	}
	return &propertyValue, nil
}

func parseSearchComparisonOperator(operator string) (zep.PropertyFilterOperator, error) {
	switch operator {
	case "=", "==":
		return zep.PropertyFilterOperatorEq, nil
	case "<>", "!=":
		return zep.PropertyFilterOperatorNe, nil
	case ">":
		return zep.PropertyFilterOperatorGt, nil
	case "<":
		return zep.PropertyFilterOperatorLt, nil
	case ">=":
		return zep.PropertyFilterOperatorGte, nil
	case "<=":
		return zep.PropertyFilterOperatorLte, nil
	default:
		return "", fmt.Errorf("unknown operator: %s", operator)
	}
}

func parseDateFilters(filters []string) (*zep.DateFilters, error) {
	result := &zep.DateFilters{}
	for _, filter := range filters {
		parsed, err := parseDateFilter(filter)
		if err != nil {
			return nil, err
		}
		result.AnyOf = append(result.AnyOf, &zep.DateFilterGroup{AllOf: []*zep.DateFilter{parsed}})
	}
	return result, nil
}

func parseDateFilter(filter string) (*zep.DateFilter, error) {
	var field string
	var operator zep.DateFilterOperator
	var rawValue string
	for _, nullOperator := range []string{"IS NOT NULL", "IS NULL"} {
		if !strings.Contains(filter, ":"+nullOperator) {
			continue
		}
		parts := strings.SplitN(filter, ":"+nullOperator, 2)
		field = strings.TrimSpace(parts[0])
		if field == "" || strings.TrimSpace(parts[1]) != "" {
			return nil, fmt.Errorf("invalid date filter format: %q", filter)
		}
		operator = zep.DateFilterOperator(strings.ToLower(strings.ReplaceAll(nullOperator, " ", "_")))
		break
	}
	if operator == "" {
		parts := strings.SplitN(filter, ":", 3)
		if len(parts) != 3 || strings.TrimSpace(parts[0]) == "" {
			return nil, fmt.Errorf("invalid date filter format: %q (expected field:operator:date)", filter)
		}
		field = strings.TrimSpace(parts[0])
		parsedOperator, err := parseSearchComparisonOperator(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid operator in date filter %q: %w", filter, err)
		}
		operator = zep.DateFilterOperator(parsedOperator)
		rawValue = strings.TrimSpace(parts[2])
	}

	dateField, err := zep.NewDateFilterFieldFromString(field)
	if err != nil {
		return nil, fmt.Errorf("invalid date filter field in %q: %w", filter, err)
	}
	result := &zep.DateFilter{Field: dateField.Ptr(), Operator: operator.Ptr()}
	if rawValue != "" {
		value, err := time.Parse(time.RFC3339, rawValue)
		if err != nil {
			value, err = time.Parse("2006-01-02", rawValue)
			if err != nil {
				return nil, fmt.Errorf("invalid date in filter %q: %w", filter, err)
			}
		}
		result.Value = &value
	}
	return result, nil
}
