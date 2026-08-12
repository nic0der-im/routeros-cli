# Changelog

All notable changes to this project will be documented in this file.

## [0.7.0] — 2026-08-12

Usability and cleanup cut: device-name shell completion across the CLI, and a
`device remove` that actually removes every local trace of a device.

### Added
- Shell completion of device names for `-d/--device` and for the `<name>` argument of `device remove|use|test|get` and `device auth set` (falls back to device ids/addresses when no name matches)
- `device remove --purge-backups` to also delete `~/.config/ros/backups/<device>/`
- `device remove` aliases `delete` and `rm`

### Fixed
- Device tab completion produced nothing: no `RegisterFlagCompletionFunc` on `-d/--device` and no `ValidArgsFunction` on any device command, so every shell fell back to filename completion
- `device remove` now purges all per-device local state: doctor freshness state (`~/.config/ros/state/<device>.doctor`) and safe-session locks and journals, which previously leaked after deletion
- `device remove` refuses to delete a device with an active safe session (bypass with `--force`), so pending changes are no longer orphaned without a rollback path
- `RunE` command errors were swallowed entirely (`SilenceErrors` with no printer in `main`); they now print to stderr

## [0.6.0] — 2026-08-07

### Added
- Secure `--password-stdin` support for generic `create user` and `set user` mutations with fail-closed input validation.

### Changed
- README: v0.5 feature showcase, safe-write spine, agent prompts, FAQ vs MCP/SSH/Winbox

## [0.5.0] — 2026-07-29

Enterprise cut: write-safety (A), production guardrails (G), observability (C), curated surface (B), YAML plans (D), and skill packs **0.5.0** (E). Live smoke on lab `home` (doctor, dry-run, session apply+rollback, backup export, plan preview/apply+rollback, read-only denial). Planned 0.6/0.7 feature slices shipped here in one release.

### Added
- `internal/diff` + global `--dry-run` on create/set/delete/enable/disable; JSON write outcomes `created|updated|removed|already_exists|no_change|dry_run`
- `apperr` kinds `conflict`/`timeout`/`busy` + `suggested_action`; ambiguous-write policy (no auto-retry after mutating send)
- Production guardrails: `env_class` prod|staging|lab + `ROS_STRICT`; safe-session gates; `max_session_changes`; path allow/deny; prod backup-before-write on `session begin --safe`; exec denylist + `exec_allow`/`exec_deny`; `ROS_PROFILE` operator|agent|agent-strict; doctor freshness gate; reboot `--confirm`
- Output caps: `ROS_MAX_OUTPUT_BYTES` (default **512000**), global `--limit N`; `[OUTPUT TRUNCATED]` / `meta.truncated=true`
- `meta.request_id` on JSON envelopes; `-v` logs include the id
- NDJSON write audit `~/.config/ros/audit/writes-YYYY-MM-DD.ndjson` (redacted; `ROS_AUDIT=0` disables)
- `maintenance_windows` on devices; refuse writes outside window unless force / `ROS_SKIP_MAINTENANCE_GATE=1`
- Destructive `--confirm <exact-inventory-name>` on reboot, `file remove`, `device remove`, `lease cleanup-waiting`
- Read-only API retries with backoff (`ROS_READ_RETRIES` / `ROS_READ_RETRY_BACKOFF`); writes never retried
- Domain aliases: dns/static, arp, netwatch, routing/table, bgp/session, ospf*, wifi registration, wg/peers, address-list, ipv6/*, …
- Curated `dns static` + `firewall address-list` list/add/set/remove (idempotent keys + `--dry-run`)
- `wg peers --stale-after`, `wifi clients`, `bgp sessions`, `ospf neighbors`
- `diag log --topics` / `--since` (router clock)
- Filter/mangle mutate by `--comment` as stable ID
- FINDINGS: WG stale, netwatch down, DNS static clutter
- `ros plan preview|apply|rollback` (YAML plans on safe sessions)
- Skill packs **0.5.0**: `safety-and-recovery.md`, `routeros-docs.md`, diag/doctor gates, AGENTS prompt library, lockstep tests

### Changed
- Bundled agent skill metadata version **0.5.0**

## [0.4.0] — 2026-07-29

### Added
- `audit --profile hygiene` (cloud, backup files, iface drops, FastTrack flags, DHCP lease hygiene) with **FINDINGS** footer; `ros doctor` alias
- `file list` / `file remove`; timestamped default `backup binary` remote name
- Domain aliases for cloud, logging, scheduler, health, ip/settings, firewall connection, discovery-settings, bandwidth-server, `interface/list`, `interface/list/member`
- `get --where key=value` (repeatable, curated + generic) → RouterOS `?key=value` query filters
- Skill packs **0.4.0** (hygiene checklist, ROS ≥7.17 cloud `yes|auto`, secret caution)

### Changed
- Strip trailing `/print`|/get` on get/set/delete path normalize
- Redact known secret fields in table, default JSON, and audit `-o json` (`--raw` shows secrets)
- `set ip/cloud`: reject `ddns-enabled=no`; normalize `ddns-enabled=false` → `auto` with stderr tip
- Safe-session journals singleton `/set` (e.g. `/ip/cloud`) without `.id`

## [0.3.3] — 2026-07-29

### Changed
- `ros audit` human output is a compact boxed summary with column-aligned tables and a long `└────` close bar per section; `-o json` keeps full raw maps
- Interface human view shows cumulative **RX/TX** from `rx-byte`/`tx-byte` (GB/MB), labeled as not live Mbps
- SYSTEM block shows memory/storage in MB/GB, bad-blocks, and disk write sectors; optional TOP CPU via `/tool/profile` (`--skip-cpu-profile`, `--cpu-profile-sec`)
- PPPoE/PPP/L2TP interfaces and addresses omitted by default (`--show-ppp` to include); PPP active shown as a count unless `--show-ppp`
- `full` / `security` audit profiles include users + IP services
- Read-only policy allows `/tool/profile` for audit CPU sampling

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
