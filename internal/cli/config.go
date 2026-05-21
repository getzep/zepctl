package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/getzep/zepctl/internal/config"
	"github.com/getzep/zepctl/internal/keyring"
	"github.com/getzep/zepctl/internal/output"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage zepctl configuration",
	Long:  `Manage zepctl configuration including profiles and defaults.`,
}

var configViewCmd = &cobra.Command{
	Use:   "view",
	Short: "Display current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		return output.Print(cfg)
	},
}

var configGetProfilesCmd = &cobra.Command{
	Use:   "get-profiles",
	Short: "List all profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("NAME", "API URL", "CURRENT")
			tbl.WriteHeader()
			for _, p := range cfg.Profiles {
				current := ""
				if p.Name == cfg.CurrentProfile {
					current = "*"
				}
				tbl.WriteRow(p.Name, p.APIURL, current)
			}
			return tbl.Flush()
		}

		return output.Print(cfg.Profiles)
	},
}

var configUseProfileCmd = &cobra.Command{
	Use:   "use-profile <name>",
	Short: "Switch active profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		if cfg.GetProfile(name) == nil {
			return fmt.Errorf("profile %q not found", name)
		}

		cfg.CurrentProfile = name
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		output.Info("Switched to profile %q", name)
		return nil
	},
}

var configAddProfileCmd = &cobra.Command{
	Use:   "add-profile <name>",
	Short: "Add a new profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		noAPIKeyFlag, _ := cmd.Flags().GetBool("no-api-key")

		// Check if profile already exists.
		existing := cfg.GetProfile(name)
		if existing != nil {
			// --no-api-key on an existing profile is ambiguous: either it
			// already has the right shape, or the user wants something else.
			// Tell them to use update-profile instead.
			if noAPIKeyFlag {
				return fmt.Errorf("profile %q already exists; use \"config update-profile\" to modify it", name)
			}
			// If the existing profile has no API key (bearer-only from auth login),
			// offer to add the key to the existing profile.
			creds, err := keyring.GetCredentials(name)
			if err != nil || creds.HasAPIKey() {
				return fmt.Errorf("profile %q already exists", name)
			}

			fmt.Printf("Profile %q has no API key. Add one to this profile? [Y/n]: ", name)
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "" && response != "y" && response != "yes" {
				output.Info("Aborted")
				return nil
			}
		}

		apiKey, _ := cmd.Flags().GetString("api-key")
		envName, _ := cmd.Flags().GetString("env")
		noAPIKey, _ := cmd.Flags().GetBool("no-api-key")

		// Resolve --env up front so we fail fast on unknown names before we
		// touch the keychain.
		var env *config.Environment
		if envName != "" {
			env = cfg.GetEnvironment(envName)
			if env == nil {
				return fmt.Errorf("environment %q not found; configure it with \"zepctl config add-environment\"", envName)
			}
		}

		if !noAPIKey {
			if apiKey == "" {
				fmt.Print("API Key: ")
				if term.IsTerminal(int(os.Stdin.Fd())) {
					keyBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
					fmt.Println() // newline after hidden input
					if err != nil {
						return fmt.Errorf("reading API key: %w", err)
					}
					apiKey = string(keyBytes)
				} else {
					// Fallback for non-terminal input (piped)
					reader := bufio.NewReader(os.Stdin)
					apiKey, _ = reader.ReadString('\n')
				}
				apiKey = strings.TrimSpace(apiKey)
			}

			if apiKey == "" {
				return fmt.Errorf("API key cannot be empty (pass --no-api-key for a bearer-only profile)")
			}

			// Store API key in system keychain (preserves bearer token if present).
			if err := keyring.Set(name, apiKey); err != nil {
				return fmt.Errorf("storing API key: %w", err)
			}
		}

		if existing == nil {
			newProfile := config.Profile{Name: name}
			applyEnvAndOverrides(&newProfile, env, cmd)
			cfg.Profiles = append(cfg.Profiles, newProfile)
		} else {
			applyEnvAndOverrides(existing, env, cmd)
		}

		if cfg.CurrentProfile == "" {
			cfg.CurrentProfile = name
		}

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		switch {
		case existing != nil:
			output.Info("Added API key to existing profile %q", name)
		case noAPIKey:
			output.Info("Added profile %q (no API key -- bearer auth only)", name)
		default:
			output.Info("Added profile %q (API key stored in system keychain)", name)
		}
		return nil
	},
}

var configDeleteProfileCmd = &cobra.Command{
	Use:   "delete-profile <name>",
	Short: "Remove a profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		force, _ := cmd.Flags().GetBool("force")

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		if cfg.GetProfile(name) == nil {
			return fmt.Errorf("profile %q not found", name)
		}

		if !force {
			fmt.Printf("Delete profile %q? [y/N]: ", name)
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				output.Info("Aborted")
				return nil
			}
		}

		var newProfiles []config.Profile
		for _, p := range cfg.Profiles {
			if p.Name != name {
				newProfiles = append(newProfiles, p)
			}
		}
		cfg.Profiles = newProfiles

		if cfg.CurrentProfile == name {
			cfg.CurrentProfile = ""
			if len(cfg.Profiles) > 0 {
				cfg.CurrentProfile = cfg.Profiles[0].Name
			}
		}

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		// Remove API key from keychain (best-effort, after config is saved)
		if err := keyring.Delete(name); err != nil {
			output.Warn("Could not remove API key from keychain: %v", err)
		}

		output.Info("Deleted profile %q", name)
		return nil
	},
}

