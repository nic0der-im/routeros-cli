# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Changed
- `ros audit` human output is a compact boxed summary with column-aligned tables and a long `└────` close bar per section; `-o json` keeps full raw maps
- Interface human view shows cumulative **RX/TX** from `rx-byte`/`tx-byte` (GB/MB), labeled as not live Mbps
- SYSTEM block shows memory/storage in MB/GB, bad-blocks, and disk write sectors; optional TOP CPU via `/tool/profile` (`--skip-cpu-profile`, `--cpu-profile-sec`)
- PPPoE/PPP/L2TP interfaces and addresses omitted by default (`--show-ppp` to include); PPP active shown as a count unless `--show-ppp`
- `full` / `security` audit profiles include users + IP services

## [0.3.2] — 2026-07-29

### Added
- SFTP download with **ephemeral SSH allowlist** for `backup binary --output` and `file get`
- Public/local IP detection (`--source-ip`, `--public-ip-url`) and `--ephemeral-ssh` (default true for SFTP)
- Winbox import `--api-port` (default 8728) and `--keep-winbox-port`
- `backup export` via `/export file=` + SFTP when the API stream is empty (common on RouterOS 7)

### Changed
- Default download transport is SFTP (API for small text `contents`; FTP opt-in only)
- Previous SSH `disabled` + `address` are always restored after transfer (success or failure)
- README rewritten (pitch, BETA warning, TOC, focused examples)

## [0.3.1] — 2026-07-29

### Fixed
- `session watch` retries auto-rollback after the link recovers (does not exit on the first failed reconnect)
- Watch reloads device inventory on each probe/rollback
- Omit `creation-time` from remove→add inverse restores

## [0.3.0] — 2026-07-29

### Added
- Safe-session **pre-state journaling** for `set`/`delete` (and curated NAT/lease helpers)
- `ros session watch` heartbeat with auto-rollback + `auto_rollback_pending`
- `ros file get` and `backup binary --output` (API contents or FTP)
- Homebrew tap `nic0der-im/homebrew-tap`; GoReleaser publishes formula updates
- AUR PKGBUILD/`.SRCINFO` with release checksums; Scoop + Chocolatey templates
- `internal/apperr` stable JSON error kinds; richer `diag` + expanded domains
- Opt-in integration tests: `ROS_INTEGRATION_DEVICE=... go test -tags=integration ./test/integration/...`

### Changed
- `recordSafeChange` only journals when session `Safe=true`
- `exec` writes during a safe session require a known inverse or `--force`

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
