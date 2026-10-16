package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/getzep/zep-go/v3"
	"github.com/getzep/zepctl/internal/abac"
	"github.com/getzep/zepctl/internal/client"
	"github.com/getzep/zepctl/internal/output"
	"github.com/spf13/cobra"
)

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Manage users",
	Long: `Create, list, update, and delete users, inspect their effective UserGroups,
and dry-run ABAC decisions for a user principal.`,
}

var userListCmd = &cobra.Command{
	Use:   listCmdUse,
	Short: "List users",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		page, _ := cmd.Flags().GetInt("page")
		pageSize, _ := cmd.Flags().GetInt("page-size")
		search, _ := cmd.Flags().GetString("search")
		orderBy, _ := cmd.Flags().GetString("order-by")
		asc, _ := cmd.Flags().GetBool("asc")

		req := &zep.UserListOrderedRequest{
			PageNumber: zep.Int(page),
			PageSize:   zep.Int(pageSize),
		}
		if search != "" {
			req.Search = zep.String(search)
		}
		if orderBy != "" {
			req.OrderBy = zep.String(orderBy)
		}
		if cmd.Flags().Changed("asc") {
			req.Asc = zep.Bool(asc)
		}

		users, err := c.User.ListOrdered(context.Background(), req)
		if err != nil {
			return fmt.Errorf("listing users: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("USER ID", "EMAIL", "FIRST NAME", "LAST NAME", "CREATED AT")
			tbl.WriteHeader()
			for _, u := range users.Users {
				email := ""
				if u.Email != nil {
					email = *u.Email
				}
				firstName := ""
				if u.FirstName != nil {
					firstName = *u.FirstName
				}
				lastName := ""
				if u.LastName != nil {
					lastName = *u.LastName
				}
				createdAt := ""
				if u.CreatedAt != nil {
					createdAt = *u.CreatedAt
				}
				userID := ""
				if u.UserID != nil {
					userID = *u.UserID
				}
				tbl.WriteRow(userID, email, firstName, lastName, createdAt)
			}
			return tbl.Flush()
		}

		return output.Print(users)
	},
}

var userGetCmd = &cobra.Command{
	Use:   "get <user-id>",
	Short: "Get user details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		userID := args[0]

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		user, err := c.User.Get(context.Background(), userID)
		if err != nil {
			return fmt.Errorf("getting user: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("FIELD", "VALUE")
			tbl.WriteHeader()
			userIDStr := ""
			if user.UserID != nil {
				userIDStr = *user.UserID
			}
			tbl.WriteRow("User ID", userIDStr)
			if user.UUID != nil {
				tbl.WriteRow("UUID", *user.UUID)
			}
			if user.Email != nil {
				tbl.WriteRow("Email", *user.Email)
			}
			if user.FirstName != nil {
				tbl.WriteRow("First Name", *user.FirstName)
			}
			if user.LastName != nil {
				tbl.WriteRow("Last Name", *user.LastName)
			}
			if user.CreatedAt != nil {
				tbl.WriteRow("Created At", *user.CreatedAt)
			}
			return tbl.Flush()
		}

		return output.Print(user)
	},
}

var userCreateCmd = &cobra.Command{
	Use:   "create <user-id>",
	Short: "Create a new user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		userID := args[0]

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		email, _ := cmd.Flags().GetString("email")
		firstName, _ := cmd.Flags().GetString("first-name")
		lastName, _ := cmd.Flags().GetString("last-name")
		metadataStr, _ := cmd.Flags().GetString("metadata")
		metadataFile, _ := cmd.Flags().GetString("metadata-file")

		req := &zep.CreateUserRequest{
			UserID: userID,
		}

		if email != "" {
			req.Email = zep.String(email)
		}
		if firstName != "" {
			req.FirstName = zep.String(firstName)
		}
		if lastName != "" {
			req.LastName = zep.String(lastName)
		}

		if metadataFile != "" {
			data, err := os.ReadFile(metadataFile)
			if err != nil {
				return fmt.Errorf("reading metadata file: %w", err)
			}
			metadataStr = string(data)
		}

		if metadataStr != "" {
			var metadata map[string]any
			if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
				return fmt.Errorf("parsing metadata: %w", err)
			}
			req.Metadata = metadata
		}

		user, err := c.User.Add(context.Background(), req)
		if err != nil {
			return fmt.Errorf("creating user: %w", err)
		}

		output.Info("Created user %q", userID)
		return output.Print(user)
	},
}

