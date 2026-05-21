package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/getzep/zepctl/internal/config"
	"github.com/getzep/zepctl/internal/output"
	"github.com/spf13/cobra"
)

var configGetEnvironmentsCmd = &cobra.Command{
	Use:   "get-environments",
	Short: "List all environment presets",
	RunE: func(_ *cobra.Command, _ []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("NAME", "API URL", "OAUTH ISSUER", "OAUTH CLIENT ID", "OAUTH AUDIENCE")
			tbl.WriteHeader()
			for _, e := range cfg.Environments {
				tbl.WriteRow(e.Name, e.APIURL, e.OAuthIssuer, e.OAuthClientID, e.OAuthAudience)
			}
			return tbl.Flush()
		}

		return output.Print(cfg.Environments)
	},
}

var configAddEnvironmentCmd = &cobra.Command{
	Use:   "add-environment <name>",
	Short: "Add a named environment preset (api-url, oauth-issuer, oauth-client-id)",
	Long: `Add a reusable environment preset that can be applied to profiles via
"config add-profile --env <name>" or "config update-profile --env <name>".

Environments are stored in the user's config file, not the binary, so
non-default endpoints stay out of distributed builds.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		if cfg.GetEnvironment(name) != nil {
			return fmt.Errorf("environment %q already exists; use \"config update-environment\" to modify it", name)
		}

		apiURL, _ := cmd.Flags().GetString("api-url")
		oauthIssuer, _ := cmd.Flags().GetString("oauth-issuer")
		oauthClientID, _ := cmd.Flags().GetString("oauth-client-id")
		oauthAudience, _ := cmd.Flags().GetString("oauth-audience")

		cfg.Environments = append(cfg.Environments, config.Environment{
			Name:          name,
			APIURL:        apiURL,
			OAuthIssuer:   oauthIssuer,
			OAuthClientID: oauthClientID,
			OAuthAudience: oauthAudience,
		})

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		output.Info("Added environment %q", name)
		return nil
	},
}

var configUpdateEnvironmentCmd = &cobra.Command{
	Use:   "update-environment <name>",
	Short: "Update an environment's settings",
	Long: `Update fields on an existing environment. Only the flags you provide are
changed; other fields are left as-is. Pass an empty string to clear a field.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		env := cfg.GetEnvironment(name)
		if env == nil {
			return fmt.Errorf("environment %q not found", name)
		}

		changed := setIfChanged(cmd, "api-url", &env.APIURL)
		if setIfChanged(cmd, "oauth-issuer", &env.OAuthIssuer) {
			changed = true
		}
		if setIfChanged(cmd, "oauth-client-id", &env.OAuthClientID) {
			changed = true
		}
		if setIfChanged(cmd, "oauth-audience", &env.OAuthAudience) {
			changed = true
		}

		if !changed {
			return fmt.Errorf("no flags provided; use --api-url, --oauth-issuer, --oauth-client-id, or --oauth-audience to update a field")
		}

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		output.Info("Updated environment %q", name)
		return nil
	},
}

var configDeleteEnvironmentCmd = &cobra.Command{
	Use:   "delete-environment <name>",
	Short: "Remove an environment preset",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		force, _ := cmd.Flags().GetBool("force")

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		if cfg.GetEnvironment(name) == nil {
			return fmt.Errorf("environment %q not found", name)
		}

		if !force {
			fmt.Printf("Delete environment %q? [y/N]: ", name)
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				output.Info("Aborted")
				return nil
			}
		}

		var kept []config.Environment
		for _, e := range cfg.Environments {
			if e.Name != name {
				kept = append(kept, e)
			}
		}
		cfg.Environments = kept

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		output.Info("Deleted environment %q", name)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configGetEnvironmentsCmd)
	configCmd.AddCommand(configAddEnvironmentCmd)
	configCmd.AddCommand(configUpdateEnvironmentCmd)
	configCmd.AddCommand(configDeleteEnvironmentCmd)

	configAddEnvironmentCmd.Flags().String("api-url", "", "API URL for the environment")
	configAddEnvironmentCmd.Flags().String("oauth-issuer", "", "OIDC issuer for the environment")
	configAddEnvironmentCmd.Flags().String("oauth-client-id", "", "OAuth client ID for the environment")
	configAddEnvironmentCmd.Flags().String("oauth-audience", "", "OAuth audience for bearer token aud claim")

	configUpdateEnvironmentCmd.Flags().String("api-url", "", "Update API URL (empty string clears it)")
	configUpdateEnvironmentCmd.Flags().String("oauth-issuer", "", "Update OIDC issuer (empty string clears it)")
	configUpdateEnvironmentCmd.Flags().String("oauth-client-id", "", "Update OAuth client ID (empty string clears it)")
	configUpdateEnvironmentCmd.Flags().String("oauth-audience", "", "Update OAuth audience (empty string clears it)")

	configDeleteEnvironmentCmd.Flags().Bool("force", false, "Skip confirmation prompt")
}
