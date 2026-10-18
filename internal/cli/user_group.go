package cli

import (
	"fmt"
	"strconv"

	"github.com/getzep/zep-go/v4"
	"github.com/spf13/cobra"

	"github.com/getzep/zepctl/internal/abac"
	"github.com/getzep/zepctl/internal/client"
	"github.com/getzep/zepctl/internal/output"
)

const userGroupLabel = "user group"

var userGroupCmd = &cobra.Command{
	Use:   "user-group",
	Short: "Manage UserGroups and their ABAC policy attachments",
	Long: "Manage UserGroups, their members, and the ABAC policy sets attached to them.\n\n" +
		"A project always has the virtual \"All Users\" group, which contains every active\n" +
		"user. It accepts policy attachments but cannot be renamed, deleted, or given\n" +
		"members. Its UUID is the first row of \"user-group list\".",
}

var userGroupListCmd = &cobra.Command{
	Use:   listCmdUse,
	Short: "List UserGroups in the current project",
	RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := client.NewWithCredential(cmd.Context(), client.CredentialBearer)
		if err != nil {
			return err
		}
		limit, _ := cmd.Flags().GetInt("limit")
		cursor, _ := cmd.Flags().GetString("cursor")
		search, _ := cmd.Flags().GetString("search")
		req := &zep.UserGroupListRequest{
			Body: &zep.SearchListRequest{Search: zep.String(search)},
		}
		if limit > 0 {
			req.PageSize = zep.Int(limit)
		}
		if cursor != "" {
			req.Cursor = zep.String(cursor)
		}
		page, err := c.UserGroup.List(cmd.Context(), req)
		if err != nil {
			return fmt.Errorf("listing user groups: %w", err)
		}
		if output.GetFormat() == output.FormatTable {
			if page.RawResponse.Next != nil && *page.RawResponse.Next != "" {
				output.Info("Next cursor: %s", *page.RawResponse.Next)
			}
			return renderUserGroupTable(page.Results)
		}
		return output.Print(map[string]any{itemsOutputKey: page.Results, nextCursorOutputKey: page.RawResponse.Next})
	},
}

var userGroupGetCmd = &cobra.Command{
	Use:   "get <group-uuid>",
	Short: "Get a UserGroup by UUID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateUUID(args[0], userGroupLabel); err != nil {
			return err
		}
		c, err := client.NewWithCredential(cmd.Context(), client.CredentialBearer)
		if err != nil {
			return err
		}
		group, err := c.UserGroup.Get(cmd.Context(), args[0])
		if err != nil {
			return fmt.Errorf("getting user group %s: %w", args[0], err)
		}
		if output.GetFormat() == output.FormatTable {
			renderUserGroup(cmd, group)
			return nil
		}
		return output.Print(group)
	},
}

var userGroupCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a managed UserGroup",
	RunE: func(cmd *cobra.Command, _ []string) error {
		name, _ := cmd.Flags().GetString("name")
		description, _ := cmd.Flags().GetString("description")
		c, err := client.NewWithCredential(cmd.Context(), client.CredentialBearer)
		if err != nil {
			return err
		}
		request := &zep.CreateUserGroupRequest{Name: name}
		if description != "" {
			request.Description = zep.String(description)
		}
		group, err := c.UserGroup.Create(cmd.Context(), request)
		if err != nil {
			return fmt.Errorf("creating user group: %w", err)
		}
		if output.GetFormat() == output.FormatTable {
			output.Info("Created user group %q (%s)", valueOrEmpty(group.Name), truncateUUID(valueOrEmpty(group.UUID)))
			return nil
		}
		return output.Print(group)
	},
}