var userUpdateCmd = &cobra.Command{
	Use:   "update <user-id>",
	Short: "Update an existing user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		userID := args[0]

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		email, _ := cmd.Flags().GetString("email")
		firstName, _ := cmd.Flags().GetString("first-name")
		lastName, _ := cmd.Flags().GetString("last-name")
		metadataStr, _ := cmd.Flags().GetString("metadata")
		metadataFile, _ := cmd.Flags().GetString("metadata-file")

		req := &zep.UpdateUserRequest{}

		if email != "" {
			req.Email = zep.String(email)
		}
		if firstName != "" {
			req.FirstName = zep.String(firstName)
		}
		if lastName != "" {
			req.LastName = zep.String(lastName)
		}

		if metadataFile != "" {
			data, err := os.ReadFile(metadataFile)
			if err != nil {
				return fmt.Errorf("reading metadata file: %w", err)
			}
			metadataStr = string(data)
		}

		if metadataStr != "" {
			var metadata map[string]any
			if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
				return fmt.Errorf("parsing metadata: %w", err)
			}
			req.Metadata = metadata
		}

		user, err := c.User.Update(context.Background(), userID, req)
		if err != nil {
			return fmt.Errorf("updating user: %w", err)
		}

		output.Info("Updated user %q", userID)
		return output.Print(user)
	},
}

