# Contributing to rho

Thanks for considering contributing to rho! This guide explains how to contribute effectively.

## Code of Conduct

By participating in this project, you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).

## How to Contribute

### Reporting Bugs

1. **Search existing issues** — your bug may already be reported.
2. **Use the issue template** — provide a clear, concise reproduction.
3. **Include system info** — terminal emulator, OS, Go version, and rho version.

### Feature Requests

- Keep it focused. Explain the problem you're solving, not just the solution.
- Check if the feature could be an extension instead of core.

### Pull Requests

1. **Start with an issue** — open an issue first to discuss the change before writing code.
2. **Keep it focused** — one change per PR. Avoid lumping unrelated changes.
3. **Write tests** — new features should include tests. Bug fixes should include a regression test.
4. **Run the checks** before submitting:

```bash
go build ./...
go test ./...
go vet ./...
```

5. **No dependency bloat** — rho has zero external Go dependencies beyond `golang.org/x/text` and `golang.org/x/sys`. New dependencies require explicit justification.

## Development Setup

### Prerequisites

- Go 1.26+
- A terminal with Unicode support

### Building

```bash
go build -o rho ./cmd/rho
```

### Running Tests

```bash
go test ./...
```

### Project Layout

```
├── cmd/rho/          # CLI entry point
├── pkg/
│   ├── tui/          # Terminal UI framework
│   ├── ai/           # AI providers & model framework
│   └── agent/        # Agent core loop, tools, extensions
├── examples/         # Example extensions
└── docs/             # Documentation
```

## Design Principles

- **Minimal core** — features that don't belong in core should be extensions.
- **Zero external deps** — prefer stdlib over external packages.
- **Static binary** — rho produces a self-contained static binary.
- **No telemetry** — rho does not collect usage data.

## Adding an AI Provider

1. Add provider implementation in `pkg/ai/providers/`
2. Register it in `pkg/ai/providers/registry.go`
3. Add model definitions in `pkg/ai/models.go`
4. Add tests using the `faux` provider as reference

## Adding a Tool

1. Add tool implementation in `pkg/agent/tools/`
2. Register it in `pkg/agent/tools/tools.go`
3. Add tests for the tool

## Adding an Extension Hook

See `pkg/agent/extensions/` for the extension system. Hooks are defined in `pkg/agent/extensions/types.go`.

## Questions?

Open a [Discussion](https://github.com/earendil-works/rho/discussions) or ask on the [pi Discord](https://discord.com/invite/3cU7Bz4UPx).

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
