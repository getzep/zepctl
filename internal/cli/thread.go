package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/getzep/zep-go/v4"
	"github.com/getzep/zepctl/internal/client"
	"github.com/getzep/zepctl/internal/output"
	"github.com/spf13/cobra"
)

var threadCmd = &cobra.Command{
	Use:   "thread",
	Short: "Manage threads",
	Long:  `Create, get, delete threads and manage thread messages.`,
}

var threadListCmd = &cobra.Command{
	Use:   listCmdUse,
	Short: "List all threads",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		limit, _ := cmd.Flags().GetInt("limit")
		cursor, _ := cmd.Flags().GetString("cursor")
		orderBy, _ := cmd.Flags().GetString("order-by")
		asc, _ := cmd.Flags().GetBool("asc")

		req := &zep.ThreadListRequest{}
		if limit > 0 {
			req.PageSize = zep.Int(limit)
		}
		if cursor != "" {
			req.Cursor = zep.String(cursor)
		}
		if orderBy != "" {
			value := zep.ThreadListRequestOrderBy(orderBy)
			req.OrderBy = &value
		}
		if cmd.Flags().Changed("asc") {
			order := zep.ThreadListRequestOrder("desc")
			if asc {
				order = "asc"
			}
			req.Order = &order
		}

		threads, err := c.Thread.List(context.Background(), req)
		if err != nil {
			return fmt.Errorf("listing threads: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("UUID", "THREAD ID", "USER UUID", "CREATED AT")
			tbl.WriteHeader()
			for _, t := range threads.Results {
				threadUUID := ""
				if t.UUID != nil {
					threadUUID = *t.UUID
				}
				threadID := ""
				if t.ThreadID != nil {
					threadID = *t.ThreadID
				}
				userUUID := ""
				if t.UserUUID != nil {
					userUUID = *t.UserUUID
				}
				createdAt := ""
				if t.CreatedAt != nil {
					createdAt = *t.CreatedAt
				}
				tbl.WriteRow(threadUUID, threadID, userUUID, createdAt)
			}
			writeNextCursor(threads.RawResponse.Next)
			return tbl.Flush()
		}

		return output.Print(map[string]any{itemsOutputKey: threads.Results, nextCursorOutputKey: threads.RawResponse.Next})
	},
}

var threadCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new thread",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		userUUID, _ := cmd.Flags().GetString("user")
		if userUUID == "" {
			return fmt.Errorf("--user flag is required")
		}

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		req := &zep.CreateThreadRequest{
			UserUUID: userUUID,
		}

		thread, err := c.Thread.Create(context.Background(), req)
		if err != nil {
			return fmt.Errorf("creating thread: %w", err)
		}

		if thread.UUID != nil {
			output.Info("Created thread %s", *thread.UUID)
		}
		return output.Print(thread)
	},
}

var threadGetCmd = &cobra.Command{
	Use:   "get <thread-uuid>",
	Short: "Get thread messages",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		threadUUID := args[0]
		lastN, _ := cmd.Flags().GetInt("last")

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		req := &zep.ThreadListMessagesRequest{}
		if lastN > 0 {
			req.PageSize = zep.Int(lastN)
			req.Order = zep.ThreadListMessagesRequestOrderDesc.Ptr()
		}
		resp, err := c.Thread.ListMessages(context.Background(), threadUUID, req)
		if err != nil {
			return fmt.Errorf("getting thread: %w", err)
		}
		if lastN > 0 {
			reverseThreadMessages(resp.Results)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("ROLE", "NAME", "CONTENT", "CREATED AT")
			tbl.WriteHeader()
			for _, m := range resp.Results {
				role := ""
				if m.Role != nil {
					role = string(*m.Role)
				}
				name := ""
				if m.Name != nil {
					name = *m.Name
				}
				content := ""
				if m.Content != nil {
					content = *m.Content
				}
				if len(content) > 50 {
					content = content[:50] + "..."
				}
				createdAt := ""
				if m.CreatedAt != nil {
					createdAt = *m.CreatedAt
				}
				tbl.WriteRow(role, name, content, createdAt)
			}
			return tbl.Flush()
		}

		return output.Print(map[string]any{itemsOutputKey: resp.Results, nextCursorOutputKey: resp.RawResponse.Next})
	},
}