var userGroupUpdateCmd = &cobra.Command{
	Use:   "update <group-uuid>",
	Short: "Update a managed UserGroup's name or description",
	Long: "Updates a managed UserGroup. Without --expected-version, the command reads and\n" +
		"sends the group's current version. A conflict occurs only if another update\n" +
		"happens between the read and the write. With --expected-version, a conflict\n" +
		"occurs if the group's version differs from that value.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateUUID(args[0], userGroupLabel); err != nil {
			return err
		}
		name, description := changedStringFlag(cmd, "name"), changedStringFlag(cmd, "description")
		if name == nil && description == nil {
			return fmt.Errorf("at least one of --name or --description is required")
		}
		c, err := client.NewWithCredential(cmd.Context(), client.CredentialBearer)
		if err != nil {
			return err
		}
		expectedVersion, err := resolveExpectedVersion(cmd, c, args[0])
		if err != nil {
			return err
		}
		request := &zep.PatchUserGroupRequest{
			ExpectedVersion: zep.Int(expectedVersion),
			Name:            name,
			Description:     description,
		}
		group, err := c.UserGroup.Update(cmd.Context(), args[0], request)
		if err != nil {
			return fmt.Errorf("updating user group %s: %w", args[0], err)
		}
		if output.GetFormat() == output.FormatTable {
			output.Info("Updated user group %q (version %s)", valueOrEmpty(group.Name), formatVersion(group.Version))
			return nil
		}
		return output.Print(group)
	},
}

var userGroupDeleteCmd = &cobra.Command{
	Use:   "delete <group-uuid>",
	Short: "Delete a managed UserGroup",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateUUID(args[0], userGroupLabel); err != nil {
			return err
		}
		confirmed, err := confirmDeletion(cmd,
			"Delete user group %s? This also removes its memberships and policy attachments.", args[0])
		if err != nil || !confirmed {
			return err
		}
		c, err := client.NewWithCredential(cmd.Context(), client.CredentialBearer)
		if err != nil {
			return err
		}
		if err := c.UserGroup.Delete(cmd.Context(), args[0]); err != nil {
			return fmt.Errorf("deleting user group %s: %w", args[0], err)
		}
		output.Info("Deleted user group %s", args[0])
		return nil
	},
}

// --- Members subgroup ---

var userGroupMembersCmd = &cobra.Command{
	Use:   "members",
	Short: "Manage UserGroup membership",
}

var userGroupMembersListCmd = &cobra.Command{
	Use:   "list <group-uuid>",
	Short: "List members of a UserGroup",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMemberList(cmd, args[0], false)
	},
}

var userGroupMembersCandidatesCmd = &cobra.Command{
	Use:   "candidates <group-uuid>",
	Short: "List users eligible to join a managed UserGroup",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMemberList(cmd, args[0], true)
	},
}

var userGroupMembersAddCmd = &cobra.Command{
	Use:   "add <group-uuid> <user-uuid>...",
	Short: "Add members to a managed UserGroup",
	Long:  "Adds up to 50 users per call. Users are identified by UUID, not by user ID.",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMemberMutation(cmd, args, true)
	},
}

var userGroupMembersRemoveCmd = &cobra.Command{
	Use:   "remove <group-uuid> <user-uuid>...",
	Short: "Remove members from a managed UserGroup",
	Long:  "Removes up to 50 users per call. Users are identified by UUID, not by user ID.",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMemberMutation(cmd, args, false)
	},
}

// --- Policy Sets subgroup ---

var userGroupPolicySetsCmd = &cobra.Command{
	Use:   "policy-sets",
	Short: "Manage policy set attachments for a UserGroup",
}

var userGroupPolicySetsListCmd = &cobra.Command{
	Use:   "list <group-uuid>",
	Short: "List policy sets attached to a UserGroup",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateUUID(args[0], userGroupLabel); err != nil {
			return err
		}
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		result, err := ac.ListUserGroupPolicySets(cmd.Context(), args[0])
		if err != nil {
			return userGroupError(err, "listing attached policy sets", args[0])
		}
		if output.GetFormat() == output.FormatTable {
			return renderPolicySetSummaries(result.PolicySets)
		}
		return output.Print(result)
	},
}

var userGroupPolicySetsAttachCmd = &cobra.Command{
	Use:   "attach <group-uuid> <policy-set-uuid>",
	Short: "Attach a policy set to a UserGroup",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateAttachmentUUIDs(args); err != nil {
			return err
		}
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		result, err := ac.AttachUserGroupPolicySet(cmd.Context(), args[0], args[1])
		if err != nil {
			return fmt.Errorf("attaching policy set: %w", err)
		}
		if output.GetFormat() == output.FormatTable {
			for _, ps := range result.PolicySets {
				if ps.UUID == args[1] {
					output.Info("Attached policy set %q to user group %s", ps.Name, args[0])
					return nil
				}
			}
			output.Info("Attached policy set to user group %s", args[0])
			return nil
		}
		return output.Print(result)
	},
}

