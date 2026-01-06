# zepctl

Command-line interface for administering Zep projects.

## Installation

### Homebrew (macOS/Linux)

```bash
brew install getzep/zepctl/zepctl
```

### Binary Download

Download the appropriate binary from the [releases page](https://github.com/getzep/zepctl/releases).

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

Set environment variables or use profiles:

| Variable | Description |
|----------|-------------|
| `ZEP_API_KEY` | API key for authentication |
| `ZEP_API_URL` | API endpoint (default: `https://api.getzep.com`) |
| `ZEP_PROFILE` | Override current profile |

Configuration file location: `~/.zepctl/config.yaml`

## Commands

| Command | Description |
|---------|-------------|
| `config` | Manage profiles and settings |
| `project` | Get project information |
| `user` | Manage users |
| `thread` | Manage conversation threads |
| `graph` | Manage knowledge graphs |
| `node` | Manage graph nodes |
| `edge` | Manage graph edges |
| `episode` | Manage graph episodes |
| `task` | Monitor async operations |
| `ontology` | Manage graph schema |
| `summary-instructions` | Manage user summary instructions |

## Global Flags

| Flag | Description |
|------|-------------|
| `--api-key`, `-k` | Override API key |
| `--profile`, `-p` | Use specific profile |
| `--output`, `-o` | Output format: `table`, `json`, `yaml`, `wide` |
| `--help`, `-h` | Display help |

## Documentation

See [docs/cli.mdx](docs/cli.mdx) for complete CLI reference.

## License

See [LICENSE](LICENSE) for details.