var userDeleteCmd = &cobra.Command{
	Use:   "delete <user-id>",
	Short: "Delete a user",
	Long:  `Delete a user and all associated data (threads, graph, knowledge). Supports RTBF compliance.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		userID := args[0]
		force, _ := cmd.Flags().GetBool("force")

		if !force {
			fmt.Printf("Delete user %q and all associated data? This cannot be undone. [y/N]: ", userID)
			if !confirmYesNo() {
				output.Info("Aborted")
				return nil
			}
		}

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		if _, err := c.User.Delete(context.Background(), userID); err != nil {
			return fmt.Errorf("deleting user: %w", err)
		}

		output.Info("Deleted user %q", userID)
		return nil
	},
}

var userThreadsCmd = &cobra.Command{
	Use:   "threads <user-id>",
	Short: "List user threads",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		userID := args[0]

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		threads, err := c.User.GetThreads(context.Background(), userID)
		if err != nil {
			return fmt.Errorf("getting user threads: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("THREAD ID", "CREATED AT")
			tbl.WriteHeader()
			for _, t := range threads {
				createdAt := ""
				if t.CreatedAt != nil {
					createdAt = *t.CreatedAt
				}
				threadID := ""
				if t.ThreadID != nil {
					threadID = *t.ThreadID
				}
				tbl.WriteRow(threadID, createdAt)
			}
			return tbl.Flush()
		}

		return output.Print(threads)
	},
}

var userNodeCmd = &cobra.Command{
	Use:   "node <user-id>",
	Short: "Get user graph node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		userID := args[0]

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		node, err := c.User.GetNode(context.Background(), userID)
		if err != nil {
			return fmt.Errorf("getting user node: %w", err)
		}

		return output.Print(node)
	},
}

// --- ABAC subcommands (bearer auth, keyed by user UUID) ---

var userMembershipsCmd = &cobra.Command{
	Use:   "groups <user-uuid>",
	Short: "List the effective UserGroups of a user",
	Long: "Lists the user's effective UserGroups: the virtual \"All Users\" group plus every\n" +
		"active managed membership. The user is identified by UUID, not by user ID.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateUUID(args[0], "user"); err != nil {
			return err
		}
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		result, err := ac.ListUserGroupsForUser(cmd.Context(), args[0])
		if err != nil {
			if abac.IsNotFound(err) {
				return fmt.Errorf("user not found: %s", args[0])
			}
			return fmt.Errorf("listing user groups: %w", err)
		}
		if output.GetFormat() == output.FormatTable {
			return renderUserGroupTable(result.UserGroups)
		}
		return output.Print(result)
	},
}

var userEvaluateCmd = &cobra.Command{
	Use:   "evaluate <user-uuid>",
	Short: "Dry-run a policy decision for a user principal",
	Long: "Asks the server what the evaluator would decide for the given action and the\n" +
		"policy sets attached to the user's effective UserGroups. The base posture is\n" +
		"default-deny and a deny from any group wins. Pass --graph to evaluate a\n" +
		"specific standalone graph resource. The user is identified by UUID, not by\n" +
		"user ID. Always exits 0 on a successful API call regardless of outcome.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateUUID(args[0], "user"); err != nil {
			return err
		}
		return runEvaluate(cmd, abac.Principal{Type: abac.PrincipalUser, UUID: args[0]}, "user")
	},
}

var userExplainCmd = &cobra.Command{
	Use:   "explain <user-uuid>",
	Short: "Dry-run a policy decision for a user principal and return the trace",
	Long: "Like evaluate, but returns the full evaluator trace: effective UserGroups, the\n" +
		"groups that contributed each policy set, matched and skipped policies, resource\n" +
		"matches, and object constraints. Always exits 0 on a successful API call.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateUUID(args[0], "user"); err != nil {
			return err
		}
		return runExplain(cmd, abac.Principal{Type: abac.PrincipalUser, UUID: args[0]}, "user")
	},
}

func init() {
	rootCmd.AddCommand(userCmd)
	userCmd.AddCommand(userListCmd)
	userCmd.AddCommand(userGetCmd)
	userCmd.AddCommand(userCreateCmd)
	userCmd.AddCommand(userUpdateCmd)
	userCmd.AddCommand(userDeleteCmd)
	userCmd.AddCommand(userThreadsCmd)
	userCmd.AddCommand(userNodeCmd)
	userCmd.AddCommand(userMembershipsCmd)
	userCmd.AddCommand(userEvaluateCmd)
	userCmd.AddCommand(userExplainCmd)

	// List flags
	userListCmd.Flags().Int("page", 1, "Page number")
	userListCmd.Flags().Int("page-size", 50, "Results per page")
	userListCmd.Flags().String("search", "", "Search by user_id, name, or email")
	userListCmd.Flags().String("order-by", "", "Sort by column (created_at, user_id, email)")
	userListCmd.Flags().Bool("asc", false, "Sort ascending")

	// Create flags
	userCreateCmd.Flags().String("email", "", "User email address")
	userCreateCmd.Flags().String("first-name", "", "User first name")
	userCreateCmd.Flags().String("last-name", "", "User last name")
	userCreateCmd.Flags().String("metadata", "", "JSON metadata string")
	userCreateCmd.Flags().String("metadata-file", "", "Path to JSON metadata file")

	// Update flags
	userUpdateCmd.Flags().String("email", "", "Update email address")
	userUpdateCmd.Flags().String("first-name", "", "Update first name")
	userUpdateCmd.Flags().String("last-name", "", "Update last name")
	userUpdateCmd.Flags().String("metadata", "", "Update metadata (JSON)")
	userUpdateCmd.Flags().String("metadata-file", "", "Path to JSON metadata file")

	// Delete flags
	userDeleteCmd.Flags().Bool("force", false, "Skip confirmation prompt")

	// Threads flags
	userThreadsCmd.Flags().Int("page", 1, "Page number")
	userThreadsCmd.Flags().Int("page-size", 50, "Results per page")

	// ABAC dry-run flags
	for _, cmd := range []*cobra.Command{userEvaluateCmd, userExplainCmd} {
		cmd.Flags().String("action", "", "Action name to evaluate (e.g. graph.search)")
		cmd.Flags().String("graph", "", "Standalone graph UUID to evaluate as the resource")
		_ = cmd.MarkFlagRequired("action")
	}

	// The ABAC subcommands reach bearer-only endpoints; the rest of the
	// group uses the SDK with an API key.
	for _, cmd := range []*cobra.Command{userMembershipsCmd, userEvaluateCmd, userExplainCmd} {
		client.SetCredentialType(cmd, client.CredentialBearer)
	}
}
