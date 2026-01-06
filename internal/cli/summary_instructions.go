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

// maxInstructionLength is the maximum length for summary instruction text.
const maxInstructionLength = 100

var summaryInstructionsCmd = &cobra.Command{
	Use:     "summary-instructions",
	Aliases: []string{"si"},
	Short:   "Manage user summary instructions",
	Long:    `List, add, and delete instructions that customize how Zep generates user summaries.`,
}

var summaryInstructionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List summary instructions",
	RunE: func(cmd *cobra.Command, args []string) error {
		userID, _ := cmd.Flags().GetString("user")

		c, err := client.New()
		if err != nil {
			return err
		}

		req := &zep.UserListUserSummaryInstructionsRequest{}
		if userID != "" {
			req.UserID = zep.String(userID)
		}

		result, err := c.User.ListUserSummaryInstructions(context.Background(), req)
		if err != nil {
			return fmt.Errorf("listing summary instructions: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("NAME", "TEXT")
			tbl.WriteHeader()
			for _, inst := range result.Instructions {
				text := inst.Text
				if len(text) > 60 {
					text = text[:60] + "..."
				}
				tbl.WriteRow(inst.Name, text)
			}
			return tbl.Flush()
		}

		return output.Print(result)
	},
}

var summaryInstructionsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add summary instructions",
	Long:  `Add instructions that customize how Zep generates the user summary.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		instruction, _ := cmd.Flags().GetString("instruction")
		file, _ := cmd.Flags().GetString("file")
		userIDs, _ := cmd.Flags().GetString("user")

		if name == "" {
			return fmt.Errorf("--name is required")
		}

		if instruction == "" && file == "" {
			return fmt.Errorf("either --instruction or --file is required")
		}

		instructionText := instruction
		if file != "" {
			data, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}
			instructionText = strings.TrimSpace(string(data))
		}

		if len(instructionText) > maxInstructionLength {
			return fmt.Errorf("instruction text exceeds maximum length of %d characters (got %d)", maxInstructionLength, len(instructionText))
		}

		c, err := client.New()
		if err != nil {
			return err
		}

		req := &zep.AddUserInstructionsRequest{
			Instructions: []*zep.UserInstruction{
				{
					Name: name,
					Text: instructionText,
				},
			},
		}

		if userIDs != "" {
			req.UserIDs = strings.Split(userIDs, ",")
		}

		result, err := c.User.AddUserSummaryInstructions(context.Background(), req)
		if err != nil {
			return fmt.Errorf("adding summary instruction: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			scope := "project-wide"
			if userIDs != "" {
				scope = fmt.Sprintf("user(s): %s", userIDs)
			}
			output.Info("Added summary instruction %q (%s)", name, scope)
			return nil
		}

		return output.Print(result)
	},
}

var summaryInstructionsDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a summary instruction",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		force, _ := cmd.Flags().GetBool("force")
		userIDs, _ := cmd.Flags().GetString("user")

		if !force {
			fmt.Printf("Delete summary instruction %q? [y/N]: ", name)
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

		req := &zep.DeleteUserInstructionsRequest{
			InstructionNames: []string{name},
		}

		if userIDs != "" {
			req.UserIDs = strings.Split(userIDs, ",")
		}

		if _, err := c.User.DeleteUserSummaryInstructions(context.Background(), req); err != nil {
			return fmt.Errorf("deleting summary instruction: %w", err)
		}

		scope := "project-wide"
		if userIDs != "" {
			scope = fmt.Sprintf("user(s): %s", userIDs)
		}
		output.Info("Deleted summary instruction %q (%s)", name, scope)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(summaryInstructionsCmd)
	summaryInstructionsCmd.AddCommand(summaryInstructionsListCmd)
	summaryInstructionsCmd.AddCommand(summaryInstructionsAddCmd)
	summaryInstructionsCmd.AddCommand(summaryInstructionsDeleteCmd)

	// List flags
	summaryInstructionsListCmd.Flags().String("user", "", "Filter by user ID")

	// Add flags
	summaryInstructionsAddCmd.Flags().String("name", "", "Instruction name (unique identifier)")
	summaryInstructionsAddCmd.Flags().String("instruction", "", "Instruction text (max 100 chars)")
	summaryInstructionsAddCmd.Flags().String("file", "", "Path to file containing instruction text")
	summaryInstructionsAddCmd.Flags().String("user", "", "Apply to specific user(s) (comma-separated)")

	// Delete flags
	summaryInstructionsDeleteCmd.Flags().Bool("force", false, "Skip confirmation prompt")
	summaryInstructionsDeleteCmd.Flags().String("user", "", "Delete from specific user(s) (comma-separated)")
}
