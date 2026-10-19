# zepctl CLI Specification

## Overview

`zepctl` is a command-line interface for administering Zep projects and improving the developer experience. It provides comprehensive access to Zep's context engineering platform, enabling developers to manage users, threads, knowledge graphs, and data operations from the terminal.

## Design Principles

1. **Consistent Command Structure**: Follow `kubectl`-style conventions with `<verb> <resource> [flags]`
2. **Output Flexibility**: Support multiple output formats (table, JSON, YAML)
3. **Scriptability**: All commands return appropriate exit codes and support machine-readable output
4. **Progressive Disclosure**: Simple commands for common tasks, advanced flags for power users
5. **Safety First**: Destructive operations require confirmation unless `--force` is specified

## Authentication & Configuration

zepctl supports two authentication modes:

- **API key** (long-lived): the original mode, used for headless / CI scenarios. Pass via `ZEP_API_KEY` or store per-profile in the system keychain.
- **Bearer token** (OAuth / Kinde): obtained interactively via `zepctl auth login`. Required for the ABAC management commands (`policy-set`, `api-key`, `user-group`, and `user groups|evaluate|explain`) and the `config set-project` interactive flow. Refresh tokens are stored in the system keychain.

Both modes coexist on the same profile. Commands that require bearer auth use the bearer token even if an API key is also present.

### Configuration File

Location: `~/.zepctl/config.yaml`

```yaml
current-profile: production
profiles:
  - name: production
    # API keys are stored securely in the system keychain
  - name: development
    api-url: https://api.example.com
    # Optional per-profile OAuth overrides; otherwise build-time defaults apply.
    oauth-issuer: https://your-tenant.kinde.com
    oauth-client-id: <client-id>
    oauth-audience: <audience>
    project-uuid: <project-uuid>
    account-uuid: <account-uuid>
environments:
  # Named presets that can be applied to profiles via `--env`.
  - name: dev
    api-url: https://api.example.com
    oauth-issuer: https://your-tenant.kinde.com
    oauth-client-id: <client-id>
    oauth-audience: <audience>
defaults:
  output: table
```

**Credential Storage**: API keys and OAuth refresh tokens are stored in the system keychain (macOS Keychain, Windows Credential Manager, or Linux Secret Service) rather than in the config file. For CI/CD environments without keychain access, use the `ZEP_API_KEY` environment variable.

### Environment Variables

| Variable | Description |
|----------|-------------|
| `ZEP_API_KEY` | API key for authentication |
| `ZEP_API_URL` | API endpoint URL (default: `https://api.getzep.com`) |
| `ZEP_PROFILE` | Override current profile |
| `ZEP_OUTPUT` | Default output format |
| `ZEP_PROJECT` | Override active project UUID |

### Configuration Commands

```bash
# Profiles
zepctl config view                      # Display current configuration
zepctl config get-profiles              # List all profiles
zepctl config use-profile <name>        # Switch active profile
zepctl config add-profile <name>        # Add a new profile (prompts for API key)
zepctl config update-profile [name]     # Update fields on an existing profile
zepctl config delete-profile <name>     # Remove a profile
zepctl config set-project [uuid]        # Set the active project (interactive if UUID omitted)

# Environment presets (reusable api-url + OAuth settings)
zepctl config get-environments          # List all environment presets
zepctl config add-environment <name>    # Add a named preset
zepctl config update-environment <name> # Update fields on a preset
zepctl config delete-environment <name> # Remove a preset
```

#### Profile Flags

`add-profile` and `update-profile` accept the same field flags. Only flags explicitly passed are applied; omitted flags leave existing values untouched (on update). Pass an empty string to clear a field on update.

| Flag | Description |
|------|-------------|
| `--api-key` | API key (stored in system keychain) |
| `--api-url` | API endpoint URL |
| `--env` | Apply a named environment preset (replaces `api-url`/OAuth fields). Per-field flags override the preset. |
| `--oauth-issuer` | OIDC issuer override for `auth login` |
| `--oauth-client-id` | OAuth client ID override for `auth login` |
| `--oauth-audience` | OAuth audience for the bearer token `aud` claim |
| `--project` | Project UUID (update only) |
| `--account` | Account UUID (update only) |
| `--no-api-key` | (add only) Create a bearer-only profile with no API key, skipping the prompt |

#### Environment Preset Flags

Environments are reusable bundles of endpoint + OAuth settings. Profiles can adopt them via `--env`.