var userGroupPolicySetsDetachCmd = &cobra.Command{
	Use:   "detach <group-uuid> <policy-set-uuid>",
	Short: "Detach a policy set from a UserGroup",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateAttachmentUUIDs(args); err != nil {
			return err
		}
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		result, err := ac.DetachUserGroupPolicySet(cmd.Context(), args[0], args[1])
		if err != nil {
			return fmt.Errorf("detaching policy set: %w", err)
		}
		if output.GetFormat() == output.FormatTable {
			output.Info("Detached policy set from user group %s", args[0])
			return nil
		}
		return output.Print(result)
	},
}

// --- Dry-run subcommands ---

var userGroupEvaluateCmd = &cobra.Command{
	Use:   "evaluate <group-uuid>",
	Short: "Dry-run a policy decision for a UserGroup principal",
	Long: "Asks the server what the evaluator would decide for the given action and the\n" +
		"policy sets attached to this UserGroup. The base posture is default-deny.\n" +
		"Pass --graph to evaluate a specific standalone graph resource. Always exits 0\n" +
		"on a successful API call regardless of allow/deny outcome.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateUUID(args[0], userGroupLabel); err != nil {
			return err
		}
		return runEvaluate(cmd, abac.Principal{Type: abac.PrincipalUserGroup, UUID: args[0]}, userGroupLabel)
	},
}

var userGroupExplainCmd = &cobra.Command{
	Use:   "explain <group-uuid>",
	Short: "Dry-run a policy decision for a UserGroup principal and return the trace",
	Long: "Like evaluate, but returns the full evaluator trace (matched and skipped\n" +
		"policies, set modes, resource matches, object constraints) for human\n" +
		"inspection. Always exits 0 on a successful API call.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateUUID(args[0], userGroupLabel); err != nil {
			return err
		}
		return runExplain(cmd, abac.Principal{Type: abac.PrincipalUserGroup, UUID: args[0]}, userGroupLabel)
	},
}

// --- Helpers ---

func runMemberList(cmd *cobra.Command, groupUUID string, candidates bool) error {
	if err := validateUUID(groupUUID, userGroupLabel); err != nil {
		return err
	}
	c, err := client.NewWithCredential(cmd.Context(), client.CredentialBearer)
	if err != nil {
		return err
	}
	limit, _ := cmd.Flags().GetInt("limit")
	cursor, _ := cmd.Flags().GetString("cursor")
	search, _ := cmd.Flags().GetString("search")
	body := &zep.SearchListRequest{Search: zep.String(search)}
	action := "listing members"
	var results []*zep.User
	var nextCursor *string
	if candidates {
		action = "listing member candidates"
		request := &zep.UserGroupListMemberCandidatesRequest{Body: body}
		if limit > 0 {
			request.PageSize = zep.Int(limit)
		}
		if cursor != "" {
			request.Cursor = zep.String(cursor)
		}
		page, err := c.UserGroup.ListMemberCandidates(cmd.Context(), groupUUID, request)
		if err != nil {
			return fmt.Errorf("%s for user group %s: %w", action, groupUUID, err)
		}
		results, nextCursor = page.Results, page.RawResponse.Next
	} else {
		request := &zep.UserGroupListMembersRequest{Body: body}
		if limit > 0 {
			request.PageSize = zep.Int(limit)
		}
		if cursor != "" {
			request.Cursor = zep.String(cursor)
		}
		page, err := c.UserGroup.ListMembers(cmd.Context(), groupUUID, request)
		if err != nil {
			return fmt.Errorf("%s for user group %s: %w", action, groupUUID, err)
		}
		results, nextCursor = page.Results, page.RawResponse.Next
	}
	if output.GetFormat() == output.FormatTable {
		tbl := output.NewTable("UUID", "USER ID", "EMAIL", "FIRST NAME", "LAST NAME")
		tbl.WriteHeader()
		for _, u := range results {
			tbl.WriteRow(valueOrEmpty(u.UUID), valueOrEmpty(u.UserID), valueOrEmpty(u.Email),
				valueOrEmpty(u.FirstName), valueOrEmpty(u.LastName))
		}
		if nextCursor != nil && *nextCursor != "" {
			output.Info("Next cursor: %s", *nextCursor)
		}
		return tbl.Flush()
	}
	return output.Print(map[string]any{itemsOutputKey: results, nextCursorOutputKey: nextCursor})
}

