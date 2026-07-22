package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"

	"github.com/getzep/zepctl/internal/abac"
	"github.com/getzep/zepctl/internal/client"
	"github.com/getzep/zepctl/internal/config"
	"github.com/getzep/zepctl/internal/output"
)

// ExitCodeError carries a specific exit code. The main function inspects
// this to exit with a code other than 1 (used by policy-set validate).
type ExitCodeError struct {
	Code int
	Err  error
}

func (e *ExitCodeError) Error() string { return e.Err.Error() }
func (e *ExitCodeError) Unwrap() error { return e.Err }

// newABACClient creates an ABAC API client from the current command context.
// It is a variable so tests can replace it with a version that returns a
// pre-configured client pointing at an httptest server.
var newABACClient = newABACClientDefault

func newABACClientDefault(cmd *cobra.Command) (*abac.Client, error) {
	httpClient, baseURL, err := client.NewBearerHTTPClient(cmd.Context())
	if err != nil {
		return nil, err
	}
	projectUUID := config.GetProjectUUID()
	if projectUUID == "" {
		return nil, fmt.Errorf("no project configured; run \"zepctl config set-project\" to select a project")
	}
	return abac.NewClient(httpClient, baseURL, projectUUID, config.GetAccountUUID()), nil
}

// validateUUID parses value as a UUID and returns a formatted error on failure.
func validateUUID(value, label string) error {
	if _, err := uuid.Parse(value); err != nil {
		return fmt.Errorf("invalid %s UUID: %q", label, value)
	}
	return nil
}

// --- Commands ---

var policySetCmd = &cobra.Command{
	Use:   "policy-set",
	Short: "Manage ABAC policy sets",
	Long:  "Create, list, update, delete, and validate ABAC policy sets.",
}

var policySetListCmd = &cobra.Command{
	Use:   listCmdUse,
	Short: "List policy sets in the current project",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		result, err := ac.ListPolicySets(cmd.Context())
		if err != nil {
			return fmt.Errorf("listing policy sets: %w", err)
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

var policySetGetCmd = &cobra.Command{
	Use:   "get <uuid>",
	Short: "Get a policy set by UUID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateUUID(args[0], "policy set"); err != nil {
			return err
		}
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		ps, err := ac.GetPolicySet(cmd.Context(), args[0])
		if err != nil {
			if abac.IsNotFound(err) {
				return fmt.Errorf("policy set not found: %s", args[0])
			}
			return fmt.Errorf("getting policy set: %w", err)
		}
		if output.GetFormat() == output.FormatTable {
			fmt.Fprintf(cmd.OutOrStdout(), "UUID:         %s\n", ps.UUID)
			fmt.Fprintf(cmd.OutOrStdout(), "Name:         %s\n", ps.Name)
			if ps.Description != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Description:  %s\n", ps.Description)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Mode:         %s\n", ps.Mode)
			fmt.Fprintf(cmd.OutOrStdout(), "Version:      %d\n", ps.Version)
			fmt.Fprintf(cmd.OutOrStdout(), "Project:      %s\n", ps.ProjectUUID)
			fmt.Fprintf(cmd.OutOrStdout(), "Created:      %s\n", ps.CreatedAt)
			fmt.Fprintf(cmd.OutOrStdout(), "Updated:      %s\n", ps.UpdatedAt)
			if ps.Spec != nil {
				specYAML, err := yaml.Marshal(ps.Spec)
				if err == nil {
					fmt.Fprintf(cmd.OutOrStdout(), "\nSpec:\n")
					for _, line := range strings.Split(strings.TrimRight(string(specYAML), "\n"), "\n") {
						fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", line)
					}
				}
			}
			return nil
		}
		return output.Print(ps)
	},
}

var policySetCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a policy set from a YAML file",
	RunE: func(cmd *cobra.Command, _ []string) error {
		filePath, _ := cmd.Flags().GetString("file")
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		ps, err := ac.CreatePolicySet(cmd.Context(), string(data))
		if err != nil {
			return fmt.Errorf("creating policy set: %w", err)
		}
		if output.GetFormat() == output.FormatTable {
			output.Info("Created policy set %q (%s..., version %d)", ps.Name, truncateUUID(ps.UUID), ps.Version)
			return nil
		}
		return output.Print(ps)
	},
}

