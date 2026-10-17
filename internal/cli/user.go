package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/getzep/zep-go/v4"
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

		limit, _ := cmd.Flags().GetInt("limit")
		cursor, _ := cmd.Flags().GetString("cursor")
		search, _ := cmd.Flags().GetString("search")
		orderBy, _ := cmd.Flags().GetString("order-by")
		asc, _ := cmd.Flags().GetBool("asc")

		req := &zep.UserListRequest{}
		if limit > 0 {
			req.PageSize = zep.Int(limit)
		}
		if cursor != "" {
			req.Cursor = zep.String(cursor)
		}
		if search != "" {
			req.Search = zep.String(search)
		}
		if orderBy != "" {
			value := zep.UserListRequestOrderBy(orderBy)
			req.OrderBy = &value
		}
		if cmd.Flags().Changed("asc") {
			order := zep.UserListRequestOrder("desc")
			if asc {
				order = "asc"
			}
			req.Order = &order
		}

		users, err := c.User.List(context.Background(), req)
		if err != nil {
			return fmt.Errorf("listing users: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("UUID", "USER ID", "EMAIL", "FIRST NAME", "LAST NAME", "CREATED AT")
			tbl.WriteHeader()
			for _, u := range users.Results {
				uuid := ""
				if u.UUID != nil {
					uuid = *u.UUID
				}
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
				tbl.WriteRow(uuid, userID, email, firstName, lastName, createdAt)
			}
			writeNextCursor(users.RawResponse.Next)
			return tbl.Flush()
		}

		return output.Print(map[string]any{itemsOutputKey: users.Results, "next_cursor": users.RawResponse.Next})
	},
}

var userGetCmd = &cobra.Command{
	Use:   "get <user-uuid>",
	Short: "Get user details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		userUUID := args[0]

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		user, err := c.User.Get(context.Background(), userUUID)
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
			if user.UUID != nil {
				tbl.WriteRow("UUID", *user.UUID)
			}
			if user.UserID != nil {
				tbl.WriteRow("User ID", userIDStr)
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
	Use:   "create",
	Short: "Create a new user",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		email, _ := cmd.Flags().GetString("email")
		firstName, _ := cmd.Flags().GetString("first-name")
		lastName, _ := cmd.Flags().GetString("last-name")
		userID, _ := cmd.Flags().GetString("user-id")
		metadataStr, _ := cmd.Flags().GetString("metadata")
		metadataFile, _ := cmd.Flags().GetString("metadata-file")

		req := &zep.CreateUserRequest{}
		if userID != "" {
			req.UserID = zep.String(userID)
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

		user, err := c.User.Create(context.Background(), req)
		if err != nil {
			return fmt.Errorf("creating user: %w", err)
		}

		if user.UUID != nil {
			output.Info("Created user %s", *user.UUID)
		}
		return output.Print(user)
	},
}

var userUpdateCmd = &cobra.Command{
	Use:   "update <user-uuid>",
	Short: "Update an existing user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		userUUID := args[0]

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		email, _ := cmd.Flags().GetString("email")
		firstName, _ := cmd.Flags().GetString("first-name")
		lastName, _ := cmd.Flags().GetString("last-name")
		metadataStr, _ := cmd.Flags().GetString("metadata")
		metadataFile, _ := cmd.Flags().GetString("metadata-file")

		req := &zep.PatchUserRequest{}

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

		user, err := c.User.Update(context.Background(), userUUID, req)
		if err != nil {
			return fmt.Errorf("updating user: %w", err)
		}

		output.Info("Updated user %q", userUUID)
		return output.Print(user)
	},
}

