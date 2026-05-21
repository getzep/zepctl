package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/getzep/zepctl/internal/abac"
	"github.com/getzep/zepctl/internal/client"
	"github.com/getzep/zepctl/internal/output"
)

const abacModeEnforce = "enforce"

var (
	validModesList = []string{"off", "report_only", abacModeEnforce}
	validModes     = func() map[string]bool {
		m := make(map[string]bool, len(validModesList))
		for _, v := range validModesList {
			m[v] = true
		}
		return m
	}()
)

var apiKeyCmd = &cobra.Command{
	Use:   "api-key",
	Short: "Manage API key ABAC configuration",
	Long:  "Configure ABAC settings and policy set attachments for API keys.",
}

var apiKeyListCmd = &cobra.Command{
	Use:   listCmdUse,
	Short: "List API keys in the current project",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		result, err := ac.ListAPIKeys(cmd.Context())
		if err != nil {
			return fmt.Errorf("listing API keys: %w", err)
		}
		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("UUID", "NAME", "KEY", "ROLE")
			tbl.WriteHeader()
			for _, k := range result.Keys {
				key := k.FirstFour + "..." + k.LastFour
				tbl.WriteRow(k.UUID, k.Name, key, k.Role)
			}
			return tbl.Flush()
		}
		return output.Print(result)
	},
}

// --- Settings subgroup ---

var apiKeySettingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Manage ABAC settings for an API key",
}

var apiKeySettingsGetCmd = &cobra.Command{
	Use:   "get <key-uuid>",
	Short: "Get ABAC settings for an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateUUID(args[0], "API key"); err != nil {
			return err
		}
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		settings, err := ac.GetAPIKeySettings(cmd.Context(), args[0])
		if err != nil {
			if abac.IsNotFound(err) {
				return fmt.Errorf("API key not found: %s", args[0])
			}
			return fmt.Errorf("getting API key settings: %w", err)
		}
		if output.GetFormat() == output.FormatTable {
			fmt.Fprintf(cmd.OutOrStdout(), "ABAC Mode:     %s\n", settings.ABACMode)
			fmt.Fprintf(cmd.OutOrStdout(), "Capabilities:  %s\n", settings.Capabilities)
			return nil
		}
		return output.Print(settings)
	},
}

var apiKeySettingsSetCmd = &cobra.Command{
	Use:   "set <key-uuid>",
	Short: "Update ABAC settings for an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateUUID(args[0], "API key"); err != nil {
			return err
		}
		mode, _ := cmd.Flags().GetString("mode")
		if mode == "" {
			return fmt.Errorf("at least one setting flag is required (e.g., --mode)")
		}
		if !validModes[mode] {
			return fmt.Errorf("invalid mode %q: must be one of: %s", mode, strings.Join(validModesList, ", "))
		}
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		settings, err := ac.SetAPIKeySettings(cmd.Context(), args[0], mode)
		if err != nil {
			if abac.IsNotFound(err) {
				return fmt.Errorf("API key not found: %s", args[0])
			}
			return fmt.Errorf("updating API key settings: %w", err)
		}
		if output.GetFormat() == output.FormatTable {
			output.Info("Updated API key settings:\n  ABAC Mode:     %s\n  Capabilities:  %s",
				settings.ABACMode, settings.Capabilities)
			return nil
		}
		return output.Print(settings)
	},
}

// --- Policy Sets subgroup ---

var apiKeyPolicySetsCmd = &cobra.Command{
	Use:   "policy-sets",
	Short: "Manage policy set attachments for an API key",
}

var apiKeyPolicySetsListCmd = &cobra.Command{
	Use:   "list <key-uuid>",
	Short: "List policy sets attached to an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateUUID(args[0], "API key"); err != nil {
			return err
		}
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		result, err := ac.ListAPIKeyPolicySets(cmd.Context(), args[0])
		if err != nil {
			if abac.IsNotFound(err) {
				return fmt.Errorf("API key not found: %s", args[0])
			}
			return fmt.Errorf("listing attached policy sets: %w", err)
		}
		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("UUID", "NAME", "MODE", "VERSION")
			tbl.WriteHeader()
			for _, ps := range result.PolicySets {
				tbl.WriteRow(ps.UUID, ps.Name, ps.Mode, fmt.Sprintf("%d", ps.Version))
			}
			return tbl.Flush()
		}
		return output.Print(result)
	},
}

