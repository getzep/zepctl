package cli

import (
	"fmt"
	"io"
	"sort"
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
		return runEvaluate(cmd, abac.Principal{Type: abac.PrincipalAPIKey, UUID: args[0]}, "API key")
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
		return runExplain(cmd, abac.Principal{Type: abac.PrincipalAPIKey, UUID: args[0]}, "API key")
	},
}

// runEvaluate performs an evaluate dry run for principal and renders the
// result. label names the principal in a not-found message. Commands
// that accept --graph pass a resource; the others cannot.
func runEvaluate(cmd *cobra.Command, principal abac.Principal, label string) error {
	action, _ := cmd.Flags().GetString("action")
	resource, err := dryRunResource(cmd)
	if err != nil {
		return err
	}
	ac, err := newABACClient(cmd)
	if err != nil {
		return err
	}
	result, err := ac.EvaluatePolicy(cmd.Context(), principal, action, resource)
	if err != nil {
		return dryRunError(err, "evaluating policy", principal, label, resource)
	}
	if output.GetFormat() == output.FormatTable {
		renderEvaluateTable(cmd.OutOrStdout(), result)
		return nil
	}
	return output.FprintRaw(cmd.OutOrStdout(), result.RawJSON)
}

// runExplain is runEvaluate plus the decision trace.
func runExplain(cmd *cobra.Command, principal abac.Principal, label string) error {
	action, _ := cmd.Flags().GetString("action")
	resource, err := dryRunResource(cmd)
	if err != nil {
		return err
	}
	ac, err := newABACClient(cmd)
	if err != nil {
		return err
	}
	result, err := ac.ExplainPolicy(cmd.Context(), principal, action, resource)
	if err != nil {
		return dryRunError(err, "explaining policy", principal, label, resource)
	}
	if output.GetFormat() == output.FormatTable {
		renderExplainTable(cmd.OutOrStdout(), result)
		return nil
	}
	return output.FprintRaw(cmd.OutOrStdout(), result.RawJSON)
}

// dryRunError names the principal in a not-found message only when the
// request carried no resource. A dry run with --graph gets the same 404
// for an absent, deleted, or cross-project graph, so the server's message
// has to decide which object was not found.
func dryRunError(err error, action string, principal abac.Principal, label string, resource *abac.Resource) error {
	if resource == nil && abac.IsNotFound(err) {
		return fmt.Errorf("%s not found: %s", label, principal.UUID)
	}
	return fmt.Errorf("%s: %w", action, err)
}

// dryRunResource builds the optional graph resource from --graph. The
// value is a standalone graph's canonical UUID, not a graph ID. Commands
// without the flag always evaluate action-only.
func dryRunResource(cmd *cobra.Command) (*abac.Resource, error) {
	graphUUID, _ := cmd.Flags().GetString("graph")
	if graphUUID == "" {
		return nil, nil
	}
	if err := validateUUID(graphUUID, "graph"); err != nil {
		return nil, err
	}
	return &abac.Resource{Type: abac.ResourceGraph, CanonicalGraphUUID: graphUUID}, nil
}

func renderEvaluateTable(w io.Writer, r *abac.EvaluateResponse) {
	renderPrincipalAndResource(w, r.Principal, r.Resource)
	fmt.Fprintf(w, "Outcome:                %s\n", r.Outcome)
	fmt.Fprintf(w, "ABAC:                   %s\n", r.ABAC)
	fmt.Fprintf(w, "ABAC shadow:            %s\n", r.ABACShadow)
	fmt.Fprintf(w, "Role allows:            %t\n", r.RoleAllows)
	renderBase(w, r.Base, r.BaseAllows)
	fmt.Fprintf(w, "Would log disagreement: %t\n", r.WouldLogDisagreement)
}

func renderExplainTable(w io.Writer, r *abac.ExplainResponse) {
	fmt.Fprintf(w, "Action:                 %s\n", r.Trace.Action)
	renderPrincipalAndResource(w, r.Principal, r.Resource)
	fmt.Fprintf(w, "Outcome:                %s\n", r.Outcome)
	fmt.Fprintf(w, "ABAC:                   %s\n", r.ABAC)
	fmt.Fprintf(w, "ABAC shadow:            %s\n", r.ABACShadow)
	fmt.Fprintf(w, "Role allows:            %t\n", r.RoleAllows)
	fmt.Fprintf(w, "Registry read-only:     %t\n", r.Trace.RegistryEntry.ReadOnly)
	renderBase(w, r.Base, r.BaseAllows)
	fmt.Fprintf(w, "Would log disagreement: %t\n", r.WouldLogDisagreement)
	fmt.Fprintln(w)

	renderEffectiveGroups(w, r.Trace.ResolvedGroupUUIDs)
	renderEvaluatedSets(w, r.Trace.EvaluatedSets)
	fmt.Fprintln(w)
	renderSkippedSets(w, r.Trace.SkippedSets)
	renderObjectConstraints(w, r.Trace.ObjectConstraints)
	renderDeadGraphUUIDs(w, r.Trace.DeadGraphUUIDs)
}

