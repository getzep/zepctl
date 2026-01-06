package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/getzep/zep-go/v3"
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
	Use:   "list",
	Short: "List edges",
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

		var edges []*zep.EntityEdge

		if userID != "" {
			result, err := c.Graph.Edge.GetByUserID(context.Background(), userID, &zep.GraphEdgesRequest{})
			if err != nil {
				return fmt.Errorf("listing edges: %w", err)
			}
			edges = result
		} else {
			result, err := c.Graph.Edge.GetByGraphID(context.Background(), graphID, &zep.GraphEdgesRequest{})
			if err != nil {
				return fmt.Errorf("listing edges: %w", err)
			}
			edges = result
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("UUID", "NAME", "FACT", "VALID AT", "INVALID AT")
			tbl.WriteHeader()
			for _, e := range edges {
				fact := e.Fact
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
				tbl.WriteRow(e.UUID, e.Name, fact, validAt, invalidAt)
			}
			return tbl.Flush()
		}

		return output.Print(edges)
	},
}

var edgeGetCmd = &cobra.Command{
	Use:   "get <uuid>",
	Short: "Get edge details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uuid := args[0]

		c, err := client.New()
		if err != nil {
			return err
		}

		edge, err := c.Graph.Edge.Get(context.Background(), uuid)
		if err != nil {
			return fmt.Errorf("getting edge: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("FIELD", "VALUE")
			tbl.WriteHeader()
			tbl.WriteRow("UUID", edge.UUID)
			tbl.WriteRow("Name", edge.Name)
			tbl.WriteRow("Fact", edge.Fact)
			tbl.WriteRow("Source Node", edge.SourceNodeUUID)
			tbl.WriteRow("Target Node", edge.TargetNodeUUID)
			if edge.ValidAt != nil {
				tbl.WriteRow("Valid At", *edge.ValidAt)
			}
			if edge.InvalidAt != nil {
				tbl.WriteRow("Invalid At", *edge.InvalidAt)
			}
			tbl.WriteRow("Created At", edge.CreatedAt)
			return tbl.Flush()
		}

		return output.Print(edge)
	},
}

var edgeDeleteCmd = &cobra.Command{
	Use:   "delete <uuid>",
	Short: "Delete an edge",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uuid := args[0]
		force, _ := cmd.Flags().GetBool("force")

		if !force {
			fmt.Printf("Delete edge %q? [y/N]: ", uuid)
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

		if _, err := c.Graph.Edge.Delete(context.Background(), uuid); err != nil {
			return fmt.Errorf("deleting edge: %w", err)
		}

		output.Info("Deleted edge %q", uuid)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(edgeCmd)
	edgeCmd.AddCommand(edgeListCmd)
	edgeCmd.AddCommand(edgeGetCmd)
	edgeCmd.AddCommand(edgeDeleteCmd)

	// List flags
	edgeListCmd.Flags().String("user", "", "List edges for user graph")
	edgeListCmd.Flags().String("graph", "", "List edges for standalone graph")
	edgeListCmd.Flags().Int("page", 1, "Page number")
	edgeListCmd.Flags().Int("page-size", 50, "Results per page")

	// Delete flags
	edgeDeleteCmd.Flags().Bool("force", false, "Skip confirmation prompt")
}