| Flag | Description |
|------|-------------|
| `--api-url` | API URL for the environment |
| `--oauth-issuer` | OIDC issuer |
| `--oauth-client-id` | OAuth client ID |
| `--oauth-audience` | OAuth audience for bearer token `aud` claim |
| `--force` | (delete only) Skip confirmation prompt |

## Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--api-key` | `-k` | Override API key |
| `--api-url` | | Override API URL |
| `--profile` | `-p` | Use specific profile |
| `--project` | | Override active project UUID for this command |
| `--output` | `-o` | Output format: `table`, `json`, `yaml`, `wide` |
| `--quiet` | `-q` | Suppress non-essential output |
| `--verbose` | `-v` | Enable verbose output |
| `--config` | | Path to config file (default: `$HOME/.zepctl/config.yaml`) |
| `--help` | `-h` | Display help |
| `--version` | | Display version |

---

## Command Reference

### Auth Commands

Manage bearer-token authentication for the current profile.

```bash
zepctl auth login [--no-browser] [--env <name>]
zepctl auth logout
zepctl auth status
```

- `auth login` opens a browser window for interactive OAuth authentication and stores the resulting refresh + access tokens in the system keychain. If no profile exists yet, one is created using `--profile`/`default` and (optionally) the named environment preset.
- `--no-browser` prints the authorization URL instead of opening a browser (useful for SSH / headless sessions).
- `auth login` auto-selects a project after authentication: if the account has exactly one project, it is selected automatically; otherwise the CLI prompts for a choice.
- `auth logout` revokes the refresh token at the OAuth provider (best-effort) and clears the bearer token from the keychain.
- `auth status` displays the active profile's API URL, OIDC issuer, masked API key (if any), and bearer-token expiration.

**Example**:
```bash
# Bootstrap an isolated dev profile with browser-based auth
zepctl --profile dev auth login --env dev
zepctl auth status
```

---

### Project Commands

```bash
zepctl project get                     # Get current project info
```

**Output Fields**: `uuid`, `name`, `created_at`, `updated_at`

---

### User Commands

#### List Users

```bash
zepctl user list [flags]
```

| Flag | Description |
|------|-------------|
| `--limit` | Maximum number of results (default: 50) |
| `--cursor` | Opaque cursor from the previous page |

#### Get User

```bash
zepctl user get <user-uuid>
```

**Output Fields**: `user_id`, `uuid`, `email`, `first_name`, `last_name`, `created_at`, `metadata`

#### Create User

```bash
zepctl user create [flags]
```

| Flag | Description |
|------|-------------|
| `--email` | User email address |
| `--first-name` | User first name |
| `--last-name` | User last name |
| `--user-id` | External identity-provider ID |
| `--metadata` | JSON metadata string |
| `--metadata-file` | Path to JSON metadata file |

#### Update User

```bash
zepctl user update <user-uuid> [flags]
```

| Flag | Description |
|------|-------------|
| `--email` | Update email address |
| `--first-name` | Update first name |
| `--last-name` | Update last name |
| `--metadata` | Update metadata (JSON) |
| `--metadata-file` | Path to JSON metadata file |

#### Delete User

```bash
zepctl user delete <user-uuid> [flags]
```

| Flag | Description |
|------|-------------|
| `--force` | Skip confirmation prompt |
| `--wait` | Wait for the deletion task |
| `--timeout` | Maximum wait time (default: 5m) |

**Note**: Deleting a user removes all associated threads, graph data, and knowledge. Supports RTBF compliance.

#### List User Threads

```bash
zepctl user threads <user-uuid> [flags]
```

| Flag | Description |
|------|-------------|
| `--limit` | Maximum number of results (default: 50) |
| `--cursor` | Opaque cursor from the previous page |

#### Get User Graph Node

```bash
zepctl user node <user-uuid>
```

#### List Effective UserGroups

```bash
zepctl user groups <user-uuid>
```

Lists the user's effective UserGroups: the virtual **All Users** group plus every active managed membership. Requires a bearer token and takes a user UUID, not a user ID.

#### Dry-Run an ABAC Decision for a User

```bash
zepctl user evaluate <user-uuid> --action <name> [--graph <graph-uuid>]
zepctl user explain <user-uuid> --action <name> [--graph <graph-uuid>]
```

| Flag | Description |
|------|-------------|
| `--action` | Required. Action name to evaluate (e.g. `graph.search`) |
| `--graph` | Standalone graph UUID to evaluate as the resource |