// renderPrincipalAndResource prints the resolved principal and resource.
// Both are absent for API key dry runs, whose output is unchanged.
func renderPrincipalAndResource(w io.Writer, principal *abac.Principal, resource *abac.Resource) {
	if principal != nil {
		fmt.Fprintf(w, "Principal:              %s %s\n", principal.Type, principal.UUID)
	}
	if resource != nil {
		fmt.Fprintf(w, "Resource:               %s %s\n", resource.Type, resource.CanonicalGraphUUID)
	}
}

// renderBase prints the evaluation base. User and user group principals
// report a default-deny base; API key principals report none.
func renderBase(w io.Writer, base string, baseAllows *bool) {
	if base != "" {
		fmt.Fprintf(w, "Base:                   %s\n", base)
	}
	if baseAllows != nil {
		fmt.Fprintf(w, "Base allows:            %t\n", *baseAllows)
	}
}

func renderEffectiveGroups(w io.Writer, groupUUIDs []string) {
	if len(groupUUIDs) == 0 {
		return
	}
	fmt.Fprintln(w, "Effective UserGroups:")
	for _, groupUUID := range groupUUIDs {
		fmt.Fprintf(w, "  %s\n", groupUUID)
	}
	fmt.Fprintln(w)
}

func renderEvaluatedSets(w io.Writer, sets []abac.ExplainEvaluatedSet) {
	if len(sets) == 0 {
		fmt.Fprintln(w, "Evaluated policy sets: (none)")
		return
	}
	fmt.Fprintln(w, "Evaluated policy sets:")
	for _, s := range sets {
		header := fmt.Sprintf("  %s (%s, set mode: %s", s.Name, truncateUUID(s.UUID), s.SetMode)
		if len(s.ContributingGroupUUIDs) > 0 {
			header += ", groups: " + strings.Join(truncateUUIDs(s.ContributingGroupUUIDs), ", ")
		}
		header += ")"
		if len(s.Matched) == 0 && len(s.Skipped) == 0 {
			fmt.Fprintln(w, header+" -- no policies matched")
			continue
		}
		fmt.Fprintln(w, header)
		for _, m := range s.Matched {
			fmt.Fprintf(w, "    [%s] %s -- matched via %s%s\n",
				m.Effect, m.PolicyID, m.MatchedVia, resourceMatchSuffix(m.ResourceMatch))
		}
		for _, s := range s.Skipped {
			fmt.Fprintf(w, "    (skipped) [%s] %s%s\n",
				s.Effect, s.PolicyID, resourceMatchSuffix(s.ResourceMatch))
		}
	}
}

func renderSkippedSets(w io.Writer, sets []abac.ExplainSkippedSet) {
	if len(sets) == 0 {
		fmt.Fprintln(w, "Skipped policy sets: (none)")
		return
	}
	fmt.Fprintln(w, "Skipped policy sets:")
	for _, s := range sets {
		fmt.Fprintf(w, "  %s (%s) -- %s\n", s.Name, truncateUUID(s.UUID), s.Reason)
	}
}

func renderObjectConstraints(w io.Writer, constraints *abac.ExplainObjectConstraints) {
	if constraints == nil || (len(constraints.Include) == 0 && len(constraints.Exclude) == 0) {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Object constraints:")
	renderConstraintGroups(w, "include", constraints.Include)
	renderConstraintGroups(w, "exclude", constraints.Exclude)
}

func renderConstraintGroups(w io.Writer, label string, groups []abac.ExplainConstraintGroup) {
	for _, g := range groups {
		keys := make([]string, 0, len(g.Attributes))
		for key := range g.Attributes {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		pairs := make([]string, 0, len(keys))
		for _, key := range keys {
			pairs = append(pairs, fmt.Sprintf("%s=%v", key, g.Attributes[key]))
		}
		fmt.Fprintf(w, "  %s: %s (subset match: %t)\n", label, strings.Join(pairs, " "), g.SubsetMatch)
	}
}

// renderDeadGraphUUIDs lists graph selectors that no longer resolve to a
// live graph in the project.
func renderDeadGraphUUIDs(w io.Writer, graphUUIDs []string) {
	if len(graphUUIDs) == 0 {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Dead graph references:")
	for _, graphUUID := range graphUUIDs {
		fmt.Fprintf(w, "  %s\n", graphUUID)
	}
}

func resourceMatchSuffix(resourceMatch string) string {
	if resourceMatch == "" {
		return ""
	}
	return " -- resource " + resourceMatch
}

func truncateUUIDs(ids []string) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = truncateUUID(id)
	}
	return out
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
