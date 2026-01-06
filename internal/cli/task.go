package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/getzep/zepctl/internal/client"
	"github.com/getzep/zepctl/internal/output"
	"github.com/spf13/cobra"
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage async tasks",
	Long:  `Get status and wait for async operations (batch imports, cloning, etc.)`,
}

var taskGetCmd = &cobra.Command{
	Use:   "get <task-id>",
	Short: "Get task status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]

		c, err := client.New()
		if err != nil {
			return err
		}

		task, err := c.Task.Get(context.Background(), taskID)
		if err != nil {
			return fmt.Errorf("getting task: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("FIELD", "VALUE")
			tbl.WriteHeader()
			if task.TaskID != nil {
				tbl.WriteRow("Task ID", *task.TaskID)
			}
			if task.Status != nil {
				tbl.WriteRow("Status", *task.Status)
			}
			if task.Type != nil {
				tbl.WriteRow("Type", *task.Type)
			}
			if task.CreatedAt != nil {
				tbl.WriteRow("Created At", *task.CreatedAt)
			}
			if task.StartedAt != nil {
				tbl.WriteRow("Started At", *task.StartedAt)
			}
			if task.CompletedAt != nil {
				tbl.WriteRow("Completed At", *task.CompletedAt)
			}
			if task.Error != nil && task.Error.Message != nil {
				tbl.WriteRow("Error", *task.Error.Message)
			}
			return tbl.Flush()
		}

		return output.Print(task)
	},
}

var taskWaitCmd = &cobra.Command{
	Use:   "wait <task-id>",
	Short: "Wait for task completion",
	Long:  `Polls the task status until it completes or fails.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]
		timeout, _ := cmd.Flags().GetDuration("timeout")
		pollInterval, _ := cmd.Flags().GetDuration("poll-interval")

		c, err := client.New()
		if err != nil {
			return err
		}

		output.Info("Waiting for task %s...", taskID)

		if err := waitForTask(c, taskID, timeout, pollInterval); err != nil {
			return err
		}

		output.Info("Task %s completed successfully", taskID)
		return nil
	},
}

// Default task polling settings.
const (
	defaultTaskTimeout      = 5 * time.Minute
	defaultTaskPollInterval = 1 * time.Second
)

// waitForTask polls the task status until completion or failure.
// This is a shared helper used by commands that need to wait for async operations.
func waitForTask(c *client.Client, taskID string, timeout, pollInterval time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for task %s", taskID)
		case <-ticker.C:
			task, err := c.Task.Get(ctx, taskID)
			if err != nil {
				return fmt.Errorf("getting task: %w", err)
			}

			status := ""
			if task.Status != nil {
				status = *task.Status
			}

			switch status {
			case "completed":
				return nil
			case "failed":
				errMsg := "unknown error"
				if task.Error != nil && task.Error.Message != nil {
					errMsg = *task.Error.Message
				}
				return fmt.Errorf("task %s failed: %s", taskID, errMsg)
			}
		}
	}
}

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskGetCmd)
	taskCmd.AddCommand(taskWaitCmd)

	// Wait flags
	taskWaitCmd.Flags().Duration("timeout", defaultTaskTimeout, "Maximum wait time")
	taskWaitCmd.Flags().Duration("poll-interval", defaultTaskPollInterval, "Polling interval")
}