var configUpdateProfileCmd = &cobra.Command{
	Use:   "update-profile [name]",
	Short: "Update a profile's settings",
	Long: `Update fields on an existing profile. If no name is given, updates the
current active profile. Only the flags you provide are changed; other
fields are left as-is.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		name := cfg.CurrentProfile
		if len(args) == 1 {
			name = args[0]
		}

		profile := cfg.GetProfile(name)
		if profile == nil {
			return fmt.Errorf("profile %q not found", name)
		}

		var env *config.Environment
		if cmd.Flags().Changed("env") {
			envName, _ := cmd.Flags().GetString("env")
			env = cfg.GetEnvironment(envName)
			if env == nil {
				return fmt.Errorf("environment %q not found; configure it with \"zepctl config add-environment\"", envName)
			}
		}

		changed := applyEnvAndOverrides(profile, env, cmd)

		if cmd.Flags().Changed("api-key") {
			v, _ := cmd.Flags().GetString("api-key")
			if v == "" {
				return fmt.Errorf("API key cannot be empty")
			}
			if err := keyring.Set(name, v); err != nil {
				return fmt.Errorf("storing API key: %w", err)
			}
			changed = true
		}
		if setIfChanged(cmd, "project", &profile.ProjectUUID) {
			changed = true
		}
		if setIfChanged(cmd, "account", &profile.AccountUUID) {
			changed = true
		}

		if !changed {
			return fmt.Errorf("no flags provided; use --env, --api-url, --api-key, --project, --account, --oauth-issuer, --oauth-client-id, or --oauth-audience to update a field")
		}

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		output.Info("Updated profile %q", name)
		return nil
	},
}

// applyEnvAndOverrides layers an environment's auth fields onto a profile,
// then lets any explicit per-field flag override the env's value. Returns
// true if any field was set.
func applyEnvAndOverrides(p *config.Profile, env *config.Environment, cmd *cobra.Command) bool {
	changed := false
	if env != nil {
		p.APIURL = env.APIURL
		p.OAuthIssuer = env.OAuthIssuer
		p.OAuthClientID = env.OAuthClientID
		p.OAuthAudience = env.OAuthAudience
		changed = true
	}
	if setIfChanged(cmd, "api-url", &p.APIURL) {
		changed = true
	}
	if setIfChanged(cmd, "oauth-issuer", &p.OAuthIssuer) {
		changed = true
	}
	if setIfChanged(cmd, "oauth-client-id", &p.OAuthClientID) {
		changed = true
	}
	if setIfChanged(cmd, "oauth-audience", &p.OAuthAudience) {
		changed = true
	}
	return changed
}

// setIfChanged copies the named string flag's value into dst if the user
// passed the flag. Returns true if the flag was set.
func setIfChanged(cmd *cobra.Command, name string, dst *string) bool {
	if !cmd.Flags().Changed(name) {
		return false
	}
	v, _ := cmd.Flags().GetString(name)
	*dst = v
	return true
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configViewCmd)
	configCmd.AddCommand(configGetProfilesCmd)
	configCmd.AddCommand(configUseProfileCmd)
	configCmd.AddCommand(configAddProfileCmd)
	configCmd.AddCommand(configUpdateProfileCmd)
	configCmd.AddCommand(configDeleteProfileCmd)

	configAddProfileCmd.Flags().String("api-key", "", "API key for the profile")
	configAddProfileCmd.Flags().String("api-url", "", "API URL for the profile (uses SDK default if not set)")
	configAddProfileCmd.Flags().String("oauth-issuer", "", "Override OIDC issuer for `auth login` (uses build-time default if unset)")
	configAddProfileCmd.Flags().String("oauth-client-id", "", "Override OAuth client ID for `auth login` (uses build-time default if unset)")
	configAddProfileCmd.Flags().String("oauth-audience", "", "Override OAuth audience for `auth login` (uses build-time default if unset)")
	configAddProfileCmd.Flags().String("env", "", "Apply a named environment preset (see \"config add-environment\"); explicit per-field flags override the preset")
	configAddProfileCmd.Flags().Bool("no-api-key", false, "Create a bearer-only profile with no API key (skip prompt)")
	configUpdateProfileCmd.Flags().String("api-key", "", "Update API key (stored in system keychain)")
	configUpdateProfileCmd.Flags().String("api-url", "", "Update API URL")
	configUpdateProfileCmd.Flags().String("project", "", "Update project UUID")
	configUpdateProfileCmd.Flags().String("account", "", "Update account UUID")
	configUpdateProfileCmd.Flags().String("oauth-issuer", "", "Update OIDC issuer override (empty string clears the override)")
	configUpdateProfileCmd.Flags().String("oauth-client-id", "", "Update OAuth client ID override (empty string clears the override)")
	configUpdateProfileCmd.Flags().String("oauth-audience", "", "Update OAuth audience override (empty string clears the override)")
	configUpdateProfileCmd.Flags().String("env", "", "Apply a named environment preset, replacing api-url/oauth-issuer/oauth-client-id/oauth-audience; explicit per-field flags override the preset")
	configDeleteProfileCmd.Flags().Bool("force", false, "Skip confirmation prompt")
}