var userDeleteCmd = &cobra.Command{
	Use:   "delete <user-uuid>",
	Short: "Delete a user",
	Long:  `Delete a user and all associated data (threads, graph, knowledge). Supports RTBF compliance.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		userUUID := args[0]
		force, _ := cmd.Flags().GetBool("force")

		if !force {
			fmt.Printf("Delete user %q and all associated data? This cannot be undone. [y/N]: ", userUUID)
			if !confirmYesNo() {
				output.Info("Aborted")
				return nil
			}
		}

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		result, err := c.User.Delete(context.Background(), userUUID)
		if err != nil {
			return fmt.Errorf("deleting user: %w", err)
		}

		wait, _ := cmd.Flags().GetBool("wait")
		timeout, _ := cmd.Flags().GetDuration("timeout")
		return reportDeletion(c, "user", userUUID, result.Task, wait, timeout)
	},
}

var userThreadsCmd = &cobra.Command{
	Use:   "threads <user-uuid>",
	Short: "List user threads",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		userUUID := args[0]

		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}

		limit, _ := cmd.Flags().GetInt("limit")
		cursor, _ := cmd.Flags().GetString("cursor")
		req := &zep.ThreadListRequest{UserUUID: zep.String(userUUID)}
		if limit > 0 {
			req.PageSize = zep.Int(limit)
		}
		if cursor != "" {
			req.Cursor = zep.String(cursor)
		}
		threads, err := c.Thread.List(context.Background(), req)
		if err != nil {
			return fmt.Errorf("getting user threads: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			tbl := output.NewTable("UUID", "THREAD ID", "CREATED AT")
			tbl.WriteHeader()
			for _, t := range threads.Results {
				createdAt := ""
				if t.CreatedAt != nil {
					createdAt = *t.CreatedAt
				}
				threadID := ""
				if t.ThreadID != nil {
					threadID = *t.ThreadID
				}
				uuid := ""
				if t.UUID != nil {
					uuid = *t.UUID
				}
				tbl.WriteRow(uuid, threadID, createdAt)
			}
			writeNextCursor(threads.RawResponse.Next)
			return tbl.Flush()
		}

		return output.Print(map[string]any{itemsOutputKey: threads.Results, "next_cursor": threads.RawResponse.Next})
	},
}

var userNodeCmd = &cobra.Command{
	Use:   "node <user-uuid>",
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
		c, err := client.NewForCommand(cmd)
		if err != nil {
			return err
		}
		limit, _ := cmd.Flags().GetInt("limit")
		cursor, _ := cmd.Flags().GetString("cursor")
		request := &zep.UserGroupListForUserRequest{}
		if limit > 0 {
			request.PageSize = zep.Int(limit)
		}
		if cursor != "" {
			request.Cursor = zep.String(cursor)
		}
		page, err := c.UserGroup.ListForUser(cmd.Context(), args[0], request)
		if err != nil {
			return fmt.Errorf("listing user groups: %w", err)
		}
		if output.GetFormat() == output.FormatTable {
			if page.RawResponse.Next != nil && *page.RawResponse.Next != "" {
				output.Info("Next cursor: %s", *page.RawResponse.Next)
			}
			return renderUserGroupTable(page.Results)
		}
		return output.Print(map[string]any{itemsOutputKey: page.Results, "next_cursor": page.RawResponse.Next})
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
	userListCmd.Flags().Int("limit", 50, "Maximum number of results")
	userListCmd.Flags().String("cursor", "", "Cursor from the previous page")
	userListCmd.Flags().String("search", "", "Search by user_id, name, or email")
	userListCmd.Flags().String("order-by", "", "Sort by column (created_at, user_id, email)")
	userListCmd.Flags().Bool("asc", false, "Sort ascending")

	// Create flags
	userCreateCmd.Flags().String("email", "", "User email address")
	userCreateCmd.Flags().String("first-name", "", "User first name")
	userCreateCmd.Flags().String("last-name", "", "User last name")
	userCreateCmd.Flags().String("user-id", "", "External identity-provider user ID")
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
	userDeleteCmd.Flags().Bool("wait", false, "Wait for asynchronous deletion")
	userDeleteCmd.Flags().Duration("timeout", defaultTaskTimeout, "Maximum time to wait")

	// Threads flags
	userThreadsCmd.Flags().Int("limit", 50, "Maximum number of results")
	userThreadsCmd.Flags().String("cursor", "", "Cursor from the previous page")

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
	userMembershipsCmd.Flags().Int("limit", 50, "Maximum number of results")
	userMembershipsCmd.Flags().String("cursor", "", "Cursor from the previous page")
}
