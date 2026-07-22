package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/getzep/zepctl/internal/client"
	"github.com/getzep/zepctl/internal/config"
	"github.com/getzep/zepctl/internal/output"
	"github.com/spf13/cobra"
)

var configSetAccountCmd = &cobra.Command{
	Use:   "set-account [uuid]",
	Short: "Set the active account for the current profile",
	Long: `Set the account bound to the current profile. It is sent as the
X-Zep-Account-UUID header on bearer requests to disambiguate multi-account
members. If a UUID is provided it is set directly. Otherwise the accounts you
belong to are fetched and you are prompted to choose.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		profile := cfg.GetCurrentProfile()
		if profile == nil {
			return fmt.Errorf("no active profile")
		}

		if len(args) == 1 {
			// Direct UUID assignment.
			profile.AccountUUID = args[0]
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}
			output.Info("Active account set to %s", args[0])
			return nil
		}

		// Interactive account selection.
		httpClient, baseURL, err := client.NewBearerHTTPClient(cmd.Context())
		if err != nil {
			return fmt.Errorf("creating client: %w", err)
		}

		accounts, err := listMyAccounts(cmd.Context(), httpClient, baseURL)
		if err != nil {
			return fmt.Errorf("fetching accounts: %w", err)
		}
		if len(accounts) == 0 {
			return fmt.Errorf("no accounts found for this member")
		}

		var selected accountInfo
		if len(accounts) == 1 {
			selected = accounts[0]
			output.Info("Auto-selected account %q (%s)", selected.Name, selected.UUID)
		} else {
			fmt.Println("Select an account:")
			for i, a := range accounts {
				marker := ""
				if a.RequiresSSO {
					marker = " [SSO -- requires enterprise SSO login]"
				}
				fmt.Printf("  %d. %s (%s)%s\n", i+1, a.Name, a.UUID, marker)
			}
			fmt.Printf("Select an account [1-%d]: ", len(accounts))

			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(response)

			idx, err := strconv.Atoi(response)
			if err != nil || idx < 1 || idx > len(accounts) {
				return fmt.Errorf("invalid selection: %q", response)
			}
			selected = accounts[idx-1]
		}

		profile.AccountUUID = selected.UUID
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		// A SSO-bound account cannot be reached over zepctl's bearer session
		// today; setting it is allowed (the header is still sent) but warn so
		// the user isn't surprised by a 403 on the next command.
		if selected.RequiresSSO {
			output.Warn("Account %s requires enterprise SSO login; zepctl bearer auth cannot reach it yet.", selected.UUID)
		}
		output.Info("Active account set to %s (%s)", selected.UUID, selected.Name)
		return nil
	},
}

type accountInfo struct {
	UUID        string
	Name        string
	RequiresSSO bool
}

// listMyAccounts calls GET /api/web/v1/me/accounts to list every account the
// bearer-authenticated member belongs to. SSO-bound accounts are included and
// flagged via RequiresSSO, which the response annotates for each account.
func listMyAccounts(ctx context.Context, httpClient *http.Client, baseURL string) ([]accountInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/web/v1/me/accounts", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("building me/accounts request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling me/accounts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("me/accounts returned %d", resp.StatusCode)
	}

	var result struct {
		Accounts []struct {
			AccountUUID      string `json:"account_uuid"`
			Name             string `json:"name"`
			RequiresSSOLogin bool   `json:"requires_sso_login"`
		} `json:"accounts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parsing accounts response: %w", err)
	}

	accounts := make([]accountInfo, 0, len(result.Accounts))
	for _, a := range result.Accounts {
		accounts = append(accounts, accountInfo{
			UUID:        a.AccountUUID,
			Name:        a.Name,
			RequiresSSO: a.RequiresSSOLogin,
		})
	}
	return accounts, nil
}

func init() {
	configCmd.AddCommand(configSetAccountCmd)
}
