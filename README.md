# zepctl

Command-line interface for administering Zep projects.

## Installation

### Homebrew (macOS/Linux)

```bash
brew tap getzep/zepctl https://github.com/getzep/zepctl.git
brew install zepctl
```

### Binary Download

Download the appropriate binary from the [releases page](https://github.com/getzep/zepctl/releases).

**macOS users:** If you see "zepctl cannot be opened because the developer cannot be verified", run:
```bash
xattr -d com.apple.quarantine /path/to/zepctl
```

## Quick Start

```bash
# Configure your API key (you will be prompted to enter it securely)
zepctl config add-profile production

# Verify connection
zepctl project get

# List users
zepctl user list
```

## Authentication

zepctl supports two authentication modes (which coexist on the same profile):

- **API key** -- long-lived, set via `ZEP_API_KEY` or stored per-profile in the system keychain. Used for headless / CI scenarios.
- **Bearer token** -- obtained interactively via `zepctl auth login` (OAuth / Kinde) and stored as a refresh token in the system keychain. Required for ABAC management (`policy-set`, `api-key`, `user-group`, and `user groups|evaluate|explain`) and the interactive `config set-project` flow.

`ZEP_BEARER_TOKEN` sets the bearer token for bearer commands. The CLI does not refresh this token. It overrides the keychain token.

```bash
# Browser-based login; auto-selects a project after authentication
zepctl auth login

# Headless mode prints the URL instead of opening a browser
zepctl auth login --no-browser

# Inspect credentials and bearer expiration
zepctl auth status
```

| Variable | Description |
|----------|-------------|
| `ZEP_API_KEY` | API key for authentication |
| `ZEP_BEARER_TOKEN` | Bearer token override for bearer commands |
| `ZEP_API_URL` | API endpoint (default: `https://api.getzep.com`) |
| `ZEP_PROFILE` | Override current profile |
| `ZEP_PROJECT` | Override active project UUID |

Configuration file location: `~/.zepctl/config.yaml`. API keys and OAuth refresh tokens are stored in the system keychain.

## Commands

| Command | Description |
|---------|-------------|
| `config` | Manage profiles, environment presets, and the active project |
| `auth` | Bearer-token login / logout / status |
| `project` | Get project information |
| `user` | Manage users; inspect their UserGroups and dry-run ABAC decisions (bearer auth for those three) |
| `thread` | Manage conversation threads |
| `graph` | Manage knowledge graphs |
| `node` | Manage graph nodes |
| `edge` | Manage graph edges |
| `episode` | Manage graph episodes |
| `observation` | List/get derived observation nodes |
| `thread-summary` | List incremental thread summaries |
| `task` | Monitor async operations |
| `ontology` | Manage graph schema |
| `summary-instructions` | Manage user summary instructions |
| `policy-set` | Manage ABAC policy sets (bearer auth) |
| `api-key` | List API keys, configure ABAC, and dry-run policy decisions (bearer auth) |
| `user-group` | Manage UserGroups, membership, and their ABAC policy attachments (bearer auth) |

## Global Flags

| Flag | Description |
|------|-------------|
| `--api-key`, `-k` | Override API key |
| `--profile`, `-p` | Use specific profile |
| `--project` | Override active project UUID for this command |
| `--output`, `-o` | Output format: `table`, `json`, `yaml`, `wide` |
| `--help`, `-h` | Display help |

## Documentation

See [docs/cli.mdx](docs/cli.mdx) for complete CLI reference.

Commands that address users, threads, and graphs require Zep UUIDs. A v3 ID returns `invalid_identifier`. The `user create --user-id` flag sets an external identity-provider ID; it is not a resource address.

Use `--limit` and `--cursor` to list the next page. The cursor is opaque.

```bash
zepctl user list --limit 50
zepctl user get <user-uuid>
zepctl user create --user-id <external-id>
zepctl thread create --user <user-uuid>
zepctl graph create --name <name> --description <description>
zepctl graph delete <graph-uuid> --force --wait
```

Delete commands that return an asynchronous task print its UUID. Add `--wait` to wait for completion. `node update` does not support `--labels`.

## License

See [LICENSE](LICENSE) for details.