func runMemberMutation(cmd *cobra.Command, args []string, add bool) error {
	if err := validateUUID(args[0], userGroupLabel); err != nil {
		return err
	}
	userUUIDs := args[1:]
	for _, userUUID := range userUUIDs {
		if err := validateUUID(userUUID, "user"); err != nil {
			return err
		}
	}
	c, err := client.NewWithCredential(cmd.Context(), client.CredentialBearer)
	if err != nil {
		return err
	}
	request := &zep.MutateMembersRequest{UserUUIDs: userUUIDs}
	var result *zep.MembershipMutationResult
	if add {
		result, err = c.UserGroup.AddMembers(cmd.Context(), args[0], request)
	} else {
		result, err = c.UserGroup.RemoveMembers(cmd.Context(), args[0], request)
	}
	if err != nil {
		if add {
			return fmt.Errorf("adding members: %w", err)
		}
		return fmt.Errorf("removing members: %w", err)
	}
	if output.GetFormat() == output.FormatTable {
		if add {
			output.Info("Added %d member(s) to user group %s (%d already a member)",
				intOrZero(result.AddedCount), args[0], intOrZero(result.NoOpCount))
		} else {
			output.Info("Removed %d member(s) from user group %s (%d not a member)",
				intOrZero(result.RemovedCount), args[0], intOrZero(result.NoOpCount))
		}
		return nil
	}
	return output.Print(result)
}

// resolveExpectedVersion returns the --expected-version value, or the
// group's current version when the flag is absent. A group without a
// version is the virtual All Users group, which cannot be updated.
func resolveExpectedVersion(cmd *cobra.Command, c *client.Client, groupUUID string) (int, error) {
	if cmd.Flags().Changed("expected-version") {
		expectedVersion, _ := cmd.Flags().GetInt("expected-version")
		return expectedVersion, nil
	}
	group, err := c.UserGroup.Get(cmd.Context(), groupUUID)
	if err != nil {
		return 0, fmt.Errorf("reading current version of user group %s: %w", groupUUID, err)
	}
	if group.Version == nil {
		return 0, fmt.Errorf("user group %s is not updatable: the All Users group is immutable", groupUUID)
	}
	return *group.Version, nil
}

func validateAttachmentUUIDs(args []string) error {
	if err := validateUUID(args[0], userGroupLabel); err != nil {
		return err
	}
	return validateUUID(args[1], "policy set")
}

func addPaginationFlags(cmd *cobra.Command, searchUsage string) {
	cmd.Flags().Int("limit", 50, "Maximum number of results")
	cmd.Flags().String("cursor", "", "Cursor from the previous page")
	cmd.Flags().String("search", "", searchUsage)
}

// changedStringFlag returns a pointer to the flag value only when the
// user set it, so an unset flag is omitted from a PATCH body and an
// explicit empty value clears the field.
func changedStringFlag(cmd *cobra.Command, name string) *string {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	value, _ := cmd.Flags().GetString(name)
	return &value
}

// formatVersion renders a version, which is null for the virtual
// All Users group.
func formatVersion(version *int) string {
	if version == nil {
		return "-"
	}
	return strconv.Itoa(*version)
}

func renderUserGroupTable(groups []*zep.UserGroup) error {
	tbl := output.NewTable("UUID", "NAME", "KIND", "VERSION")
	tbl.WriteHeader()
	for _, g := range groups {
		kind := "managed"
		if g.Version == nil {
			kind = "all_users"
		}
		tbl.WriteRow(valueOrEmpty(g.UUID), valueOrEmpty(g.Name), kind, formatVersion(g.Version))
	}
	return tbl.Flush()
}

func renderUserGroup(cmd *cobra.Command, group *zep.UserGroup) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "UUID:          %s\n", valueOrEmpty(group.UUID))
	fmt.Fprintf(out, "Name:          %s\n", valueOrEmpty(group.Name))
	if group.Description != nil {
		fmt.Fprintf(out, "Description:   %s\n", *group.Description)
	}
	kind := "managed"
	if group.Version == nil {
		kind = "all_users"
	}
	fmt.Fprintf(out, "Kind:          %s\n", kind)
	fmt.Fprintf(out, "Version:       %s\n", formatVersion(group.Version))
	fmt.Fprintf(out, "Created:       %s\n", formatTimestamp(group.CreatedAt))
	fmt.Fprintf(out, "Updated:       %s\n", formatTimestamp(group.UpdatedAt))
}

