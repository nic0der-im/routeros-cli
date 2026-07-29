# Command reference

Binary: **`ros`** (alias: `routeros-cli`)

## Global flags

| Flag | Env | Description |
|------|-----|-------------|
| `-d, --device` | | Device name, id, or IP |
| `-o, --output` | `ROS_DEFAULT_OUTPUT` | `table` or `json` |
| `--config` | | Config path (default `~/.config/ros/config.toml`) |
| `--timeout` | | Dial timeout (default `10s`) |
| `--read-only` | `ROS_READ_ONLY=1` | Block all writes |
| | `ROS_READ_RETRIES` | Max attempts for **read-only** API calls (default `3`; `0`/`1` disables). Never retries writes (A5). |
| | `ROS_READ_RETRY_BACKOFF` | Base backoff for read retries (default `75ms`, exp+jitter, cap `1s`) |
| `--skip-doctor-gate` | `ROS_SKIP_DOCTOR_GATE=1` | Break-glass: skip prod doctor freshness gate (also sets `App.Force`, which bypasses maintenance windows) |
| | `ROS_SKIP_MAINTENANCE_GATE=1` | Break-glass: skip `maintenance_windows` write gate |
| `--raw` | | Include raw RouterOS fields in JSON |
| `--limit` | | Max rows for list prints (`0` = unlimited) |
| `-v, --verbose` | | Verbose logs (includes `request_id`) |
| `--no-color` | | Disable color (reserved) |

### Runtime profiles (`ROS_PROFILE`)

| Profile | Default when | Behavior |
|---------|--------------|----------|
| `operator` | (default) | Current behavior; G1 prod/staging gates only |
| `agent` | `ROS_READ_ONLY=1` and profile unset | Writes require an active safe session (even on lab) |
| `agent-strict` | explicit | Agent gates + `ROS_STRICT` semantics (all devices prod-class) |

### Prod write protocol

Successful `ros doctor` / `audit --profile hygiene` records `~/.config/ros/state/<device>.doctor`. On `env_class=prod` (or strict), mutating commands refuse if that timestamp is missing or older than 60m unless `--skip-doctor-gate`, command `--force`, or `ROS_SKIP_DOCTOR_GATE=1`. Staging soft-warns only.

### Destructive confirm (`--confirm`)

Typed device-name gate for high-impact actions. `--confirm NAME` must equal the **resolved inventory device name** (exact match). `--force` does **not** substitute for `--confirm`.

| Command | Notes |
|---------|--------|
| `ros system reboot` | Required; `--force` skips interactive `[y/N]` only |
| `ros file remove` | Required |
| `ros device remove` | Required; `--force` skips interactive `[y/N]` only |
| `ros lease cleanup-waiting` | Required for real deletes; `--dry-run` skips the gate |

Generic single-row `ros delete … .id=*N` does **not** require `--confirm` (too noisy). Deletes without `.id` are refused (no mass-delete selectors).

### Maintenance windows (`maintenance_windows`)

Per-device TOML list. When non-empty, **real writes** are refused outside every listed window (all `env_class` values). Empty / omitted = no restriction. Dry-run never hits this gate.

Times for weekly forms use the **local timezone of the machine running `ros`**.

| Spec | Example |
|------|---------|
| Weekday range + `HH:MM-HH:MM` | `"Mon-Fri 22:00-06:00"` (overnight OK) |
| Weekday list | `"Sat,Sun 00:00-23:59"` |
| Explicit keys | `"weekday=sat,sun;start=00:00;end=23:59"` |
| One-shot RFC3339 range | `"2026-07-29T22:00:00-03:00/2026-07-30T06:00:00-03:00"` (also `..`) |

Break-glass: command `--force`, `--skip-doctor-gate` / `App.Force`, or `ROS_SKIP_MAINTENANCE_GATE=1`.

```toml
[devices.edge]
env_class = "prod"
maintenance_windows = ["Mon-Fri 22:00-06:00", "Sat,Sun 00:00-23:59"]
```

### Output caps

| Setting | Default | Description |
|---------|---------|-------------|
| `ROS_MAX_OUTPUT_BYTES` | `512000` (512 KiB) | Cap rendered table/JSON size; oversize output is truncated with `[OUTPUT TRUNCATED]` and JSON sets `meta.truncated=true` when possible |
| `--limit N` | `0` (unlimited) | Cap number of rows from list prints (`ros get …`, curated gets via `a.render`) |