var apiKeyPolicySetsAttachCmd = &cobra.Command{
	Use:   "attach <key-uuid> <policy-set-uuid>",
	Short: "Attach a policy set to an API key",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateUUID(args[0], "API key"); err != nil {
			return err
		}
		if err := validateUUID(args[1], "policy set"); err != nil {
			return err
		}
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		result, err := ac.AttachPolicySet(cmd.Context(), args[0], args[1])
		if err != nil {
			return fmt.Errorf("attaching policy set: %w", err)
		}
		if output.GetFormat() == output.FormatTable {
			for _, ps := range result.PolicySets {
				if ps.UUID == args[1] {
					output.Info("Attached policy set %q to API key %s", ps.Name, args[0])
					return nil
				}
			}
			output.Info("Attached policy set to API key %s", args[0])
			return nil
		}
		return output.Print(result)
	},
}

var apiKeyPolicySetsDetachCmd = &cobra.Command{
	Use:   "detach <key-uuid> <policy-set-uuid>",
	Short: "Detach a policy set from an API key",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateUUID(args[0], "API key"); err != nil {
			return err
		}
		if err := validateUUID(args[1], "policy set"); err != nil {
			return err
		}
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		result, err := ac.DetachPolicySet(cmd.Context(), args[0], args[1])
		if err != nil {
			return fmt.Errorf("detaching policy set: %w", err)
		}
		if output.GetFormat() == output.FormatTable {
			output.Info("Detached policy set from API key %s", args[0])
			return nil
		}
		return output.Print(result)
	},
}

// --- Dry-Run subcommands (Sections 4.7, 4.8) ---

var apiKeyEvaluateCmd = &cobra.Command{
	Use:   "evaluate <key-uuid>",
	Short: "Dry-run a policy decision against an API key's live configuration",
	Long: "Asks the server what the evaluator would decide for the given action " +
		"and API key, without performing the request. Always exits 0 on a successful " +
		"API call regardless of allow/deny outcome.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateUUID(args[0], "API key"); err != nil {
			return err
		}
		action, _ := cmd.Flags().GetString("action")
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		result, err := ac.EvaluatePolicy(cmd.Context(), args[0], action)
		if err != nil {
			if abac.IsNotFound(err) {
				return fmt.Errorf("API key not found: %s", args[0])
			}
			return fmt.Errorf("evaluating policy: %w", err)
		}
		if output.GetFormat() == output.FormatTable {
			renderEvaluateTable(cmd.OutOrStdout(), result)
			return nil
		}
		return output.FprintRaw(cmd.OutOrStdout(), result.RawJSON)
	},
}

var apiKeyExplainCmd = &cobra.Command{
	Use:   "explain <key-uuid>",
	Short: "Dry-run a policy decision and return the evaluator trace",
	Long: "Like evaluate, but returns the full evaluator trace (matched policies, " +
		"set modes, skipped sets) for human inspection. Always exits 0 on a successful " +
		"API call regardless of allow/deny outcome.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateUUID(args[0], "API key"); err != nil {
			return err
		}
		action, _ := cmd.Flags().GetString("action")
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		result, err := ac.ExplainPolicy(cmd.Context(), args[0], action)
		if err != nil {
			if abac.IsNotFound(err) {
				return fmt.Errorf("API key not found: %s", args[0])
			}
			return fmt.Errorf("explaining policy: %w", err)
		}
		if output.GetFormat() == output.FormatTable {
			renderExplainTable(cmd.OutOrStdout(), result)
			return nil
		}
		return output.FprintRaw(cmd.OutOrStdout(), result.RawJSON)
	},
}

