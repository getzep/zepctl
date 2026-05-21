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

var configSetProjectCmd = &cobra.Command{
	Use:   "set-project [uuid]",
	Short: "Set the active project for the current profile",
	Long: `Set the active project for the current profile. If a UUID is provided,
it is set directly. Otherwise, the accessible projects are fetched and
you are prompted to choose.`,
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
			profile.ProjectUUID = args[0]
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}
			output.Info("Active project set to %s", args[0])
			return nil
		}

		// Interactive project selection.
		httpClient, baseURL, err := client.NewBearerHTTPClient(cmd.Context())
		if err != nil {
			return fmt.Errorf("creating client: %w", err)
		}

		accountUUID, projects, err := authenticateAndGetProjects(cmd.Context(), httpClient, baseURL)
		if err != nil {
			return fmt.Errorf("fetching account and projects: %w", err)
		}
		profile.AccountUUID = accountUUID
		// Persist the resolved account UUID immediately so it survives even
		// if there are zero projects or the user aborts interactive
		// selection. Mirrors the auth login path in auth.go.
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving account UUID: %w", err)
		}

		if len(projects) == 0 {
			return fmt.Errorf("no projects found for this account")
		}

		var selected projectInfo
		if len(projects) == 1 {
			selected = projects[0]
			output.Info("Auto-selected project %q (%s)", selected.Name, selected.UUID)
		} else {
			fmt.Println("Select a project:")
			for i, p := range projects {
				fmt.Printf("  %d. %s (%s)\n", i+1, p.Name, p.UUID)
			}
			fmt.Printf("Select a project [1-%d]: ", len(projects))

			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(response)

			idx, err := strconv.Atoi(response)
			if err != nil || idx < 1 || idx > len(projects) {
				return fmt.Errorf("invalid selection: %q", response)
			}
			selected = projects[idx-1]
		}

		profile.ProjectUUID = selected.UUID
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		output.Info("Active project set to %s (%s)", selected.UUID, selected.Name)
		return nil
	},
}

type projectInfo struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

// authenticateAndGetProjects calls POST /api/web/v1/authenticate to resolve
// the bearer-authenticated user's account UUID and accessible projects in a
// single round trip. The endpoint returns AccountMemberResponse, which carries
// both account_uuid and the projects array (src/api/apidata/account.go).
func authenticateAndGetProjects(ctx context.Context, httpClient *http.Client, baseURL string) (string, []projectInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/web/v1/authenticate", http.NoBody)
	if err != nil {
		return "", nil, fmt.Errorf("building authenticate request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("calling authenticate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("authenticate returned %d", resp.StatusCode)
	}

	var result struct {
		AccountUUID string        `json:"account_uuid"`
		Projects    []projectInfo `json:"projects"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", nil, fmt.Errorf("parsing authenticate response: %w", err)
	}
	if result.AccountUUID == "" {
		return "", nil, fmt.Errorf("no account_uuid in authenticate response")
	}
	return result.AccountUUID, result.Projects, nil
}

func init() {
	configCmd.AddCommand(configSetProjectCmd)
}