var threadDeleteCmd = &cobra.Command{
	Use:   "delete <thread-uuid>",
	Short: "Delete a thread",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		threadUUID := args[0]
		force, _ := cmd.Flags().GetBool("force")

		if !force {
			fmt.Printf("Delete thread %q? [y/N]: ", threadUUID)
			if !confirmYesNo() {
				output.Info("Aborted")
				return nil
			}
		}

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		result, err := c.Thread.Delete(context.Background(), threadUUID)
		if err != nil {
			return fmt.Errorf("deleting thread: %w", err)
		}

		wait, _ := cmd.Flags().GetBool("wait")
		timeout, _ := cmd.Flags().GetDuration("timeout")
		return reportDeletion(c, "thread", threadUUID, result.Task, wait, timeout)
	},
}

var threadMessagesCmd = &cobra.Command{
	Use:   "messages <thread-uuid>",
	Short: "List thread messages",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		threadUUID := args[0]
		lastN, _ := cmd.Flags().GetInt("last")
		limit, _ := cmd.Flags().GetInt("limit")
		cursor, _ := cmd.Flags().GetString("cursor")

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		req := &zep.ThreadListMessagesRequest{}
		if lastN > 0 {
			req.PageSize = zep.Int(lastN)
			req.Order = zep.ThreadListMessagesRequestOrderDesc.Ptr()
		} else if limit > 0 {
			req.PageSize = zep.Int(limit)
		}
		if cursor != "" {
			req.Cursor = zep.String(cursor)
		}

		messages, err := c.Thread.ListMessages(context.Background(), threadUUID, req)
		if err != nil {
			return fmt.Errorf("getting thread messages: %w", err)
		}
		if lastN > 0 {
			reverseThreadMessages(messages.Results)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("ROLE", "NAME", "CONTENT", "CREATED AT")
			tbl.WriteHeader()
			for _, m := range messages.Results {
				role := ""
				if m.Role != nil {
					role = string(*m.Role)
				}
				name := ""
				if m.Name != nil {
					name = *m.Name
				}
				content := ""
				if m.Content != nil {
					content = *m.Content
				}
				if len(content) > 50 {
					content = content[:50] + "..."
				}
				createdAt := ""
				if m.CreatedAt != nil {
					createdAt = *m.CreatedAt
				}
				tbl.WriteRow(role, name, content, createdAt)
			}
			writeNextCursor(messages.RawResponse.Next)
			return tbl.Flush()
		}

		return output.Print(map[string]any{itemsOutputKey: messages.Results, nextCursorOutputKey: messages.RawResponse.Next})
	},
}

func reverseThreadMessages(messages []*zep.Message) {
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
}

// MessageInput represents the input format for adding messages.
type MessageInput struct {
	Messages []MessageData `json:"messages"`
}

