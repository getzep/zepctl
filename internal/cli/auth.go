package cli

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/getzep/zepctl/internal/auth"
	"github.com/getzep/zepctl/internal/client"
	"github.com/getzep/zepctl/internal/config"
	"github.com/getzep/zepctl/internal/keyring"
	"github.com/getzep/zepctl/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage bearer token authentication",
	Long:  `Authenticate with Zep using your browser to obtain a bearer token for CLI access.`,
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate via browser to obtain a bearer token",
	RunE: func(cmd *cobra.Command, args []string) error {
		noBrowser, _ := cmd.Flags().GetBool("no-browser")

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		profile := cfg.GetCurrentProfile()
		if profile == nil {
			// No profile exists -- create one. Use the name passed via
			// --profile if provided, otherwise "default", so users can
			// bootstrap an isolated profile via
			// `zepctl --profile foo auth login`.
			name := viper.GetString("profile")
			if name == "" {
				name = "default"
			}

			// Validate --env before the interactive browser flow.
			envName, _ := cmd.Flags().GetString("env")
			newProfile := config.Profile{Name: name, APIURL: auth.DefaultAPIURL}
			if envName != "" {
				env := cfg.GetEnvironment(envName)
				if env == nil {
					return fmt.Errorf("environment %q not found; configure it with \"zepctl config add-environment\"", envName)
				}
				newProfile.APIURL = env.APIURL
				newProfile.OAuthIssuer = env.OAuthIssuer
				newProfile.OAuthClientID = env.OAuthClientID
				newProfile.OAuthAudience = env.OAuthAudience
			}

			cfg.Profiles = append(cfg.Profiles, newProfile)
			cfg.CurrentProfile = name
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("creating profile %q: %w", name, err)
			}
			profile = cfg.GetProfile(name)
		} else if envName, _ := cmd.Flags().GetString("env"); envName != "" {
			// --env on an existing profile is ambiguous (mutate? ignore?). Force
			// the user through update-profile so the change is explicit.
			return fmt.Errorf("profile %q already exists; use \"config update-profile --env %s\" to change its environment", profile.Name, envName)
		}

		// If there's an existing bearer token, revoke its refresh token first.
		oauthCfg := auth.OAuthConfigFor(profile.OAuthIssuer, profile.OAuthClientID, profile.OAuthAudience)
		creds, err := keyring.GetCredentials(profile.Name)
		if err == nil && creds.RefreshToken != "" {
			if err := auth.RevokeToken(cmd.Context(), oauthCfg, creds.RefreshToken); err != nil {
				output.Warn("Could not revoke existing token: %v", err)
			}
		}

		session := auth.NewKeychainSession(profile.Name)
		result, err := auth.Login(cmd.Context(), oauthCfg, session, noBrowser)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		// The SDK persists tokens via SetRawToken, but we also need to
		// store the user email (extracted from the ID token) in the
		// keychain entry.
		email := ""
		if result.IDToken != "" {
			if claims, err := auth.ParseUnverifiedIDToken(result.IDToken); err == nil {
				email = claims.Email
			}
		}
		if email != "" {
			// Update the keychain entry with the email.
			creds, err = keyring.GetCredentials(profile.Name)
			if err == nil {
				creds.UserEmail = email
				_ = keyring.SetCredentials(profile.Name, creds)
			}
		}

		if email != "" {
			output.Info("Authenticated as %s", email)
		} else {
			output.Info("Authenticated successfully")
		}

		// Auto-select project if needed.
		if profile.ProjectUUID == "" {
			if err := autoSelectProject(cmd.Context(), cfg, profile, result.AccessToken); err != nil {
				output.Warn("Could not auto-select project: %v", err)
				output.Info("Run \"zepctl config set-project\" to select a project.")
			}
		}

		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear bearer token for the current profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		profile := cfg.GetCurrentProfile()
		if profile == nil {
			return fmt.Errorf("no active profile")
		}

		// Revoke refresh token at Kinde (best-effort).
		creds, err := keyring.GetCredentials(profile.Name)
		if err == nil && creds.RefreshToken != "" {
			oauthCfg := auth.OAuthConfigFor(profile.OAuthIssuer, profile.OAuthClientID, profile.OAuthAudience)
			if err := auth.RevokeToken(cmd.Context(), oauthCfg, creds.RefreshToken); err != nil {
				output.Warn("Could not revoke token at server: %v", err)
			}
		}

		if err := auth.ClearBearerToken(profile.Name); err != nil {
			return fmt.Errorf("clearing token: %w", err)
		}

		output.Info("Bearer token cleared for profile %q.", profile.Name)
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display authentication status for the current profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		profile := cfg.GetCurrentProfile()
		if profile == nil {
			return fmt.Errorf("no active profile")
		}

		creds, err := keyring.GetCredentials(profile.Name)
		if err != nil {
			return fmt.Errorf("reading credentials: %w", err)
		}

		apiURL := profile.APIURL
		if apiURL == "" {
			apiURL = "(SDK default)"
		}

		oauthCfg := auth.OAuthConfigFor(profile.OAuthIssuer, profile.OAuthClientID, profile.OAuthAudience)

		fmt.Printf("Profile:       %s\n", profile.Name)
		fmt.Printf("API URL:       %s\n", apiURL)
		fmt.Printf("OIDC issuer:   %s\n", oauthCfg.Issuer)

		if creds.HasAPIKey() {
			masked := maskKey(creds.APIKey)
			fmt.Printf("API key:       %s\n", masked)
		} else {
			fmt.Println("API key:       not configured")
		}

		if creds.HasBearerToken() {
			if creds.IsExpired() {
				fmt.Println("Bearer token:  expired")
			} else {
				d := creds.ExpiresIn()
				fmt.Printf("Bearer token:  valid (expires in %s)\n", formatDuration(d))
			}
			if creds.UserEmail != "" {
				fmt.Printf("User:          %s\n", creds.UserEmail)
			}
		} else {
			fmt.Println("Bearer token:  not configured")
		}

		return nil
	},
}