These evaluate the policy sets attached to the user's effective UserGroups against a default-deny base; a deny from any group wins. Both require a bearer token, take a user UUID, and always exit 0 on a successful API call. See the UserGroup ABAC commands below for the output format.

---

### Thread Commands

#### Create Thread

```bash
zepctl thread create --user <user-uuid> [flags]
```

| Flag | Description |
|------|-------------|
| `--user` | User UUID (required) |
| `--metadata` | JSON metadata string |
| `--metadata-file` | Path to JSON metadata file |

#### Get Thread

```bash
zepctl thread get <thread-uuid>
```

#### Delete Thread

```bash
zepctl thread delete <thread-uuid> [flags]
```

| Flag | Description |
|------|-------------|
| `--force` | Skip confirmation prompt |
| `--wait` | Wait for the deletion task |
| `--timeout` | Maximum wait time (default: 5m) |

#### List Thread Messages

```bash
zepctl thread messages <thread-uuid> [flags]
```

| Flag | Description |
|------|-------------|
| `--limit` | Maximum number of results (default: 50) |
| `--cursor` | Opaque cursor from the previous page |

#### Add Messages to Thread

```bash
zepctl thread add-messages <thread-uuid> [flags]
```

| Flag | Description |
|------|-------------|
| `--file` | Path to JSON file containing messages |
| `--stdin` | Read messages from stdin |
| `--batch` | Use batch processing for large imports |
| `--wait` | Wait for batch processing to complete |

**Message Format (JSON)**:
```json
{
  "messages": [
    {
      "role": "user",
      "name": "Alice",
      "content": "Hello, I need help with my account",
      "reference_time": "2025-01-01T12:00:00Z"
    },
    {
      "role": "assistant",
      "content": "I'd be happy to help!"
    }
  ]
}
```

#### Get Thread Context

```bash
zepctl thread context <thread-uuid>
```

Returns relevant context from the user graph based on recent thread messages.

---

### Graph Commands

#### List Graphs

```bash
zepctl graph list [flags]
```

| Flag | Description |
|------|-------------|
| `--limit` | Maximum number of results (default: 50) |
| `--cursor` | Opaque cursor from the previous page |

#### Create Graph

```bash
zepctl graph create [flags]
```

| Flag | Description |
|------|-------------|
| `--name` | Graph name |
| `--description` | Graph description |

#### Delete Graph

```bash
zepctl graph delete <graph-uuid> [flags]
```

| Flag | Description |
|------|-------------|
| `--force` | Skip confirmation prompt |
| `--wait` | Wait for the deletion task |
| `--timeout` | Maximum wait time (default: 5m) |

**Note**: To delete a user graph, use `zepctl user delete` instead.

#### Clone Graph

```bash
zepctl graph clone [flags]
```

| Flag | Description |
|------|-------------|
| `--source-user` | Source user UUID (for user graphs) |
| `--source-graph` | Source graph UUID (for standalone graphs) |
| `--wait` | Wait for clone operation to complete |
| `--timeout` | Maximum wait time (default: 5m) |

**Examples**:
```bash
# Clone a user graph
zepctl graph clone --source-user <user-uuid>

# Clone a standalone graph
zepctl graph clone --source-graph <graph-uuid>
```

#### Add Data to Graph

```bash
zepctl graph add <graph-uuid> [flags]
```

| Flag | Description |
|------|-------------|
| `--type` | Data type: `text`, `json`, `message` (default: `text`) |
| `--data` | Inline data string |
| `--file` | Path to data file |
| `--stdin` | Read data from stdin |
| `--user` | User UUID for a user graph |
| `--batch` | Enable batch processing (up to 20 episodes) |
| `--wait` | Wait for ingestion to complete |

**Examples**:
```bash
# Add text data to a user graph
zepctl graph add --user user_123 --type text --data "The user prefers dark mode"

# Add JSON data from file
zepctl graph add graph_456 --type json --file data.json

# Batch import from file
zepctl graph add --user user_123 --batch --file episodes.json --wait
```

**Batch File Format (JSON)**:
```json
{
  "episodes": [
    {"type": "text", "data": "User prefers morning meetings"},
    {"type": "json", "data": "{\"preference\": \"dark_mode\"}"},
    {"type": "message", "data": "Alice: I love hiking on weekends"}
  ]
}
```

#### Search Graph