Every JSON envelope (success and error) includes `meta.request_id` (per CLI invocation). With `-v`, stderr logs are prefixed with that id.

### Write audit trail

Successful, idempotent, and `--dry-run` mutations append one NDJSON line under `~/.config/ros/audit/writes-YYYY-MM-DD.ndjson` (dirs `0700`, files `0600`). Each line includes `ts`, `request_id`, `device`, `profile`, `env_class`, `verb`, `action`/`outcome`, `command`, `path`, redacted `args`/`properties`, and `dry_run`. Known secrets are replaced with `***`. Disable with `ROS_AUDIT=0` (also `ROS_NO_AUDIT=1`). Audit I/O failures never fail the command (logged at `-v` only).

## Verb + domain model

```
ros -d DEV get|create|set|delete|enable|disable <domain|/path> [params...]
```

- **Curated aliases:** `ros domains` lists friendly keys → RouterOS API base paths (get/create/set/delete). Includes short forms agents type often (`wg`, `wg/peers`, `address-list`, `arp`, `dns/static`, `ospf`, `bgp/session`, `netwatch`, `ipv6/*`, wifi registration, …).
- **Raw path:** any RouterOS API path, e.g. `/ip/firewall/filter`
- **Params:** `key=value` → `=key=value`; filters `?=key=value` or `--where key=value`; ids `.id=*N`

Examples:

```sh
ros domains
ros -d router-edge get user
ros -d router-edge get /ip/firewall/filter
ros -d router-edge get interface --where name=ether1
ros -d router-edge get firewall/nat -o json --raw
ros -d router-edge get dns/static
ros -d router-edge get dns static
ros -d router-edge get firewall address-list
ros -d router-edge get wg/peers
ros -d router-edge get ospf/neighbor
ros -d router-edge create firewall/address-list list=blacklist address=1.2.3.4
ros -d router-edge create address-list list=blacklist address=1.2.3.4
ros -d router-edge create firewall address-list --list blacklist --address 1.2.3.4
ros -d router-edge create dns static --name router.lan --address 192.168.88.1
ros -d router-edge set dhcp/server .id=*1 lease-time=1d
ros -d router-edge delete dhcp/lease .id=*F9
ros -d router-edge enable interface .id=*E
ros -d router-edge disable interface/wireguard .id=*E
ros -d router-edge set firewall/filter --comment allow-web disabled=yes
ros -d router-edge delete firewall/filter --comment allow-web
ros -d router-edge enable firewall/mangle --comment mark-conn
```

### Firewall filter/mangle by comment (B5)

Mutations on `/ip/firewall/filter` and `/ip/firewall/mangle` accept **`--comment <exact>`** as a stable alternative to `.id=*N` (exact, case-sensitive match on the `comment` field). Zero matches → error; multiple matches → refused (use unique comments or `.id`). `--dry-run` resolves the comment first, then previews.

| Path | Commands |
|------|----------|
| Curated filter | `ros firewall filter remove\|enable\|disable --id '*N' \| --comment …` |
| kubectl-style | `ros set\|delete\|enable\|disable firewall/filter\|firewall/mangle --comment …` |
| Tree delete | `ros delete firewall filter\|mangle --id … \| --comment …` |

Do **not** use `--comment` as a selector for NAT or address-list (address-list keeps `list`+`address`). On `set`, change the comment *field* with positional `comment=value` after targeting via `--id` / `--comment`.

```sh
ros -d router-edge firewall filter disable --comment allow-web
ros -d router-edge firewall filter remove --comment allow-web --force --dry-run
ros -d router-edge set firewall/filter --comment allow-web action=drop
ros -d router-edge delete firewall/mangle --comment mark-conn
```

### DNS static + address-list (curated)

Friendly domain packs (same paths as aliases; mutations use `apply*Mutation` with `--dry-run` and write outcomes):

| Resource | Idempotent key (`diff.SemanticKey`) | Commands |
|----------|-------------------------------------|----------|
| `/ip/dns/static` | `name` + `type` (default `A`) | `ros dns static list\|add\|set\|remove` |
| `/ip/firewall/address-list` | `list` + `address` | `ros firewall address-list list\|add\|set\|remove` |

