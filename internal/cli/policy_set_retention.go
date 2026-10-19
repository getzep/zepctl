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
}

var policySetRetentionTargetListCmd = &cobra.Command{
	Use:   "list <policy-set-uuid>",
	Short: "List the graph ID patterns of a retention policy set",
	Args:  cobra.ExactArgs(1),
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
	Args:  cobra.ExactArgs(1),
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
	Args:  cobra.ExactArgs(1),
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