```bash
zepctl graph search <query> [flags]
```

| Flag | Description |
|------|-------------|
| `--user` | Search user graph |
| `--graph` | Search standalone graph |
| `--scope` | Search scope: `edges`, `nodes`, `episodes`, `observations`, `thread_summaries`, `auto` (default: `edges`) |
| `--limit` | Maximum results (default: 10) |
| `--reranker` | Reranker: `rrf`, `mmr`, `cross_encoder` |
| `--mmr-lambda` | MMR diversity/relevance balance (0-1) |
| `--max-characters` | Max total characters across selected results (`scope=auto`, max 50000) |
| `--return-raw-results` | When `scope=auto`, include raw graph results alongside the materialized context block |
| `--exclude-node-labels` | Comma-separated node labels to exclude (`scope=nodes`) |
| `--node-labels` | Comma-separated node labels to include (`scope=nodes`) |
| `--exclude-edge-types` | Comma-separated edge types to exclude (`scope=edges`) |
| `--edge-types` | Comma-separated edge types to include (`scope=edges`) |
| `--property-filter` | Repeatable property filter (`scope=edges` or `scope=nodes`) |
| `--date-filter` | Repeatable date filter (`scope=edges`) |

The command rejects filter flags that the selected scope does not support.

**Examples**:
```bash
# Search a user graph for edges (facts)
zepctl graph search "project status" --user <user-uuid> --scope edges

# Search with cross-encoder reranking
zepctl graph search "critical decisions" --user user_123 --reranker cross_encoder

# Search nodes with filters
zepctl graph search "product" --graph graph_456 --scope nodes --exclude-node-labels "Assistant,Document"
```

---

### Node Commands

#### List Nodes

```bash
zepctl node list [flags]
```

| Flag | Description |
|------|-------------|
| `--user` | User UUID for a user graph |
| `--graph` | Graph UUID for a standalone graph |
| `--limit` | Maximum number of results (default: 50) |
| `--cursor` | Opaque cursor from the previous page |

#### Get Node

```bash
zepctl node get <uuid>
```

#### Get Node Edges

```bash
zepctl node edges <uuid>
```

Returns all entity edges connected to the specified node.

#### Get Node Episodes

```bash
zepctl node episodes <uuid>
```

Returns all episodes that mention the specified node.

---

### Edge Commands

#### List Edges

```bash
zepctl edge list [flags]
```

| Flag | Description |
|------|-------------|
| `--user` | User UUID for a user graph |
| `--graph` | Graph UUID for a standalone graph |
| `--limit` | Maximum number of results (default: 50) |
| `--cursor` | Opaque cursor from the previous page |

#### Get Edge

```bash
zepctl edge get <uuid>
```

#### Delete Edge

```bash
zepctl edge delete <uuid> [flags]
```

| Flag | Description |
|------|-------------|
| `--force` | Skip confirmation prompt |
| `--wait` | Wait for the deletion task |
| `--timeout` | Maximum wait time (default: 5m) |

---

### Episode Commands

#### List Episodes

```bash
zepctl episode list [flags]
```

| Flag | Description |
|------|-------------|
| `--user` | List episodes for user graph |
| `--graph` | List episodes for standalone graph |
| `--limit` | Maximum number of results (default: 50) |
| `--cursor` | Opaque cursor from the previous page |
| `--last` | Get last N episodes (shortcut, ignores pagination) |

#### Get Episode

```bash
zepctl episode get <uuid>
```

#### Get Episode Mentions

```bash
zepctl episode mentions <episode-uuid> [--limit N] [--nodes-cursor CURSOR] [--edges-cursor CURSOR]
```

| Flag | Description |
|------|-------------|
| `--limit` | Maximum number of results in each list (default: 50) |
| `--nodes-cursor` | Cursor for the node list |
| `--edges-cursor` | Cursor for the edge list |

The command returns nodes and edges that the episode mentions.

#### Delete Episode

```bash
zepctl episode delete <uuid> [flags]
```

| Flag | Description |
|------|-------------|
| `--force` | Skip confirmation prompt |
| `--wait` | Wait for the deletion task |
| `--timeout` | Maximum wait time (default: 5m) |

---

### Observation Commands

Read-only access to derived observation nodes for a user or graph.

#### List Observations

```bash
zepctl observation list [flags]
```

