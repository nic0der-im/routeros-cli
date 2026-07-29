# Changelog

All notable changes to this project will be documented in this file.

## [0.2.0] — 2026-07-29

### Added
- Binary renamed to **`ros`** with `routeros-cli` compatibility alias
- Verb + domain API: `get|create|set|delete|enable|disable` over curated aliases or raw `/path`
- `--read-only` / `ROS_READ_ONLY=1` for agent-safe workflows (exit code 4)
- `ros audit` snapshot profiles (`full`, `network`, `security`)
- Safe sessions: `session begin|commit|rollback|status`
- Interactive `device add` wizard + agentic `--password-stdin` / `ROS_PASSWORD`
- Device lookup by name, id slug, or IP; `device auth set`, `device import` (Winbox 3/4)
- `ros skills install` for Cursor, Codex, Claude, OpenCode (`ros` + `ros-safe-apply` packs)
- `ros domains`, `ros diag` (log/ping/neighbors)
- TLS CA cert loading; port-based TLS inference (8728/8729)
- Config path `~/.config/ros/` with migration from `routeros-cli`
- Docs: COMMANDS, AGENTS, TROUBLESHOOTING; professional README + logo
- CI matrix: Ubuntu + macOS; Go version from `go.mod`

### Fixed
- `exec` empty JSON responses use the standard envelope
- `config.Validate()` enforced on startup

## [0.1.0] — 2026-03-16

### Added
- Initial public release: inventory, system/IP/firewall/DHCP/backup/monitor/exec/schema
