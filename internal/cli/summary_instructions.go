package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/getzep/zep-go/v4"
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
	Use:   listCmdUse,
	Short: "List summary instructions",
	RunE: func(cmd *cobra.Command, args []string) error {
		userUUID, _ := cmd.Flags().GetString("user")

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		result, err := getSummaryInstructions(c, userUUID)
		if err != nil {
			return fmt.Errorf("listing summary instructions: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("NAME", "TEXT")
			tbl.WriteHeader()
			for _, inst := range result.Instructions {
				text := truncateText(inst.Text, 60)
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
		userUUID, _ := cmd.Flags().GetString("user")

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

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		existing, err := getSummaryInstructions(c, userUUID)
		if err != nil {
			return fmt.Errorf("getting summary instructions: %w", err)
		}
		instructions := existing.Instructions
		replaced := false
		for i, current := range instructions {
			if current.Name == name {
				instructions[i] = &zep.UserInstruction{Name: name, Text: instructionText}
				replaced = true
				break
			}
		}
		if !replaced {
			instructions = append(instructions, &zep.UserInstruction{Name: name, Text: instructionText})
		}
		result, err := setSummaryInstructions(c, userUUID, instructions)
		if err != nil {
			return fmt.Errorf("adding summary instruction: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			scope := "project-wide"
			if userUUID != "" {
				scope = fmt.Sprintf("user: %s", userUUID)
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
		userUUID, _ := cmd.Flags().GetString("user")

		if !force {
			fmt.Printf("Delete summary instruction %q? [y/N]: ", name)
			if !confirmYesNo() {
				output.Info("Aborted")
				return nil
			}
		}

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		existing, err := getSummaryInstructions(c, userUUID)
		if err != nil {
			return fmt.Errorf("getting summary instructions: %w", err)
		}
		instructions := make([]*zep.UserInstruction, 0, len(existing.Instructions))
		found := false
		for _, current := range existing.Instructions {
			if current.Name == name {
				found = true
				continue
			}
			instructions = append(instructions, current)
		}
		if !found {
			return fmt.Errorf("summary instruction %q was not found", name)
		}
		if _, err := setSummaryInstructions(c, userUUID, instructions); err != nil {
			return fmt.Errorf("deleting summary instruction: %w", err)
		}

		scope := "project-wide"
		if userUUID != "" {
			scope = fmt.Sprintf("user: %s", userUUID)
		}
		output.Info("Deleted summary instruction %q (%s)", name, scope)
		return nil
	},
}

func getSummaryInstructions(c *client.Client, userUUID string) (*zep.UserSummaryInstructions, error) {
	if userUUID != "" {
		return c.User.GetSummaryInstructions(context.Background(), userUUID)
	}
	return c.Project.GetUserSummaryInstructions(context.Background())
}

func setSummaryInstructions(c *client.Client, userUUID string, instructions []*zep.UserInstruction) (*zep.UserSummaryInstructions, error) {
	request := &zep.UserSummaryInstructions{Instructions: instructions}
	if userUUID != "" {
		return c.User.SetSummaryInstructions(context.Background(), userUUID, request)
	}
	return c.Project.SetUserSummaryInstructions(context.Background(), request)
}

func init() {
	rootCmd.AddCommand(summaryInstructionsCmd)
	summaryInstructionsCmd.AddCommand(summaryInstructionsListCmd)
	summaryInstructionsCmd.AddCommand(summaryInstructionsAddCmd)
	summaryInstructionsCmd.AddCommand(summaryInstructionsDeleteCmd)

	// List flags
	summaryInstructionsListCmd.Flags().String("user", "", "User UUID")

	// Add flags
	summaryInstructionsAddCmd.Flags().String("name", "", "Instruction name (unique identifier)")
	summaryInstructionsAddCmd.Flags().String("instruction", "", "Instruction text (max 100 chars)")
	summaryInstructionsAddCmd.Flags().String("file", "", "Path to file containing instruction text")
	summaryInstructionsAddCmd.Flags().String("user", "", "User UUID")

	// Delete flags
	summaryInstructionsDeleteCmd.Flags().Bool("force", false, "Skip confirmation prompt")
	summaryInstructionsDeleteCmd.Flags().String("user", "", "User UUID")
}
