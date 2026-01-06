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

### Configuration File

Location: `~/.zepctl/config.yaml`

```yaml
current-profile: production
profiles:
  - name: production
    # API keys are stored securely in the system keychain
  - name: development
    api-url: https://api.dev.getzep.com  # Optional: only if using non-default URL
defaults:
  output: table
  page-size: 50
```

**Credential Storage**: API keys are stored in the system keychain (macOS Keychain, Windows Credential Manager, or Linux Secret Service) rather than in the config file. For CI/CD environments without keychain access, use the `ZEP_API_KEY` environment variable.

### Environment Variables

| Variable | Description |
|----------|-------------|
| `ZEP_API_KEY` | API key for authentication |
| `ZEP_API_URL` | API endpoint URL (default: `https://api.getzep.com`) |
| `ZEP_PROFILE` | Override current profile |
| `ZEP_OUTPUT` | Default output format |

### Configuration Commands

```bash
zepctl config use-profile <name>       # Switch active profile
zepctl config get-profiles             # List all profiles
zepctl config add-profile <name>       # Add a new profile
zepctl config delete-profile <name>    # Remove a profile
zepctl config view                     # Display current configuration
```

## Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--api-key` | `-k` | Override API key |
| `--api-url` | | Override API URL |
| `--profile` | `-p` | Use specific profile |
| `--output` | `-o` | Output format: `table`, `json`, `yaml`, `wide` |
| `--quiet` | `-q` | Suppress non-essential output |
| `--verbose` | `-v` | Enable verbose output |
| `--help` | `-h` | Display help |
| `--version` | | Display version |

---

## Command Reference

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
| `--scope` | Search scope: `edges`, `nodes`, `episodes` (default: `edges`) |
| `--limit` | Maximum results (default: 10) |
| `--reranker` | Reranker: `rrf`, `mmr`, `cross_encoder` |
| `--mmr-lambda` | MMR diversity/relevance balance (0-1) |
| `--min-score` | Minimum relevance score |
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
