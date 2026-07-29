# Contributing

Thanks for contributing to `ros` (routeros-cli).

## Development

```sh
git clone https://github.com/nic0der-im/routeros-cli.git
cd routeros-cli
go test ./...
go build -o ros .
./ros version
```

## Guidelines

- Prefer conventional commits: `feat:`, `fix:`, `docs:`, `test:`, `chore:`
- Keep passwords out of flags, config, logs, and tests (use `MemoryStore` in unit tests)
- Read-only and safe-session behavior must not regress — add tests when touching `internal/policy` or `internal/session`
- Update `docs/COMMANDS.md` when adding commands
- Run `go test ./...` before opening a PR

## Pull requests

1. Fork and branch from `main`
2. Keep PRs focused (one feature or fix)
3. Include tests for new behavior
4. Update CHANGELOG `[Unreleased]` section when user-facing

## Code of conduct

Be respectful. This tool manages production network gear — prioritize safety and clarity.