| Flag | Description |
|------|-------------|
| `--user` | List observations for user graph |
| `--graph` | List observations for standalone graph |
| `--limit` | Maximum results (default: 50) |
| `--cursor` | UUID cursor for pagination (last UUID from previous page) |

#### Get Observation

```bash
zepctl observation get <uuid>
```

---

### Thread Summary Commands

List incremental thread summaries derived from messages in a user's or graph's threads.

#### List Thread Summaries

```bash
zepctl thread-summary list [flags]
```

| Flag | Description |
|------|-------------|
| `--user` | List thread summaries for user graph |
| `--graph` | List thread summaries for standalone graph |
| `--limit` | Maximum results (default: 50) |
| `--cursor` | UUID cursor for pagination (last UUID from previous page) |

---

### Task Commands

For monitoring async operations (batch imports, cloning, etc.)

#### Get Task Status

```bash
zepctl task get <task-id>
```

**Output Fields**: `task_id`, `status`, `created_at`, `completed_at`, `error`

**Status Values**: `pending`, `processing`, `completed`, `failed`

#### Wait for Task

```bash
zepctl task wait <task-id> [flags]
```

| Flag | Description |
|------|-------------|
| `--timeout` | Maximum wait time (default: 5m) |
| `--poll-interval` | Polling interval (default: 1s) |

---

### Ontology Commands

#### Get Ontology

```bash
zepctl ontology get [flags]
```

| Flag | Description |
|------|-------------|
| `--user` | Get ontology for specific user |
| `--graph` | Get ontology for specific graph |

Returns current entity and edge type definitions.

#### Set Ontology

```bash
zepctl ontology set [flags]
```

| Flag | Description |
|------|-------------|
| `--file` | Path to ontology definition file (YAML/JSON) |
| `--user` | Apply to specific user(s) (comma-separated) |
| `--graph` | Apply to specific graph(s) (comma-separated) |

**Note**: If no `--user` or `--graph` is specified, ontology is applied project-wide.

**Ontology File Format (YAML)**:
```yaml
entities:
  Customer:
    description: "A customer of the business"
    fields:
      tier:
        description: "Customer tier level"
      account_number:
        description: "Customer account number"
  Product:
    description: "A product or service"
    fields:
      sku:
        description: "Product SKU"

edges:
  PURCHASED:
    description: "Customer purchased a product"
    source_types: [Customer]
    target_types: [Product]
  INTERESTED_IN:
    description: "Customer expressed interest"
```

---

### User Summary Instructions Commands

#### List Instructions

```bash
zepctl summary-instructions list [flags]
```

| Flag | Description |
|------|-------------|
| `--user` | Filter by user ID |
| `--limit` | Maximum number of results (default: 50) |
| `--cursor` | Opaque cursor from the previous page |

#### Add Instructions

```bash
zepctl summary-instructions add [flags]
```

| Flag | Description |
|------|-------------|
| `--instruction` | Instruction text |
| `--file` | Path to file containing instructions |
| `--user` | Apply to specific user(s) |

#### Delete Instructions

```bash
zepctl summary-instructions delete <instruction-id> [flags]
```

| Flag | Description |
|------|-------------|
| `--force` | Skip confirmation prompt |

---

### Policy Set Commands

ABAC policy sets are reusable bundles of access rules attached to API keys. Policy-set commands require a bearer token (`auth login`) and operate on the currently-selected project (including `validate`, which calls the server).

```bash
zepctl policy-set list
zepctl policy-set get <uuid>
zepctl policy-set create --file <path.yaml>
zepctl policy-set update <uuid> --file <path.yaml>
zepctl policy-set delete <uuid> [--force]
zepctl policy-set validate --file <path.yaml>
zepctl policy-set retention-target list <policy-set-uuid>
zepctl policy-set retention-target attach <policy-set-uuid> --pattern <pattern>
zepctl policy-set retention-target detach <policy-set-uuid> --pattern <pattern>
```

| Flag | Description |
|------|-------------|
| `--file` | (create/update/validate) Path to policy set YAML file |
| `--force` | (delete) Skip confirmation prompt |
| `--pattern` | (retention-target attach/detach) Required graph ID pattern |

- `validate` exits 0 if the spec is valid, 1 if validation fails (errors printed to stderr), and 2 on client or transport errors.
- `delete` requires `--force` in non-interactive contexts; in a TTY it prompts for confirmation.
- Table output for `get` shows the policy set metadata plus the spec rendered as indented YAML.
- `--pattern` accepts an exact graph ID, a prefix that ends in `*`, or `*` for all graphs in the project. The server validates the pattern.
- A retention policy set has no effect until you attach it to a target.
- `attach` and `detach` print a confirmation in table output. JSON and YAML output return the full target list.
- The server accepts targets only for retention policy sets. It rejects targets for other policy-set types.

