package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	zep "github.com/getzep/zep-go/v4"
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
	Use:   "get <task-uuid>",
	Short: "Get task status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskUUID := args[0]

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		task, err := c.Task.Get(context.Background(), taskUUID)
		if err != nil {
			return fmt.Errorf("getting task: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("FIELD", "VALUE")
			tbl.WriteHeader()
			if task.UUID != nil {
				tbl.WriteRow("UUID", *task.UUID)
			}
			if task.Status != nil {
				tbl.WriteRow("Status", string(*task.Status))
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
			if task.Result != nil {
				tbl.WriteRow("Result", task.Result.String())
			}
			return tbl.Flush()
		}

		return output.Print(task)
	},
}

var taskWaitCmd = &cobra.Command{
	Use:   "wait <task-uuid>",
	Short: "Wait for task completion",
	Long:  `Polls the task status until it completes or fails.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskUUID := args[0]
		timeout, _ := cmd.Flags().GetDuration("timeout")
		pollInterval, _ := cmd.Flags().GetDuration("poll-interval")

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		output.Info("Waiting for task %s...", taskUUID)

		if err := waitForTask(c, taskUUID, timeout, pollInterval); err != nil {
			return err
		}

		output.Info("Task %s completed successfully", taskUUID)
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
func waitForTask(c *client.Client, taskUUID string, timeout, pollInterval time.Duration) error {
	return waitForTaskWith(
		taskUUID,
		timeout,
		pollInterval,
		func(ctx context.Context, uuid string) (*zep.Task, error) {
			return c.Task.Get(ctx, uuid)
		},
		time.Now,
		sleepContext,
	)
}

func reportDeletion(c *client.Client, resource, resourceUUID string, task *zep.Task, wait bool, timeout time.Duration) error {
	if task == nil || task.UUID == nil || *task.UUID == "" {
		output.Info("Deleted %s %s", resource, resourceUUID)
		return nil
	}
	if !wait {
		output.Info("Deletion of %s %s started; task %s", resource, resourceUUID, *task.UUID)
		return nil
	}
	output.Info("Deletion task: %s", *task.UUID)
	if err := waitForTask(c, *task.UUID, timeout, defaultTaskPollInterval); err != nil {
		return err
	}
	output.Info("Deleted %s %s", resource, resourceUUID)
	return nil
}

func waitForTaskWith(
	taskUUID string,
	timeout, pollInterval time.Duration,
	getTask func(context.Context, string) (*zep.Task, error),
	now func() time.Time,
	sleep func(context.Context, time.Duration) error,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	deadline := now().Add(timeout)

	for {
		if !now().Before(deadline) {
			return fmt.Errorf("timeout waiting for task %s", taskUUID)
		}

		task, err := getTask(ctx, taskUUID)
		if err != nil {
			return taskWaitOperationError(taskUUID, "getting", err, ctx)
		}
		if task != nil && task.Status != nil {
			done, err := taskPollResult(taskUUID, task)
			if done {
				return err
			}
		}

		remaining := deadline.Sub(now())
		if remaining <= 0 {
			return fmt.Errorf("timeout waiting for task %s", taskUUID)
		}
		sleepDuration := pollInterval
		if sleepDuration > remaining {
			sleepDuration = remaining
		}
		if err := sleep(ctx, sleepDuration); err != nil {
			return taskWaitOperationError(taskUUID, "waiting", err, ctx)
		}
	}
}

func taskWaitOperationError(taskUUID, operation string, err error, ctx context.Context) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("timeout waiting for task %s", taskUUID)
	}
	return fmt.Errorf("%s task %s: %w", operation, taskUUID, err)
}

func taskPollResult(taskUUID string, task *zep.Task) (bool, error) {
	switch *task.Status {
	case "succeeded":
		return true, nil
	case "partial", "failed":
		message := "unknown error"
		if task.Error != nil && task.Error.Message != nil {
			message = *task.Error.Message
		}
		return true, fmt.Errorf("task %s %s: %s", taskUUID, *task.Status, message)
	default:
		return false, nil
	}
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func waitForBatch(c *client.Client, batchUUID string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		if err := sleepContext(ctx, defaultTaskPollInterval); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return fmt.Errorf("timeout waiting for batch %s", batchUUID)
			}
			return fmt.Errorf("waiting for batch %s: %w", batchUUID, err)
		}
		batch, err := c.Batch.Get(ctx, batchUUID)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("timeout waiting for batch %s", batchUUID)
			}
			return fmt.Errorf("getting batch %s: %w", batchUUID, err)
		}
		if batch == nil || batch.Status == nil {
			continue
		}
		switch *batch.Status {
		case "succeeded":
			return nil
		case "partial", "failed":
			return fmt.Errorf("batch %s %s", batchUUID, *batch.Status)
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
