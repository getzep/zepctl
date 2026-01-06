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

var threadCmd = &cobra.Command{
	Use:   "thread",
	Short: "Manage threads",
	Long:  `Create, get, delete threads and manage thread messages.`,
}

var threadCreateCmd = &cobra.Command{
	Use:   "create <thread-id>",
	Short: "Create a new thread",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		threadID := args[0]

		userID, _ := cmd.Flags().GetString("user")
		if userID == "" {
			return fmt.Errorf("--user flag is required")
		}

		c, err := client.New()
		if err != nil {
			return err
		}

		req := &zep.CreateThreadRequest{
			ThreadID: threadID,
			UserID:   userID,
		}

		thread, err := c.Thread.Create(context.Background(), req)
		if err != nil {
			return fmt.Errorf("creating thread: %w", err)
		}

		output.Info("Created thread %q for user %q", threadID, userID)
		return output.Print(thread)
	},
}

var threadGetCmd = &cobra.Command{
	Use:   "get <thread-id>",
	Short: "Get thread messages",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		threadID := args[0]
		lastN, _ := cmd.Flags().GetInt("last")

		c, err := client.New()
		if err != nil {
			return err
		}

		req := &zep.ThreadGetRequest{}
		if lastN > 0 {
			req.Lastn = zep.Int(lastN)
		}

		resp, err := c.Thread.Get(context.Background(), threadID, req)
		if err != nil {
			return fmt.Errorf("getting thread: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("ROLE", "NAME", "CONTENT", "CREATED AT")
			tbl.WriteHeader()
			for _, m := range resp.Messages {
				name := ""
				if m.Name != nil {
					name = *m.Name
				}
				content := m.Content
				if len(content) > 50 {
					content = content[:50] + "..."
				}
				createdAt := ""
				if m.CreatedAt != nil {
					createdAt = *m.CreatedAt
				}
				tbl.WriteRow(string(m.Role), name, content, createdAt)
			}
			return tbl.Flush()
		}

		return output.Print(resp)
	},
}

var threadDeleteCmd = &cobra.Command{
	Use:   "delete <thread-id>",
	Short: "Delete a thread",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		threadID := args[0]
		force, _ := cmd.Flags().GetBool("force")

		if !force {
			fmt.Printf("Delete thread %q? [y/N]: ", threadID)
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

		if _, err := c.Thread.Delete(context.Background(), threadID); err != nil {
			return fmt.Errorf("deleting thread: %w", err)
		}

		output.Info("Deleted thread %q", threadID)
		return nil
	},
}

var threadMessagesCmd = &cobra.Command{
	Use:   "messages <thread-id>",
	Short: "List thread messages",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		threadID := args[0]
		lastN, _ := cmd.Flags().GetInt("last")
		limit, _ := cmd.Flags().GetInt("limit")

		c, err := client.New()
		if err != nil {
			return err
		}

		req := &zep.ThreadGetRequest{}
		if lastN > 0 {
			req.Lastn = zep.Int(lastN)
		} else if limit > 0 {
			req.Limit = zep.Int(limit)
		}

		messages, err := c.Thread.Get(context.Background(), threadID, req)
		if err != nil {
			return fmt.Errorf("getting thread messages: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("ROLE", "NAME", "CONTENT", "CREATED AT")
			tbl.WriteHeader()
			for _, m := range messages.Messages {
				name := ""
				if m.Name != nil {
					name = *m.Name
				}
				content := m.Content
				if len(content) > 50 {
					content = content[:50] + "..."
				}
				createdAt := ""
				if m.CreatedAt != nil {
					createdAt = *m.CreatedAt
				}
				tbl.WriteRow(string(m.Role), name, content, createdAt)
			}
			return tbl.Flush()
		}

		return output.Print(messages)
	},
}

// MessageInput represents the input format for adding messages.
type MessageInput struct {
	Messages []MessageData `json:"messages"`
}

// MessageData represents a single message.
type MessageData struct {
	Role     string         `json:"role"`
	Name     string         `json:"name,omitempty"`
	Content  string         `json:"content"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

var threadAddMessagesCmd = &cobra.Command{
	Use:   "add-messages <thread-id>",
	Short: "Add messages to a thread",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		threadID := args[0]

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
			data, err = io.ReadAll(os.Stdin)
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

		c, err := client.New()
		if err != nil {
			return err
		}

		var messages []*zep.Message
		for _, m := range input.Messages {
			msg := &zep.Message{
				Role:    zep.RoleType(m.Role),
				Content: m.Content,
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
			resp, err := c.Thread.AddMessagesBatch(context.Background(), threadID, &zep.AddThreadMessagesRequest{
				Messages: messages,
			})
			if err != nil {
				return fmt.Errorf("adding messages batch: %w", err)
			}

			if wait && resp.TaskID != nil {
				output.Info("Batch task started: %s", *resp.TaskID)
				if err := waitForTask(c, *resp.TaskID, defaultTaskTimeout, defaultTaskPollInterval); err != nil {
					return err
				}
				output.Info("Batch processing completed")
			} else if resp.TaskID != nil {
				output.Info("Batch task started: %s", *resp.TaskID)
			}

			return output.Print(resp)
		}

		resp, err := c.Thread.AddMessages(context.Background(), threadID, &zep.AddThreadMessagesRequest{
			Messages: messages,
		})
		if err != nil {
			return fmt.Errorf("adding messages: %w", err)
		}

		output.Info("Added %d messages to thread %q", len(messages), threadID)
		return output.Print(resp)
	},
}

var threadContextCmd = &cobra.Command{
	Use:   "context <thread-id>",
	Short: "Get thread context",
	Long:  `Returns relevant context from the user graph based on recent thread messages.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		threadID := args[0]

		c, err := client.New()
		if err != nil {
			return err
		}

		ctx, err := c.Thread.GetUserContext(context.Background(), threadID, &zep.ThreadGetUserContextRequest{})
		if err != nil {
			return fmt.Errorf("getting thread context: %w", err)
		}

		return output.Print(ctx)
	},
}

func init() {
	rootCmd.AddCommand(threadCmd)
	threadCmd.AddCommand(threadCreateCmd)
	threadCmd.AddCommand(threadGetCmd)
	threadCmd.AddCommand(threadDeleteCmd)
	threadCmd.AddCommand(threadMessagesCmd)
	threadCmd.AddCommand(threadAddMessagesCmd)
	threadCmd.AddCommand(threadContextCmd)

	// Create flags
	threadCreateCmd.Flags().String("user", "", "User ID (required)")
	_ = threadCreateCmd.MarkFlagRequired("user")

	// Get flags
	threadGetCmd.Flags().Int("last", 0, "Get last N messages")

	// Delete flags
	threadDeleteCmd.Flags().Bool("force", false, "Skip confirmation prompt")

	// Messages flags
	threadMessagesCmd.Flags().Int("last", 0, "Get last N messages")
	threadMessagesCmd.Flags().Int("limit", 50, "Maximum messages to return")

	// Add messages flags
	threadAddMessagesCmd.Flags().String("file", "", "Path to JSON file containing messages")
	threadAddMessagesCmd.Flags().Bool("stdin", false, "Read messages from stdin")
	threadAddMessagesCmd.Flags().Bool("batch", false, "Use batch processing for large imports")
	threadAddMessagesCmd.Flags().Bool("wait", false, "Wait for batch processing to complete")
}