---

### API Key ABAC Commands

Configure ABAC enforcement on individual API keys and dry-run policy decisions. All `api-key` commands require a bearer token.

```bash
# List API keys (UUID, name, masked key, role)
zepctl api-key list

# Per-key ABAC settings
zepctl api-key settings get <key-uuid>
zepctl api-key settings set <key-uuid> --mode <off|report_only|enforce>

# Policy set attachments
zepctl api-key policy-sets list <key-uuid>
zepctl api-key policy-sets attach <key-uuid> <policy-set-uuid>
zepctl api-key policy-sets detach <key-uuid> <policy-set-uuid>

# Dry-run policy decisions (do not perform the action)
zepctl api-key evaluate <key-uuid> --action <name>   # decision only
zepctl api-key explain <key-uuid> --action <name>    # decision + full evaluator trace
```

| Flag | Description |
|------|-------------|
| `--mode` | (`settings set`) ABAC enforcement mode: `off`, `report_only`, `enforce` |
| `--action` | (`evaluate`, `explain`) Required. Action name to evaluate (e.g. `thread.get`) |

- `settings set` requires at least one setting flag.
- `evaluate` returns the final outcome (`allow` / `deny`), the ABAC and ABAC-shadow decisions, whether the role would have allowed the action, and whether the disagreement would be logged.
- `explain` returns everything `evaluate` does, plus the registry entry's read-only flag and a list of evaluated and skipped policy sets with per-policy match reasons.
- Both commands always exit 0 on a successful API call regardless of allow/deny outcome -- inspect the JSON or table output to check the decision.

**Examples**:
```bash
# Switch a key to report-only mode and attach a policy set
zepctl api-key settings set abcd1234-... --mode report_only
zepctl api-key policy-sets attach abcd1234-... efgh5678-...

# Check what the evaluator would decide for thread.get
zepctl api-key evaluate abcd1234-... --action thread.get -o json
```

---

### UserGroup ABAC Commands

Manage UserGroups, their members, and the policy sets attached to them. All `user-group` commands require a bearer token.

Every project has a virtual **All Users** group containing every active user. It accepts policy attachments -- the project-wide grant path -- but cannot be renamed, deleted, or given members; those mutations return a conflict. It sorts first in `user-group list`.

```bash
# Groups
zepctl user-group list [--limit N] [--cursor CURSOR] [--search TERM]
zepctl user-group get <group-uuid>
zepctl user-group create --name <name> [--description <text>]
zepctl user-group update <group-uuid> [--name <name>] [--description <text>] [--expected-version N]
zepctl user-group delete <group-uuid> [--force]

# Membership (users identified by UUID)
zepctl user-group members list <group-uuid> [--limit N] [--cursor CURSOR] [--search TERM]
zepctl user-group members candidates <group-uuid> [--limit N] [--cursor CURSOR] [--search TERM]
zepctl user-group members add <group-uuid> <user-uuid>...
zepctl user-group members remove <group-uuid> <user-uuid>...

# Policy set attachments
zepctl user-group policy-sets list <group-uuid>
zepctl user-group policy-sets attach <group-uuid> <policy-set-uuid>
zepctl user-group policy-sets detach <group-uuid> <policy-set-uuid>

# Dry-run policy decisions (do not perform the action)
zepctl user-group evaluate <group-uuid> --action <name> [--graph <graph-uuid>]
zepctl user-group explain <group-uuid> --action <name> [--graph <graph-uuid>]
```

| Flag | Description |
|------|-------------|
| `--name` | (`create`, `update`) Group name; required on `create` |
| `--description` | (`create`, `update`) Group description |
| `--expected-version` | (`update`) Version the update must match; if the group version differs, the update conflicts. Without the flag, the command reads and sends the current version; a conflict occurs only if another update happens between the read and the write. |
| `--search` | (list commands) Name search; user search for the member commands |
| `--force` | (`delete`) Skip confirmation prompt |
| `--action` | (`evaluate`, `explain`) Required. Action name to evaluate |
| `--graph` | (`evaluate`, `explain`) Standalone graph UUID to evaluate as the resource |

