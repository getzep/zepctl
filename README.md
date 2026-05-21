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
- **Bearer token** -- obtained interactively via `zepctl auth login` (OAuth / Kinde) and stored as a refresh token in the system keychain. Required for ABAC management (`policy-set`, `api-key`) and the interactive `config set-project` flow.

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
| `user` | Manage users |
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

## License

See [LICENSE](LICENSE) for details.
