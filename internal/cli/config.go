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

		if cfg.GetProfile(name) != nil {
			return fmt.Errorf("profile %q already exists", name)
		}

		apiKey, _ := cmd.Flags().GetString("api-key")
		apiURL, _ := cmd.Flags().GetString("api-url")

		if apiKey == "" {
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("API Key: ")
			apiKey, _ = reader.ReadString('\n')
			apiKey = strings.TrimSpace(apiKey)
		}

		if apiURL == "" {
			apiURL = "https://api.getzep.com"
		}

		cfg.Profiles = append(cfg.Profiles, config.Profile{
			Name:   name,
			APIKey: apiKey,
			APIURL: apiURL,
		})

		if cfg.CurrentProfile == "" {
			cfg.CurrentProfile = name
		}

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		output.Info("Added profile %q", name)
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

		output.Info("Deleted profile %q", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configViewCmd)
	configCmd.AddCommand(configGetProfilesCmd)
	configCmd.AddCommand(configUseProfileCmd)
	configCmd.AddCommand(configAddProfileCmd)
	configCmd.AddCommand(configDeleteProfileCmd)

	configAddProfileCmd.Flags().String("api-key", "", "API key for the profile")
	configAddProfileCmd.Flags().String("api-url", "https://api.getzep.com", "API URL for the profile")
	configDeleteProfileCmd.Flags().Bool("force", false, "Skip confirmation prompt")
}