- `update` requires at least one of `--name` or `--description`. Without `--expected-version`, it reads and sends the group's current version, so a conflict occurs only if another update happens between the read and the write. With `--expected-version`, it returns a conflict if the group's version differs from that value.
- `delete` removes the group's memberships and policy attachments. It requires `--force` in non-interactive contexts.
- `members candidates` lists active users who are not yet members. Adding an existing member or removing an absent one is a successful no-op and is reported in the count. The server accepts up to 50 UUIDs per call.
- Only authorization policy sets can attach to a UserGroup; retention sets are rejected.
- `evaluate` adds the resolved principal, resource, and default-deny base to the `api-key evaluate` output. `explain` adds the effective UserGroups, the groups contributing each policy set, whether a match was graph-specific or action-wide, skipped policies, object constraints, and dead graph references.
- `--graph` takes a standalone graph's canonical UUID, not its `graph_id`. Omit it for the action-only question.
- Both dry runs always exit 0 on a successful API call regardless of outcome.

**Examples**:
```bash
# Create a group, populate it, grant it a policy set
zepctl user-group create --name Support --description "Support-facing users"
zepctl user-group members candidates abcd1234-...
zepctl user-group members add abcd1234-... 2b3c4d5e-...
zepctl user-group policy-sets attach abcd1234-... efgh5678-...

# Ask why a graph search would be allowed or denied
zepctl user-group explain abcd1234-... --action graph.search --graph 4d5e6f7a-...

# Grant the same policy set to every user in the project
zepctl user-group policy-sets attach <all-users-uuid> efgh5678-...
```

---

## Scripting Examples

### Export All Users

```bash
zepctl user list -o json | jq '.users[].user_id'
```

### Bulk User Creation

```bash
cat users.json | jq -c '.[]' | while read user; do
  zepctl user create --user-id "$(echo "$user" | jq -r '.user_id')" \
    --email "$(echo $user | jq -r '.email')" \
    --first-name "$(echo $user | jq -r '.first_name')"
done
```

### Migrate User Data

```bash
# Clone user graph to test environment
cloned_graph_uuid=$(zepctl graph clone --source-user <source-user-uuid> --wait -o json | jq -r '.graph.uuid')

# Verify clone
zepctl node list --graph "$cloned_graph_uuid" -o json | jq '.items | length'
```

### Monitor Batch Import

```bash
# Start batch import
TASK_ID=$(zepctl graph add --user user_123 --batch --file data.json -o json | jq -r '.task_id')

# Wait for completion
zepctl task wait $TASK_ID --timeout 10m
```

### Delete User (RTBF Compliance)

```bash
# Preview what will be deleted
zepctl user get $USER_ID
zepctl user threads $USER_ID

# Delete user and all associated data
zepctl user delete $USER_UUID --force --wait
```

---

## Error Handling

### Exit Codes

| Code | Description |
|------|-------------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid arguments |
| 3 | Authentication error |
| 4 | Resource not found |
| 5 | Rate limit exceeded |
| 6 | Server error |
| 7 | Timeout |

### Error Output Format

```json
{
  "error": {
    "code": "RESOURCE_NOT_FOUND",
    "message": "User 'user_123' not found",
    "details": {
      "resource_type": "user",
      "resource_id": "user_123"
    }
  }
}
```

---

## Implementation Notes

### Technology Stack

- **Language**: Go
- **CLI Framework**: Cobra
- **Configuration**: Viper
- **HTTP Client**: Standard library with retry logic
- **Output Formatting**: `tablewriter` for tables, `encoding/json` and `gopkg.in/yaml.v3`

### Rate Limiting

The CLI should implement client-side rate limiting awareness:
- Respect `Retry-After` headers
- Exponential backoff on 429 responses
- `--retry` flag for automatic retry configuration

### Pagination

List commands automatically paginate unless `--no-paginate` is specified. Use `--all` to fetch all results.

### Caching

Consider implementing local caching for:
- Project info
- Ontology definitions
- User lookups (with TTL)

---

## Future Considerations

1. **Plugin System**: Allow custom commands via plugins
2. **Shell Completions**: Generate completions for bash, zsh, fish, PowerShell
3. **TUI Mode**: Rich terminal UI for graph exploration
4. **Export/Import**: Full project backup and restore
5. **Diff Tool**: Compare graph states across users or time periods
6. **Watch Mode**: Real-time monitoring of graph changes
7. **Local Development**: Support for local Zep instances
