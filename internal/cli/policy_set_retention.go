package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/getzep/zepctl/internal/abac"
	"github.com/getzep/zepctl/internal/output"
)

var policySetRetentionTargetCmd = &cobra.Command{
	Use:     "retention-target",
	Aliases: []string{"retention-targets"},
	Short:   "Manage the graph ID patterns of a retention policy set",
	Long: `Manage the graph ID patterns of a retention policy set. A retention policy set has no effect until it is attached to at least one pattern. A pattern is an exact graph ID, a prefix that ends in "*", or "*" for all graphs in the project.

Examples:
  zepctl policy-set retention-target list <policy-set-uuid>
  zepctl policy-set retention-target attach <policy-set-uuid> --pattern "scratch-*"
  zepctl policy-set retention-target detach <policy-set-uuid> --pattern "scratch-*"
`,
}

var policySetRetentionTargetListCmd = &cobra.Command{
	Use:   "list <policy-set-uuid>",
	Short: "List the graph ID patterns of a retention policy set",
	Long: `List the graph ID patterns that a retention policy set is attached to in the current project.

Example:
  zepctl policy-set retention-target list <policy-set-uuid>
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateUUID(args[0], "policy set"); err != nil {
			return err
		}
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		result, err := ac.ListRetentionTargets(cmd.Context(), args[0])
		if err != nil {
			if abac.IsNotFound(err) {
				return fmt.Errorf("policy set not found: %s", args[0])
			}
			return fmt.Errorf("listing retention targets: %w", err)
		}
		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("PATTERN")
			tbl.WriteHeader()
			for _, target := range result.Targets {
				tbl.WriteRow(target.Pattern)
			}
			return tbl.Flush()
		}
		return output.Print(result)
	},
}

var policySetRetentionTargetAttachCmd = &cobra.Command{
	Use:   "attach <policy-set-uuid>",
	Short: "Attach a graph ID pattern to a retention policy set",
	Long: `Attach a retention policy set to a graph ID pattern in the current project. The server accepts targets only for retention policy sets. If the pattern is already attached, the command does not fail.

Examples:
  zepctl policy-set retention-target attach <policy-set-uuid> --pattern customer-42
  zepctl policy-set retention-target attach <policy-set-uuid> --pattern "scratch-*"
  zepctl policy-set retention-target attach <policy-set-uuid> --pattern "*"
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateUUID(args[0], "policy set"); err != nil {
			return err
		}
		pattern, _ := cmd.Flags().GetString("pattern")
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		result, err := ac.AttachRetentionTarget(cmd.Context(), args[0], pattern)
		if err != nil {
			if abac.IsNotFound(err) {
				return fmt.Errorf("policy set not found: %s", args[0])
			}
			return fmt.Errorf("attaching retention target: %w", err)
		}
		if output.GetFormat() == output.FormatTable {
			output.Info("Attached policy set %s to pattern %q", args[0], pattern)
			return nil
		}
		return output.Print(result)
	},
}

var policySetRetentionTargetDetachCmd = &cobra.Command{
	Use:   "detach <policy-set-uuid>",
	Short: "Detach a graph ID pattern from a retention policy set",
	Long: `Detach a retention policy set from a graph ID pattern in the current project. If the pattern is not attached, the command does not fail.

Example:
  zepctl policy-set retention-target detach <policy-set-uuid> --pattern "scratch-*"
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateUUID(args[0], "policy set"); err != nil {
			return err
		}
		pattern, _ := cmd.Flags().GetString("pattern")
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		result, err := ac.DetachRetentionTarget(cmd.Context(), args[0], pattern)
		if err != nil {
			if abac.IsNotFound(err) {
				return fmt.Errorf("policy set not found: %s", args[0])
			}
			return fmt.Errorf("detaching retention target: %w", err)
		}
		if output.GetFormat() == output.FormatTable {
			output.Info("Detached policy set %s from pattern %q", args[0], pattern)
			return nil
		}
		return output.Print(result)
	},
}

func init() {
	policySetRetentionTargetCmd.AddCommand(
		policySetRetentionTargetListCmd,
		policySetRetentionTargetAttachCmd,
		policySetRetentionTargetDetachCmd,
	)
	policySetRetentionTargetAttachCmd.Flags().String("pattern", "", "Graph ID pattern: an exact graph ID, a prefix that ends in \"*\", or \"*\" for all graphs in the project")
	policySetRetentionTargetDetachCmd.Flags().String("pattern", "", "Graph ID pattern: an exact graph ID, a prefix that ends in \"*\", or \"*\" for all graphs in the project")
	_ = policySetRetentionTargetAttachCmd.MarkFlagRequired("pattern")
	_ = policySetRetentionTargetDetachCmd.MarkFlagRequired("pattern")
}