```sh
ros -d router-edge dns static list
ros -d router-edge dns static add --name router.lan --address 192.168.88.1
ros -d router-edge dns static add --name router.lan --address 192.168.88.1 --dry-run
ros -d router-edge dns static set --name router.lan --address 192.168.88.2
ros -d router-edge dns static remove --name router.lan

ros -d router-edge firewall address-list list --list blacklist
ros -d router-edge firewall address-list add --list blacklist --address 1.2.3.4
ros -d router-edge firewall address-list remove --list blacklist --address 1.2.3.4
ros -d router-edge firewall address-list set --id '*1' --comment updated
```

Also available via kubectl-style trees: `ros get|create|delete dns static`, `ros get|create|delete firewall address-list`.

## Device inventory

```
ros device add                          # interactive wizard
ros device add NAME --address host:port --password-stdin   # agentic
ros device auth set <name>
ros device list|get|use|test|remove
ros device discover                     # MNDP neighbors via current device
ros device import --from winbox [--file ...] [--dry-run] [--with-passwords] [--force]
               [--api-port 8728] [--keep-winbox-port]
```

Winbox import auto-detects OS paths (macOS/Linux/Windows) for Winbox 4 `Addresses.cdb` and Winbox 3 `.WBX`.
By default `--api-port 8728` is applied to every host (Winbox GUI ports are ignored).

## Diagnostics

```
ros diag log
ros diag log --topics firewall,error
ros diag log --since 15m
ros diag log --topics info,error --since 1h --limit 50
ros diag ping 1.1.1.1 --count 4
ros diag neighbors
```

`diag log` prints `/log/print`. `--topics` keeps rows whose `topics` field contains any
comma-separated token (case-insensitive substring). `--since` keeps entries on or after
`(router clock − duration)` using Go duration syntax (`15m`, `1h`, `2h30m`); the router
clock comes from `/system/clock/print`. Log times without a year are parsed best-effort;
unparseable times are kept when topics match. Row cap is only the global `--limit`
(no local default).

### WireGuard / WiFi / BGP / OSPF (curated reads)

```
ros wg peers
ros wg peers --stale-after 5m
ros interface wireguard peers --stale-after 5m
ros get wg peers --stale-after 5m

ros wifi clients
ros get wifi clients
ros get wifi registration

ros bgp sessions
ros get bgp sessions

ros ospf neighbors
ros get ospf neighbors
```

`wg peers --stale-after` is **read-only**: it lists peers and marks those whose
`last-handshake` is empty/`never`/unparseable or older than the Go duration
(e.g. `5m`, `3m30s`). Human output adds a FINDINGS-style note with the stale
count; peers are never deleted. Handshake ages accept RouterOS `HH:MM:SS`,
Go durations, integer seconds, and absolute datetimes (best-effort).

Domain aliases still work for raw dumps: `ros get wg/peers`, `ros get wifi/registration`,
`ros get bgp/session`, `ros get ospf/neighbor`.

## Agent skills

```
ros skills list
ros skills path --agent all --scope user
ros skills install --agent all --scope user          # recommended
ros skills install --agent cursor --scope project --force
ros skills uninstall --agent cursor --scope user
```

Bundled packs: `ros` (read/audit) and `ros-safe-apply` (writes via safe sessions).

## Plans (YAML preview / apply / rollback)

Multi-step mutations from a YAML file. Plans reuse dry-run/diff previews and the **safe-session journal** — they are not a separate MCP-style change log.

### Schema

```yaml
device: home   # optional; else -d / default device
steps:
  - op: create|set|delete|enable|disable   # unknown ops rejected
    path: /ip/firewall/address-list        # or friendly alias (ros domains)
    props:                                 # create/set
      list: blacklist
      address: 1.2.3.4
    id: "*1"                               # set/delete/enable/disable
    comment: "stable-id"                   # optional; filter/mangle comment-as-ID
```

### Commands

```sh
ros plan preview --file plan.yaml
ros -d home session begin --safe
ros plan apply --file plan.yaml              # creates/sets/enables/disables
ros plan apply --file plan.yaml --confirm home   # required when plan has any delete
ros plan rollback                            # alias of ros session rollback
```

