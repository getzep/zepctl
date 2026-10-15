package cli

import (
	"fmt"
	"strconv"

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
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		page, pageSize, search := paginationFlags(cmd)
		result, err := ac.ListUserGroups(cmd.Context(), page, pageSize, search)
		if err != nil {
			return fmt.Errorf("listing user groups: %w", err)
		}
		if output.GetFormat() == output.FormatTable {
			return renderUserGroupTable(result.UserGroups)
		}
		return output.Print(result)
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
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		group, err := ac.GetUserGroup(cmd.Context(), args[0])
		if err != nil {
			return userGroupError(err, "getting user group", args[0])
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
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		group, err := ac.CreateUserGroup(cmd.Context(), name, description)
		if err != nil {
			return fmt.Errorf("creating user group: %w", err)
		}
		if output.GetFormat() == output.FormatTable {
			output.Info("Created user group %q (%s)", group.Name, truncateUUID(group.UUID))
			return nil
		}
		return output.Print(group)
	},
}

var userGroupUpdateCmd = &cobra.Command{
	Use:   "update <group-uuid>",
	Short: "Update a managed UserGroup's name or description",
	Long: "Updates a managed UserGroup. Without --expected-version the command reads the\n" +
		"group's current version first and sends that, which is last-writer-wins. Pass\n" +
		"--expected-version to fail with a conflict when someone else has updated the\n" +
		"group since you read it.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateUUID(args[0], userGroupLabel); err != nil {
			return err
		}
		name, description := changedStringFlag(cmd, "name"), changedStringFlag(cmd, "description")
		if name == nil && description == nil {
			return fmt.Errorf("at least one of --name or --description is required")
		}
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		expectedVersion, err := resolveExpectedVersion(cmd, ac, args[0])
		if err != nil {
			return err
		}
		group, err := ac.UpdateUserGroup(cmd.Context(), args[0], name, description, expectedVersion)
		if err != nil {
			return userGroupError(err, "updating user group", args[0])
		}
		if output.GetFormat() == output.FormatTable {
			output.Info("Updated user group %q (version %s)", group.Name, formatVersion(group.Version))
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
		ac, err := newABACClient(cmd)
		if err != nil {
			return err
		}
		if err := ac.DeleteUserGroup(cmd.Context(), args[0]); err != nil {
			return userGroupError(err, "deleting user group", args[0])
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
	ac, err := newABACClient(cmd)
	if err != nil {
		return err
	}
	page, pageSize, search := paginationFlags(cmd)
	action := "listing members"
	list := ac.ListUserGroupMembers
	if candidates {
		action = "listing member candidates"
		list = ac.ListUserGroupMemberCandidates
	}
	result, err := list(cmd.Context(), groupUUID, page, pageSize, search)
	if err != nil {
		return userGroupError(err, action, groupUUID)
	}
	if output.GetFormat() == output.FormatTable {
		tbl := output.NewTable("UUID", "USER ID", "EMAIL", "FIRST NAME", "LAST NAME")
		tbl.WriteHeader()
		for _, u := range result.Users {
			tbl.WriteRow(u.UUID, u.UserID, u.Email, u.FirstName, u.LastName)
		}
		return tbl.Flush()
	}
	return output.Print(result)
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
	ac, err := newABACClient(cmd)
	if err != nil {
		return err
	}
	var result *abac.MembershipResult
	if add {
		result, err = ac.AddUserGroupMembers(cmd.Context(), args[0], userUUIDs)
	} else {
		result, err = ac.RemoveUserGroupMembers(cmd.Context(), args[0], userUUIDs)
	}
	if err != nil {
		// The group and every supplied user share one 404, so the
		// server's message has to name the object that was not found.
		if add {
			return fmt.Errorf("adding members: %w", err)
		}
		return fmt.Errorf("removing members: %w", err)
	}
	if output.GetFormat() == output.FormatTable {
		if add {
			output.Info("Added %d member(s) to user group %s (%d already a member)",
				result.Added, args[0], result.NoOp)
		} else {
			output.Info("Removed %d member(s) from user group %s (%d not a member)",
				result.Removed, args[0], result.NoOp)
		}
		return nil
	}
	return output.Print(result)
}

// resolveExpectedVersion returns the --expected-version value, or the
// group's current version when the flag is absent. A group without a
// version is the virtual All Users group, which cannot be updated.
func resolveExpectedVersion(cmd *cobra.Command, ac *abac.Client, groupUUID string) (int, error) {
	if cmd.Flags().Changed("expected-version") {
		expectedVersion, _ := cmd.Flags().GetInt("expected-version")
		return expectedVersion, nil
	}
	group, err := ac.GetUserGroup(cmd.Context(), groupUUID)
	if err != nil {
		return 0, userGroupError(err, "reading current version of user group", groupUUID)
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
	cmd.Flags().Int("page", 1, "Page number")
	cmd.Flags().Int("page-size", 50, "Results per page")
	cmd.Flags().String("search", "", searchUsage)
}

// paginationFlags reads the shared list flags. pageSize is always sent
// because the API requires it.
func paginationFlags(cmd *cobra.Command) (page, pageSize int, search string) {
	page, _ = cmd.Flags().GetInt("page")
	pageSize, _ = cmd.Flags().GetInt("page-size")
	search, _ = cmd.Flags().GetString("search")
	return page, pageSize, search
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

func renderUserGroupTable(groups []abac.UserGroup) error {
	tbl := output.NewTable("UUID", "NAME", "KIND", "MEMBERS", "POLICY SETS", "VERSION")
	tbl.WriteHeader()
	for _, g := range groups {
		tbl.WriteRow(g.UUID, g.Name, g.Kind,
			strconv.Itoa(g.MemberCount), strconv.Itoa(g.AttachedPolicyCount), formatVersion(g.Version))
	}
	return tbl.Flush()
}

func renderUserGroup(cmd *cobra.Command, group *abac.UserGroup) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "UUID:          %s\n", group.UUID)
	fmt.Fprintf(out, "Name:          %s\n", group.Name)
	if group.Description != "" {
		fmt.Fprintf(out, "Description:   %s\n", group.Description)
	}
	fmt.Fprintf(out, "Kind:          %s\n", group.Kind)
	fmt.Fprintf(out, "Members:       %d\n", group.MemberCount)
	fmt.Fprintf(out, "Policy sets:   %d\n", group.AttachedPolicyCount)
	fmt.Fprintf(out, "Version:       %s\n", formatVersion(group.Version))
	fmt.Fprintf(out, "Project:       %s\n", group.ProjectUUID)
	fmt.Fprintf(out, "Created:       %s\n", formatTimestamp(group.CreatedAt))
	fmt.Fprintf(out, "Updated:       %s\n", formatTimestamp(group.UpdatedAt))
}

func formatTimestamp(value *string) string {
	if value == nil {
		return "-"
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
		"Version the update expects; defaults to the group's current version")

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