// MessageData represents a single message.
type MessageData struct {
	Role          string         `json:"role"`
	Name          string         `json:"name,omitempty"`
	Content       string         `json:"content"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	ReferenceTime string         `json:"reference_time,omitempty"`
}

var threadAddMessagesCmd = &cobra.Command{
	Use:   "add-messages <thread-uuid>",
	Short: "Add messages to a thread",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		threadUUID := args[0]

		file, _ := cmd.Flags().GetString("file")
		useStdin, _ := cmd.Flags().GetBool("stdin")
		batch, _ := cmd.Flags().GetBool("batch")
		wait, _ := cmd.Flags().GetBool("wait")

		var data []byte
		var err error

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
			return fmt.Errorf("either --file or --stdin is required")
		}

		var input MessageInput
		if err := json.Unmarshal(data, &input); err != nil {
			return fmt.Errorf("parsing messages: %w", err)
		}

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		var messages []*zep.AddMessage
		for _, m := range input.Messages {
			msg := &zep.AddMessage{
				Role:    zep.RoleType(m.Role).Ptr(),
				Content: zep.String(m.Content),
			}
			if m.ReferenceTime != "" {
				msg.ReferenceTime = zep.String(m.ReferenceTime)
			}
			if m.Name != "" {
				msg.Name = zep.String(m.Name)
			}
			if m.Metadata != nil {
				msg.Metadata = m.Metadata
			}
			messages = append(messages, msg)
		}

		if batch {
			items := make([]*zep.BatchItemInput, 0, len(messages))
			for _, message := range messages {
				role := zep.BatchItemInputRole(*message.Role)
				item := &zep.BatchItemInput{
					Type:       zep.BatchItemInputTypeThreadMessage,
					ThreadUUID: zep.String(threadUUID),
					Role:       &role,
					Content:    message.Content,
					Name:       message.Name,
					Metadata:   message.Metadata,
				}
				if message.ReferenceTime != nil {
					item.ReferenceTime = message.ReferenceTime
				}
				items = append(items, item)
			}
			createdBatch, err := c.Batch.Create(context.Background(), &zep.CreateBatchRequest{})
			if err != nil {
				return fmt.Errorf("creating message batch: %w", err)
			}
			if createdBatch.UUID == nil {
				return fmt.Errorf("creating message batch: response did not include a batch UUID")
			}
			if _, err := c.Batch.AddItems(context.Background(), *createdBatch.UUID, &zep.AddBatchItemsRequest{Items: items}); err != nil {
				return fmt.Errorf("adding message batch items: %w", err)
			}
			resp, err := c.Batch.Process(context.Background(), *createdBatch.UUID)
			if err != nil {
				return fmt.Errorf("processing message batch: %w", err)
			}

			if resp.Task != nil && resp.Task.UUID != nil {
				output.Info("Task started: %s", *resp.Task.UUID)
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

		resp, err := c.Thread.AddMessages(context.Background(), threadUUID, &zep.AddMessagesRequest{
			Messages: messages,
		})
		if err != nil {
			return fmt.Errorf("adding messages: %w", err)
		}

		if resp.Task != nil && resp.Task.UUID != nil {
			output.Info("Task started: %s", *resp.Task.UUID)
			if wait {
				if err := waitForTask(c, *resp.Task.UUID, defaultTaskTimeout, defaultTaskPollInterval); err != nil {
					return err
				}
			}
		}
		output.Info("Added %d messages to thread %q", len(messages), threadUUID)
		return output.Print(resp)
	},
}

var threadContextCmd = &cobra.Command{
	Use:   "context <thread-uuid>",
	Short: "Get thread context",
	Long:  `Returns relevant context from the user graph based on recent thread messages.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		threadUUID := args[0]

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		ctx, err := c.Thread.GetContext(context.Background(), threadUUID, &zep.ThreadGetContextRequest{})
		if err != nil {
			return fmt.Errorf("getting thread context: %w", err)
		}

		return output.Print(ctx)
	},
}

func init() {
	rootCmd.AddCommand(threadCmd)
	threadCmd.AddCommand(threadListCmd)
	threadCmd.AddCommand(threadCreateCmd)
	threadCmd.AddCommand(threadGetCmd)
	threadCmd.AddCommand(threadDeleteCmd)
	threadCmd.AddCommand(threadMessagesCmd)
	threadCmd.AddCommand(threadAddMessagesCmd)
	threadCmd.AddCommand(threadContextCmd)

	// List flags
	threadListCmd.Flags().Int("limit", 50, "Maximum number of results")
	threadListCmd.Flags().String("cursor", "", "Cursor from the previous page")
	threadListCmd.Flags().String("order-by", "", "Field to sort by")
	threadListCmd.Flags().Bool("asc", false, "Sort ascending")

	// Create flags
	threadCreateCmd.Flags().String("user", "", "User UUID (required)")
	_ = threadCreateCmd.MarkFlagRequired("user")

	// Get flags
	threadGetCmd.Flags().Int("last", 0, "Get last N messages")

	// Delete flags
	threadDeleteCmd.Flags().Bool("force", false, "Skip confirmation prompt")
	threadDeleteCmd.Flags().Bool("wait", false, "Wait for asynchronous deletion")
	threadDeleteCmd.Flags().Duration("timeout", defaultTaskTimeout, "Maximum time to wait")

	// Messages flags
	threadMessagesCmd.Flags().Int("last", 0, "Get last N messages")
	threadMessagesCmd.Flags().Int("limit", 50, "Maximum messages to return")
	threadMessagesCmd.Flags().String("cursor", "", "Cursor from the previous page")

	// Add messages flags
	threadAddMessagesCmd.Flags().String("file", "", "Path to JSON file containing messages")
	threadAddMessagesCmd.Flags().Bool("stdin", false, "Read messages from stdin")
	threadAddMessagesCmd.Flags().Bool("batch", false, "Use batch processing for large imports")
	threadAddMessagesCmd.Flags().Bool("wait", false, "Wait for batch processing to complete")
}