func formatTimestamp(value *string) string { return valueOrEmpty(value) }

func intOrZero(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func renderPolicySetSummaries(summaries []abac.PolicySetSummary) error {
	tbl := output.NewTable("UUID", "NAME", "MODE", "VERSION")
	tbl.WriteHeader()
	for _, ps := range summaries {
		tbl.WriteRow(ps.UUID, ps.Name, ps.Mode, strconv.Itoa(ps.Version))
	}
	return tbl.Flush()
}

// userGroupError maps a 404 to a resource-specific message and otherwise
// wraps the server error with the attempted action.
func userGroupError(err error, action, groupUUID string) error {
	if abac.IsNotFound(err) {
		return fmt.Errorf("user group not found: %s", groupUUID)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func init() {
	rootCmd.AddCommand(userGroupCmd)
	userGroupCmd.AddCommand(userGroupListCmd)
	userGroupCmd.AddCommand(userGroupGetCmd)
	userGroupCmd.AddCommand(userGroupCreateCmd)
	userGroupCmd.AddCommand(userGroupUpdateCmd)
	userGroupCmd.AddCommand(userGroupDeleteCmd)
	userGroupCmd.AddCommand(userGroupMembersCmd)
	userGroupCmd.AddCommand(userGroupPolicySetsCmd)
	userGroupCmd.AddCommand(userGroupEvaluateCmd)
	userGroupCmd.AddCommand(userGroupExplainCmd)

	userGroupMembersCmd.AddCommand(userGroupMembersListCmd)
	userGroupMembersCmd.AddCommand(userGroupMembersCandidatesCmd)
	userGroupMembersCmd.AddCommand(userGroupMembersAddCmd)
	userGroupMembersCmd.AddCommand(userGroupMembersRemoveCmd)

	userGroupPolicySetsCmd.AddCommand(userGroupPolicySetsListCmd)
	userGroupPolicySetsCmd.AddCommand(userGroupPolicySetsAttachCmd)
	userGroupPolicySetsCmd.AddCommand(userGroupPolicySetsDetachCmd)

	addPaginationFlags(userGroupListCmd, "Search by group name")
	addPaginationFlags(userGroupMembersListCmd, "Search by user_id, name, or email")
	addPaginationFlags(userGroupMembersCandidatesCmd, "Search by user_id, name, or email")

	userGroupCreateCmd.Flags().String("name", "", "Group name")
	userGroupCreateCmd.Flags().String("description", "", "Group description")
	_ = userGroupCreateCmd.MarkFlagRequired("name")

	userGroupUpdateCmd.Flags().String("name", "", "New group name")
	userGroupUpdateCmd.Flags().String("description", "", "New group description")
	userGroupUpdateCmd.Flags().Int("expected-version", 0,
		"Expected group version; the update conflicts if the current version differs. Without this flag, the command reads and sends the current version and conflicts only if another update happens between the read and the write")

	userGroupDeleteCmd.Flags().Bool("force", false, "Skip confirmation prompt")

	for _, cmd := range []*cobra.Command{userGroupEvaluateCmd, userGroupExplainCmd} {
		cmd.Flags().String("action", "", "Action name to evaluate (e.g. graph.search)")
		cmd.Flags().String("graph", "", "Standalone graph UUID to evaluate as the resource")
		_ = cmd.MarkFlagRequired("action")
	}

	for _, cmd := range []*cobra.Command{
		userGroupListCmd, userGroupGetCmd, userGroupCreateCmd, userGroupUpdateCmd, userGroupDeleteCmd,
		userGroupMembersListCmd, userGroupMembersCandidatesCmd, userGroupMembersAddCmd, userGroupMembersRemoveCmd,
		userGroupPolicySetsListCmd, userGroupPolicySetsAttachCmd, userGroupPolicySetsDetachCmd,
		userGroupEvaluateCmd, userGroupExplainCmd,
	} {
		client.SetCredentialType(cmd, client.CredentialBearer)
	}
}
