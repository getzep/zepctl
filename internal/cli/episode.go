package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/getzep/zep-go/v3"
	"github.com/getzep/zep-go/v3/graph"
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
	Use:   "list",
	Short: "List episodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		userID, _ := cmd.Flags().GetString("user")
		graphID, _ := cmd.Flags().GetString("graph")
		lastN, _ := cmd.Flags().GetInt("last")

		if userID == "" && graphID == "" {
			return fmt.Errorf("either --user or --graph is required")
		}

		c, err := client.New()
		if err != nil {
			return err
		}

		var episodeResp *zep.EpisodeResponse

		if userID != "" {
			req := &graph.EpisodeGetByUserIDRequest{}
			if lastN > 0 {
				req.Lastn = zep.Int(lastN)
			}
			result, err := c.Graph.Episode.GetByUserID(context.Background(), userID, req)
			if err != nil {
				return fmt.Errorf("listing episodes: %w", err)
			}
			episodeResp = result
		} else {
			req := &graph.EpisodeGetByGraphIDRequest{}
			if lastN > 0 {
				req.Lastn = zep.Int(lastN)
			}
			result, err := c.Graph.Episode.GetByGraphID(context.Background(), graphID, req)
			if err != nil {
				return fmt.Errorf("listing episodes: %w", err)
			}
			episodeResp = result
		}

		episodes := episodeResp.Episodes

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
					role = *ep.Role
				}
				content := ep.Content
				if len(content) > 40 {
					content = content[:40] + "..."
				}
				tbl.WriteRow(ep.UUID, source, role, content, ep.CreatedAt)
			}
			return tbl.Flush()
		}

		return output.Print(episodes)
	},
}

var episodeGetCmd = &cobra.Command{
	Use:   "get <uuid>",
	Short: "Get episode details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uuid := args[0]

		c, err := client.New()
		if err != nil {
			return err
		}

		episode, err := c.Graph.Episode.Get(context.Background(), uuid)
		if err != nil {
			return fmt.Errorf("getting episode: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("FIELD", "VALUE")
			tbl.WriteHeader()
			tbl.WriteRow("UUID", episode.UUID)
			if episode.Source != nil {
				tbl.WriteRow("Source", string(*episode.Source))
			}
			if episode.SourceDescription != nil {
				tbl.WriteRow("Source Description", *episode.SourceDescription)
			}
			if episode.Role != nil {
				tbl.WriteRow("Role", *episode.Role)
			}
			if episode.RoleType != nil {
				tbl.WriteRow("Role Type", string(*episode.RoleType))
			}
			tbl.WriteRow("Content", episode.Content)
			tbl.WriteRow("Created At", episode.CreatedAt)
			return tbl.Flush()
		}

		return output.Print(episode)
	},
}

var episodeMentionsCmd = &cobra.Command{
	Use:   "mentions <uuid>",
	Short: "Get nodes and edges mentioned in an episode",
	Long:  `Returns nodes and edges mentioned in the specified episode.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uuid := args[0]

		c, err := client.New()
		if err != nil {
			return err
		}

		mentions, err := c.Graph.Episode.GetNodesAndEdges(context.Background(), uuid)
		if err != nil {
			return fmt.Errorf("getting episode mentions: %w", err)
		}

		return output.Print(mentions)
	},
}

var episodeDeleteCmd = &cobra.Command{
	Use:   "delete <uuid>",
	Short: "Delete an episode",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uuid := args[0]
		force, _ := cmd.Flags().GetBool("force")

		if !force {
			fmt.Printf("Delete episode %q? [y/N]: ", uuid)
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

		if _, err := c.Graph.Episode.Delete(context.Background(), uuid); err != nil {
			return fmt.Errorf("deleting episode: %w", err)
		}

		output.Info("Deleted episode %q", uuid)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(episodeCmd)
	episodeCmd.AddCommand(episodeListCmd)
	episodeCmd.AddCommand(episodeGetCmd)
	episodeCmd.AddCommand(episodeMentionsCmd)
	episodeCmd.AddCommand(episodeDeleteCmd)

	// List flags
	episodeListCmd.Flags().String("user", "", "List episodes for user graph")
	episodeListCmd.Flags().String("graph", "", "List episodes for standalone graph")
	episodeListCmd.Flags().Int("page", 1, "Page number")
	episodeListCmd.Flags().Int("page-size", 50, "Results per page")
	episodeListCmd.Flags().Int("last", 0, "Get last N episodes (shortcut, ignores pagination)")

	// Delete flags
	episodeDeleteCmd.Flags().Bool("force", false, "Skip confirmation prompt")
}
