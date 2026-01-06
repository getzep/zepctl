package cli

import (
	"context"
	"fmt"

	"github.com/getzep/zepctl/internal/client"
	"github.com/getzep/zepctl/internal/output"
	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage project",
	Long:  `Get project information.`,
}

var projectGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get project information",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}

		project, err := c.Project.Get(context.Background())
		if err != nil {
			return fmt.Errorf("getting project: %w", err)
		}

		return output.Print(project)
	},
}

func init() {
	rootCmd.AddCommand(projectCmd)
	projectCmd.AddCommand(projectGetCmd)
}
