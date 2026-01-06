# Agent Development Guide for zepctl

This document outlines development practices required when contributing to the zepctl codebase.

## Project Overview

- **Language**: Go 1.25+
- **CLI Framework**: Cobra with Viper for configuration
- **SDK**: github.com/getzep/zep-go/v3

## Build and Quality Commands

```bash
make build          # Build for current platform
make test           # Run tests with race detection
make lint           # Run golangci-lint
make fmt            # Format code with gofumpt
make fmt-check      # Verify formatting without changes
make tidy           # Tidy go.mod dependencies
```

Always run `make lint` before committing changes.

## Code Style

### Formatting

- Use `gofumpt` for formatting (stricter than `gofmt`)
- Run `make fmt` to format all files
- CI will fail if code is not properly formatted

### Linting

The project uses golangci-lint v2 with the following enabled linters:

- `bodyclose`, `copyloopvar`, `dogsled`, `errorlint`
- `goconst`, `gocritic`, `gocyclo`, `godot`, `gosec`
- `misspell`, `nakedret`, `noctx`, `nolintlint`
- `revive`, `staticcheck`, `unconvert`, `unparam`, `whitespace`

Key linter settings:
- **goconst**: Minimum 8 occurrences before suggesting a constant
- **gocyclo**: Maximum complexity of 15
- **nolintlint**: Requires explanation and specific linter name for `//nolint` directives

### Error Handling

- Always wrap errors with context: `fmt.Errorf("operation: %w", err)`
- Use the `errorlint` patterns for error comparisons
- Check type assertions: `value, ok := x.(Type)`

### Naming Conventions

- Follow Go standard naming conventions
- Use `ID` not `Id` for identifiers (e.g., `UserID`, `GraphID`)
- Use `UUID` not `Uuid` for UUIDs
- Use `URL` not `Url` for URLs
- Use `API` not `Api` for API references

## Project Structure

```
cmd/zepctl/         # Main entry point
internal/
  cli/              # Cobra command implementations
  client/           # Zep API client wrapper
  config/           # Configuration management
  output/           # Output formatting (table, JSON, YAML)
docs/               # Documentation files
```

## Adding New Commands

1. Create a new file in `internal/cli/` (e.g., `resource.go`)
2. Define the command hierarchy:
   ```go
   var resourceCmd = &cobra.Command{
       Use:   "resource",
       Short: "Manage resources",
       Long:  `Detailed description of resource management.`,
   }
   ```
3. Register commands in `init()`:
   ```go
   func init() {
       rootCmd.AddCommand(resourceCmd)
       resourceCmd.AddCommand(resourceListCmd)
       // Add flags
       resourceListCmd.Flags().Int("limit", 50, "Maximum results")
   }
   ```
4. Follow existing patterns for table output, error handling, and confirmation prompts

## SDK Usage

### Pagination

The Zep SDK uses cursor-based pagination:
- `Limit *int` - Maximum items to return
- `UUIDCursor *string` - UUID of last item from previous page

Use `zep.Int()` and `zep.String()` helpers for pointer fields:
```go
req := &zep.GraphNodesRequest{
    Limit:      zep.Int(limit),
    UUIDCursor: zep.String(cursor),
}
```

### Search Filters

When implementing exclusion filters, use the correct SDK fields:
- `ExcludeNodeLabels` (not `NodeLabels`) for `--exclude-node-labels` flag
- `ExcludeEdgeTypes` (not `EdgeTypes`) for `--exclude-edge-types` flag

## Output Formatting

Support multiple output formats via the `output` package:

```go
if output.GetFormat() == output.FormatTable {
    tbl := output.NewTable("COLUMN1", "COLUMN2")
    tbl.WriteHeader()
    tbl.WriteRow(value1, value2)
    return tbl.Flush()
}
return output.Print(data)  // JSON/YAML output
```

## User Input

### Confirmation Prompts

For destructive operations, require confirmation unless `--force` is provided:

```go
if !force {
    fmt.Printf("Delete %q? [y/N]: ", name)
    reader := bufio.NewReader(os.Stdin)
    response, _ := reader.ReadString('\n')
    response = strings.TrimSpace(strings.ToLower(response))
    if response != "y" && response != "yes" {
        output.Info("Aborted")
        return nil
    }
}
```

### Secure Input

For sensitive data like API keys, use `golang.org/x/term` for masked input:

```go
if term.IsTerminal(int(os.Stdin.Fd())) {
    keyBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
    fmt.Println() // newline after hidden input
    // ...
}
```

## Flag Validation

Implement mutual exclusion and dependency validation early in command handlers:

```go
if sourceUser != "" && sourceGraph != "" {
    return fmt.Errorf("--source-user and --source-graph are mutually exclusive")
}

if sourceUser != "" && targetGraph != "" {
    return fmt.Errorf("--target-graph cannot be used with --source-user")
}
```

## Documentation

- Update `docs/cli.mdx` when adding or modifying commands
- Include usage examples in command `Long` descriptions
- Document all flags with clear descriptions

## CI/CD

### GitHub Actions

- **CI** (`ci.yml`): Runs lint, test, and build on PRs
- **Release** (`release.yml`): Creates releases via GoReleaser on tags

### GoReleaser

Releases are built for:
- Linux (amd64, arm64)
- macOS (arm64)

Homebrew tap is published to `getzep/homebrew-tap`.

## Testing

Run tests with race detection:
```bash
make test
```

Tests should:
- Use table-driven test patterns
- Mock external API calls
- Avoid hardcoded test data that could become stale
