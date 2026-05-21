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
- **Bearer token** (OAuth / Kinde): obtained interactively via `zepctl auth login`. Required for the ABAC management commands (`policy-set`, `api-key`) and the `config set-project` interactive flow. Refresh tokens are stored in the system keychain.

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
  page-size: 50
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
| `--page` | Page number (default: 1) |
| `--page-size` | Results per page (default: 50) |

#### Get User

```bash
zepctl user get <user-id>
```

**Output Fields**: `user_id`, `uuid`, `email`, `first_name`, `last_name`, `created_at`, `metadata`

#### Create User

```bash
zepctl user create <user-id> [flags]
```

| Flag | Description |
|------|-------------|
| `--email` | User email address |
| `--first-name` | User first name |
| `--last-name` | User last name |
| `--metadata` | JSON metadata string |
| `--metadata-file` | Path to JSON metadata file |

#### Update User

```bash
zepctl user update <user-id> [flags]
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
zepctl user delete <user-id> [flags]
```

| Flag | Description |
|------|-------------|
| `--force` | Skip confirmation prompt |

**Note**: Deleting a user removes all associated threads, graph data, and knowledge. Supports RTBF compliance.

#### List User Threads

```bash
zepctl user threads <user-id> [flags]
```

| Flag | Description |
|------|-------------|
| `--page` | Page number (default: 1) |
| `--page-size` | Results per page (default: 50) |

#### Get User Graph Node

```bash
zepctl user node <user-id>
```

---

### Thread Commands

#### Create Thread

```bash
zepctl thread create <thread-id> --user <user-id> [flags]
```

| Flag | Description |
|------|-------------|
| `--user` | User ID (required) |
| `--metadata` | JSON metadata string |
| `--metadata-file` | Path to JSON metadata file |

#### Get Thread

```bash
zepctl thread get <thread-id>
```

#### Delete Thread

```bash
zepctl thread delete <thread-id> [flags]
```

| Flag | Description |
|------|-------------|
| `--force` | Skip confirmation prompt |

#### List Thread Messages

```bash
zepctl thread messages <thread-id> [flags]
```

| Flag | Description |
|------|-------------|
| `--page` | Page number (default: 1) |
| `--page-size` | Results per page (default: 50) |

#### Add Messages to Thread

```bash
zepctl thread add-messages <thread-id> [flags]
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
      "content": "Hello, I need help with my account"
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
zepctl thread context <thread-id>
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
| `--page` | Page number (default: 1) |
| `--page-size` | Results per page (default: 50) |

#### Create Graph

```bash
zepctl graph create <graph-id>
```

#### Delete Graph

```bash
zepctl graph delete <graph-id> [flags]
```

| Flag | Description |
|------|-------------|
| `--force` | Skip confirmation prompt |

**Note**: To delete a user graph, use `zepctl user delete` instead.

#### Clone Graph

```bash
zepctl graph clone [flags]
```

| Flag | Description |
|------|-------------|
| `--source-user` | Source user ID (for user graphs) |
| `--target-user` | Target user ID (for user graphs) |
| `--source-graph` | Source graph ID (for standalone graphs) |
| `--target-graph` | Target graph ID (for standalone graphs) |
| `--wait` | Wait for clone operation to complete |

**Examples**:
```bash
# Clone a user graph
zepctl graph clone --source-user user_123 --target-user user_123_test

# Clone a standalone graph
zepctl graph clone --source-graph graph_456 --target-graph graph_456_backup
```

#### Add Data to Graph

```bash
zepctl graph add <graph-id> [flags]
```

| Flag | Description |
|------|-------------|
| `--type` | Data type: `text`, `json`, `message` (default: `text`) |
| `--data` | Inline data string |
| `--file` | Path to data file |
| `--stdin` | Read data from stdin |
| `--user` | Add to user graph instead of standalone graph |
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
| `--exclude-node-labels` | Comma-separated node labels to exclude |
| `--exclude-edge-types` | Comma-separated edge types to exclude |

**Examples**:
```bash
# Search user graph for edges (facts)
zepctl graph search "project status" --user user_123 --scope edges

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
| `--user` | List nodes for user graph |
| `--graph` | List nodes for standalone graph |
| `--page` | Page number (default: 1) |
| `--page-size` | Results per page (default: 50) |

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
| `--user` | List edges for user graph |
| `--graph` | List edges for standalone graph |
| `--page` | Page number (default: 1) |
| `--page-size` | Results per page (default: 50) |

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
| `--page` | Page number (default: 1) |
| `--page-size` | Results per page (default: 50) |
| `--last` | Get last N episodes (shortcut, ignores pagination) |

#### Get Episode

```bash
zepctl episode get <uuid>
```

#### Get Episode Mentions

```bash
zepctl episode mentions <uuid>
```

Returns nodes and edges mentioned in the specified episode.

#### Delete Episode

```bash
zepctl episode delete <uuid> [flags]
```

| Flag | Description |
|------|-------------|
| `--force` | Skip confirmation prompt |

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
| `--page` | Page number (default: 1) |
| `--page-size` | Results per page (default: 50) |

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
```

| Flag | Description |
|------|-------------|
| `--file` | (create/update/validate) Path to policy set YAML file |
| `--force` | (delete) Skip confirmation prompt |

- `validate` exits 0 if the spec is valid, 1 if validation fails (errors printed to stderr), and 2 on client or transport errors.
- `delete` requires `--force` in non-interactive contexts; in a TTY it prompts for confirmation.
- Table output for `get` shows the policy set metadata plus the spec rendered as indented YAML.

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

## Scripting Examples

### Export All Users

```bash
zepctl user list -o json | jq '.users[].user_id'
```

### Bulk User Creation

```bash
cat users.json | jq -c '.[]' | while read user; do
  zepctl user create $(echo $user | jq -r '.user_id') \
    --email "$(echo $user | jq -r '.email')" \
    --first-name "$(echo $user | jq -r '.first_name')"
done
```

### Migrate User Data

```bash
# Clone user graph to test environment
zepctl graph clone --source-user prod_user_123 --target-user test_user_123 --wait

# Verify clone
zepctl node list --user test_user_123 -o json | jq '.nodes | length'
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
zepctl user delete $USER_ID --force
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