| Command | Behavior |
|---------|----------|
| `preview` | Validate + dry-run each step (no writes / no journal). Human summary + risk notes; `-o json` → `action=plan_preview` envelope. Exit non-zero if any step cannot be previewed. |
| `apply` | Requires an **existing** safe session (`session begin --safe`). Fail-fast; journals via session changes. Refuses `--dry-run` (use preview). `--confirm DEV` when any step is `delete`. |
| `rollback` | Thin alias of `session rollback` (inverse journal on the resolved device). |

## Sessions / agents

```
ros exec /interface/print
# Builtin denylist + per-device exec_allow/exec_deny globs; defense-in-depth, not a hard security boundary if the RouterOS user is full-admin.
ros --read-only audit --profile full|network|security|hygiene [--show-ppp] [--skip-cpu-profile]
# Human: boxed column tables; iface RX/TX = cumulative bytes; PPPoE omitted unless --show-ppp; -o json redacts secrets unless --raw
# hygiene: cloud DDNS, *.backup clutter, iface drops, FastTrack flags, services leftovers, DHCP lease hygiene; soft-fetches WG peers / netwatch / DNS static for FINDINGS (skips CPU sample)
# hygiene/full FINDINGS also: WG stale (>15m/never), netwatch down, DNS static clutter (>50 | ≥5 disabled | duplicate name+type)
ros --read-only doctor
# Alias of audit --profile hygiene with FINDINGS; exit 0 even when warnings present
ros session begin|commit|rollback|status|watch
ros file list
ros file get <name> [--output ./local] [--via sftp|auto|api|ftp] [--ephemeral-ssh]
ros file remove <name-or-id> --confirm DEV
ros backup export [--file ./local.rsc] [--via sftp|auto|api|ftp] [--ephemeral-ssh]
ros backup binary [--file name] [--output ./dir]
    [--via sftp|auto|api|ftp] [--source-ip CIDR] [--ephemeral-ssh]
# Default binary --file is UTC ros-backup-YYYYMMDD-HHMMSS (avoids overwrite)

# Text export: /export file=… on router + SFTP download (API stream is often empty)
ros -d router-edge backup export --file ~/edge.rsc
ros -d router-edge backup export --file ~/edge.rsc --ephemeral-ssh=false   # LAN, SSH already allowlisted

# Binary backup download (default --via sftp):
# 1) /system/backup/save  2) detect local+public IP  3) temporarily merge into
#    /ip/service ssh address  4) SFTP get  5) always restore previous SSH state
ros -d router-edge backup binary --output ~/backups/   # timestamped remote name
ros -d router-edge backup binary --file nightly --output ~/backups/
ros -d router-edge backup binary --output ~/backups/ --ephemeral-ssh=false
ros -d router-edge file list
ros -d router-edge file get nightly.backup --via api   # text files with API contents only
ros -d router-edge file remove stale.backup --confirm router-edge
ros nat set-out-interface --id '*1' --interface ether1
ros lease cleanup-waiting --confirm DEV
ros lease cleanup-waiting --dry-run
ros system reboot --confirm DEV [--force]
ros device remove DEV --confirm DEV [--force]
```

### Exit codes / error kinds

| Code | Kind examples |
|------|----------------|
| 0 | OK |
| 1 | `api`, `session`, `not_found`, `conflict`, `busy`, `internal` |
| 2 | `connection`, `auth`, `timeout` |
| 3 | `config` |
| 4 | `read_only` |

JSON errors may include `error.suggested_action` (e.g. after an ambiguous write timeout: verify with read-only get before retry).

### Read retries

`client.Connect` wraps the live client with automatic retries for **non-mutating** commands only (`/print`, `/get`, `/listen`, and any path that is not a known write verb). Transient failures (timeout, EOF, connection reset, temporary network errors, `busy`) are retried with exponential backoff + jitter (defaults: 3 attempts, base `75ms`, cap `1s`; override with `ROS_READ_RETRIES` / `ROS_READ_RETRY_BACKOFF`). Mutating commands (`/add`, `/set`, `/remove`, …) are **never** auto-retried after dispatch — ambiguous-write policy A5 remains: on timeout/EOF after a write was sent, verify with a read before re-running. Soft in-memory per-address backoff applies after recent failures. `MockClient` is unaffected unless wrapped with `WithReadRetries`.

Successful mutate JSON sets `meta.action` (and usually `data.action`) to one of:
`created` | `updated` | `removed` | `already_exists` | `no_change` | `dry_run`.