func renderEvaluateTable(w io.Writer, r *abac.EvaluateResponse) {
	fmt.Fprintf(w, "Outcome:                %s\n", r.Outcome)
	fmt.Fprintf(w, "ABAC:                   %s\n", r.ABAC)
	fmt.Fprintf(w, "ABAC shadow:            %s\n", r.ABACShadow)
	fmt.Fprintf(w, "Role allows:            %t\n", r.RoleAllows)
	fmt.Fprintf(w, "Would log disagreement: %t\n", r.WouldLogDisagreement)
}

func renderExplainTable(w io.Writer, r *abac.ExplainResponse) {
	fmt.Fprintf(w, "Action:                 %s\n", r.Trace.Action)
	fmt.Fprintf(w, "Outcome:                %s\n", r.Outcome)
	fmt.Fprintf(w, "ABAC:                   %s\n", r.ABAC)
	fmt.Fprintf(w, "ABAC shadow:            %s\n", r.ABACShadow)
	fmt.Fprintf(w, "Role allows:            %t\n", r.RoleAllows)
	fmt.Fprintf(w, "Registry read-only:     %t\n", r.Trace.RegistryEntry.ReadOnly)
	fmt.Fprintf(w, "Would log disagreement: %t\n", r.WouldLogDisagreement)
	fmt.Fprintln(w)

	if len(r.Trace.EvaluatedSets) == 0 {
		fmt.Fprintln(w, "Evaluated policy sets: (none)")
	} else {
		fmt.Fprintln(w, "Evaluated policy sets:")
		for _, s := range r.Trace.EvaluatedSets {
			header := fmt.Sprintf("  %s (%s, set mode: %s)", s.Name, truncateUUID(s.UUID), s.SetMode)
			if len(s.Matched) == 0 {
				fmt.Fprintln(w, header+" -- no policies matched")
				continue
			}
			fmt.Fprintln(w, header)
			for _, m := range s.Matched {
				fmt.Fprintf(w, "    [%s] %s -- matched via %s\n", m.Effect, m.PolicyID, m.MatchedVia)
			}
		}
	}
	fmt.Fprintln(w)

	if len(r.Trace.SkippedSets) == 0 {
		fmt.Fprintln(w, "Skipped policy sets: (none)")
		return
	}
	fmt.Fprintln(w, "Skipped policy sets:")
	for _, s := range r.Trace.SkippedSets {
		fmt.Fprintf(w, "  %s (%s) -- %s\n", s.Name, truncateUUID(s.UUID), s.Reason)
	}
}

func init() {
	rootCmd.AddCommand(apiKeyCmd)
	apiKeyCmd.AddCommand(apiKeyListCmd)
	apiKeyCmd.AddCommand(apiKeySettingsCmd)
	apiKeyCmd.AddCommand(apiKeyPolicySetsCmd)
	apiKeyCmd.AddCommand(apiKeyEvaluateCmd)
	apiKeyCmd.AddCommand(apiKeyExplainCmd)

	apiKeySettingsCmd.AddCommand(apiKeySettingsGetCmd)
	apiKeySettingsCmd.AddCommand(apiKeySettingsSetCmd)
	apiKeyPolicySetsCmd.AddCommand(apiKeyPolicySetsListCmd)
	apiKeyPolicySetsCmd.AddCommand(apiKeyPolicySetsAttachCmd)
	apiKeyPolicySetsCmd.AddCommand(apiKeyPolicySetsDetachCmd)

	apiKeySettingsSetCmd.Flags().String("mode", "", "ABAC enforcement mode: off, report_only, enforce")
	apiKeyEvaluateCmd.Flags().String("action", "", "Action name to evaluate (e.g. thread.get)")
	apiKeyExplainCmd.Flags().String("action", "", "Action name to explain (e.g. thread.get)")
	_ = apiKeyEvaluateCmd.MarkFlagRequired("action")
	_ = apiKeyExplainCmd.MarkFlagRequired("action")

	for _, cmd := range []*cobra.Command{
		apiKeyListCmd, apiKeySettingsGetCmd, apiKeySettingsSetCmd,
		apiKeyPolicySetsListCmd, apiKeyPolicySetsAttachCmd, apiKeyPolicySetsDetachCmd,
		apiKeyEvaluateCmd, apiKeyExplainCmd,
	} {
		client.SetCredentialType(cmd, client.CredentialBearer)
	}
}