// autoSelectProject resolves the user's account and selects a project
// after auth login.
func autoSelectProject(ctx context.Context, cfg *config.Config, profile *config.Profile, accessToken string) error {
	apiURL := config.GetAPIURL()
	if apiURL == "" {
		apiURL = auth.DefaultAPIURL
	}

	httpClient := &http.Client{
		Transport: &client.BearerTransport{Token: accessToken, Base: http.DefaultTransport},
	}

	accountUUID, projects, err := authenticateAndGetProjects(ctx, httpClient, apiURL)
	if err != nil {
		return err
	}
	profile.AccountUUID = accountUUID
	// Persist account UUID immediately so it survives even if the user has
	// zero projects (early return below) or interactive selection is aborted.
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("saving account UUID: %w", err)
	}

	if len(projects) == 0 {
		return fmt.Errorf("no projects found")
	}

	if len(projects) == 1 {
		profile.ProjectUUID = projects[0].UUID
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		output.Info("Auto-selected project %q (%s)", projects[0].Name, projects[0].UUID)
		return nil
	}

	fmt.Println("Multiple projects found:")
	for i, p := range projects {
		fmt.Printf("  %d. %s (%s)\n", i+1, p.Name, p.UUID)
	}
	fmt.Printf("Select a project [1-%d]: ", len(projects))

	var choice string
	if _, err := fmt.Scanln(&choice); err != nil {
		return fmt.Errorf("reading selection: %w", err)
	}

	idx, err := strconv.Atoi(strings.TrimSpace(choice))
	if err != nil || idx < 1 || idx > len(projects) {
		return fmt.Errorf("invalid selection: %q", choice)
	}

	selected := projects[idx-1]
	profile.ProjectUUID = selected.UUID
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	output.Info("Active project set to %s (%s)", selected.UUID, selected.Name)
	return nil
}

// maskKey returns the first 2 and last 4 characters of a key with "..." in between.
func maskKey(key string) string {
	if len(key) <= 6 {
		return key
	}
	return key[:2] + "..." + key[len(key)-4:]
}

// formatDuration returns a human-readable duration string.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if m == 0 {
		return fmt.Sprintf("%d hours", h)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)

	authLoginCmd.Flags().Bool("no-browser", false, "Print authorization URL instead of opening browser")
	authLoginCmd.Flags().String("env", "", "When creating a new profile, apply a named environment preset (see \"config add-environment\")")
}