var policySetUpdateCmd = &cobra.Command{
	Use:   "update <uuid>",
	Short: "Update a policy set from a YAML file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateUUID(args[0], "policy set"); err != nil {
			return err
		}
		filePath, _ := cmd.Flags().GetString("file")
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		ps, err := ac.UpdatePolicySet(cmd.Context(), args[0], string(data))
		if err != nil {
			if abac.IsNotFound(err) {
				return fmt.Errorf("policy set not found: %s", args[0])
			}
			return fmt.Errorf("updating policy set: %w", err)
		}
		if output.GetFormat() == output.FormatTable {
			output.Info("Updated policy set %q (version %d)", ps.Name, ps.Version)
			return nil
		}
		return output.Print(ps)
	},
}

var policySetDeleteCmd = &cobra.Command{
	Use:   "delete <uuid>",
	Short: "Delete a policy set",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateUUID(args[0], "policy set"); err != nil {
			return err
		}
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				return fmt.Errorf("use --force to delete without confirmation")
			}
			fmt.Fprintf(cmd.ErrOrStderr(),
				"Delete policy set %s? This will also remove all attachments. [y/N]: ", args[0])
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				return nil
			}
		}
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		if err := ac.DeletePolicySet(cmd.Context(), args[0]); err != nil {
			if abac.IsNotFound(err) {
				return fmt.Errorf("policy set not found: %s", args[0])
			}
			return fmt.Errorf("deleting policy set: %w", err)
		}
		output.Info("Deleted policy set %s", args[0])
		return nil
	},
}

var policySetValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a policy set YAML file",
	RunE: func(cmd *cobra.Command, _ []string) error {
		filePath, _ := cmd.Flags().GetString("file")
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}
		ac, err := newABACClient(cmd)
		if err != nil {
			return &ExitCodeError{Code: 2, Err: err}
		}
		result, err := ac.ValidatePolicySet(cmd.Context(), string(data))
		if err != nil {
			return &ExitCodeError{Code: 2, Err: fmt.Errorf("validating policy set: %w", err)}
		}

		if output.GetFormat() != output.FormatTable {
			if err := output.Print(result); err != nil {
				return err
			}
		}

		if result.Valid {
			if output.GetFormat() == output.FormatTable {
				output.Info("Validation passed.")
			}
			return nil
		}

		// Invalid -- print errors and exit 1.
		if output.GetFormat() == output.FormatTable {
			fmt.Fprintln(cmd.ErrOrStderr(), "Validation failed:")
			for _, ve := range result.Errors {
				if ve.PolicyID != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "  - %s (policy: %s)\n", ve.Message, ve.PolicyID)
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "  - %s\n", ve.Message)
				}
			}
		}
		cmd.SilenceErrors = true
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("validation failed")}
	},
}

// truncateUUID returns the first 8 characters of a UUID followed by "...".
func truncateUUID(id string) string {
	if len(id) > 8 {
		return id[:8] + "..."
	}
	return id
}

func init() {
	rootCmd.AddCommand(policySetCmd)
	policySetCmd.AddCommand(policySetListCmd)
	policySetCmd.AddCommand(policySetGetCmd)
	policySetCmd.AddCommand(policySetCreateCmd)
	policySetCmd.AddCommand(policySetUpdateCmd)
	policySetCmd.AddCommand(policySetDeleteCmd)
	policySetCmd.AddCommand(policySetValidateCmd)

	policySetCreateCmd.Flags().String("file", "", "Path to policy set YAML file")
	_ = policySetCreateCmd.MarkFlagRequired("file")
	policySetUpdateCmd.Flags().String("file", "", "Path to updated policy set YAML file")
	_ = policySetUpdateCmd.MarkFlagRequired("file")
	policySetValidateCmd.Flags().String("file", "", "Path to policy set YAML file to validate")
	_ = policySetValidateCmd.MarkFlagRequired("file")
	policySetDeleteCmd.Flags().Bool("force", false, "Skip confirmation prompt")

	for _, cmd := range []*cobra.Command{
		policySetListCmd, policySetGetCmd, policySetCreateCmd,
		policySetUpdateCmd, policySetDeleteCmd, policySetValidateCmd,
	} {
		client.SetCredentialType(cmd, client.CredentialBearer)
	}
}
