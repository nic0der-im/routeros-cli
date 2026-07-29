<div align="center">

<img src="assets/logo.png" alt="ros — RouterOS CLI" width="520"/>

<br/>

**`ros` — a structured CLI for MikroTik RouterOS, built for engineers and AI agents**

[![CI](https://github.com/nic0der-im/routeros-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/nic0der-im/routeros-cli/actions/workflows/ci.yml)
[![Release](https://github.com/nic0der-im/routeros-cli/actions/workflows/release.yml/badge.svg)](https://github.com/nic0der-im/routeros-cli/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/nic0der-im/routeros-cli)](https://goreportcard.com/report/github.com/nic0der-im/routeros-cli)

[Commands](docs/COMMANDS.md) · [Agents](docs/AGENTS.md) · [Troubleshooting](docs/TROUBLESHOOTING.md) · [Issues](https://github.com/nic0der-im/routeros-cli/issues)

</div>

---

Tired of installing MCP servers that waste context and barely work?
Tired of Winbox because it cannot be automated — while you want your agents to manage *your* MikroTik routers (or your customers')?

**This is the solution.**

`ros` speaks the **native binary RouterOS API** (not SSH scraping, not REST-as-primary). It returns clean tables or JSON, keeps passwords in the OS keyring, and ships **CLI + skill packs** so LLMs can audit and change routers safely — without burning a context window on 100+ MCP tools.

> **Production caution.** Prefer `--read-only` / `ROS_READ_ONLY=1` until you trust the workflow. For writes: `doctor` → `--dry-run` → `session begin --safe` → apply → verify → `commit`|`rollback`. Use a least-privilege RouterOS user. Set `env_class=prod` (and friends) when the box is real customer gear — see [guardrails in COMMANDS](docs/COMMANDS.md).

### What you get (v0.5)

| Area | Highlights |
|------|------------|
| **Change safety** | `--dry-run` + semantic diffs, idempotent write outcomes, safe-session journals, `plan preview\|apply\|rollback` |
| **Production guardrails** | `env_class`, blast-radius / path allow-deny, backup-before-write, exec policy, doctor freshness, maintenance windows, `--confirm` on destructive ops |
| **Observability** | NDJSON write audit, `meta.request_id`, output caps (`--limit` / `ROS_MAX_OUTPUT_BYTES`), read-only retries |
| **Diagnose** | `doctor` / `audit` + FINDINGS, `diag log --topics/--since`, `wg peers --stale-after`, wifi / BGP / OSPF views |
| **Agent fit** | Skill packs **0.5.0**, JSON envelopes, prompt library in [AGENTS.md](docs/AGENTS.md) — CLI contract, not MCP |

---

### Requirements

| Need | Notes |
|------|--------|
| OS | macOS, Windows, or Linux |
| Device | RouterOS-compatible MikroTik (7.x recommended) |
| Reachability | LAN, public IP, or VPN/WireGuard to the router |
| API | Enable `/ip/service` **api** (8728) or **api-ssl** (8729) and grant credentials |

Enable the API on the router (example, LAN-only):

```text
/ip/service/set api disabled=no address=192.168.88.0/24
```

---

## Contents

1. [Install](#install)
2. [Build from source](#build-from-source)
3. [Add a device](#add-a-device)
4. [Import from Winbox](#import-from-winbox)
5. [How commands work](#how-commands-work)
6. [Examples](#examples)
7. [Safe writes, plans & sessions](#safe-writes-plans--sessions)
8. [Backups](#backups)
9. [AI agents](#ai-agents)
10. [FAQ](#faq)
11. [Config & secrets](#config--secrets)
12. [Docs & license](#docs--license)

---

## Install

Pick one path for your OS. The binary is always named **`ros`** (alias `routeros-cli` on some packages).

| Platform | Method |
|----------|--------|
| **macOS** | Homebrew |
| **Windows** | Scoop (or release zip) |
| **Arch** | AUR `routeros-cli-bin` |
| **Ubuntu / other Linux** | `install.sh` or release tarball |

```sh
# macOS
brew tap nic0der-im/tap && brew install ros

# Windows (PowerShell)
scoop bucket add nic0der-im https://github.com/nic0der-im/scoop-bucket
scoop install nic0der-im/ros

# Arch
yay -S routeros-cli-bin

# Ubuntu / generic Linux
curl -sSL https://raw.githubusercontent.com/nic0der-im/routeros-cli/main/install.sh | sh
# optional: INSTALL_DIR="$HOME/.local/bin" curl ... | sh
```

Prebuilt archives for every OS/arch live on the [Releases](https://github.com/nic0der-im/routeros-cli/releases/latest) page (`ros_*_checksums.txt` included).

Verify:

```sh
ros version
```

---

## Build from source

Needs **Go 1.26+** (see `go.mod`).

```sh
git clone https://github.com/nic0der-im/routeros-cli.git
cd routeros-cli
go test ./...
go build -o ros .
./ros version
```

That is enough for local development on macOS, Linux, and Windows (`go build -o ros.exe .`).

---

## Add a device

Passwords go to the **OS keyring** (`ros` service). They are never written to `config.toml`.

**Interactive** (best on a laptop):

```sh
ros device add
```

You will be prompted for name, host, port, username, password, and TLS.

**Scripted / agentic** (stdin password):

```sh
echo "$PASS" | ros device add "central-hub-buenos-aires" \
  --address 10.0.0.1:8728 \
  --username admin \
  --id central-hub-ba \
  --password-stdin
```

Then select, test, and list:

```sh
ros device use "central-hub-buenos-aires"
ros device test
ros device list
```

`-d` accepts **name**, **id**, or **IP**. Port `8728` → plain API; `8729` → API-SSL (TLS inferred).

Rotate a secret later: `ros device auth set <name>`.

Optional per-device production fields in `config.toml` (`env_class`, `maintenance_windows`, write path allow/deny, …) — see [COMMANDS.md](docs/COMMANDS.md).

---

## Import from Winbox

Yes — Winbox import works. `ros` can read your local Winbox address book and turn each entry into an inventory device.

| Source | File |
|--------|------|
| Winbox 4 | `Addresses.cdb` |
| Winbox 3 | `addresses.WBX` |

If you omit `--file`, `ros` auto-detects the default MikroTik/Winbox data directory on macOS, Linux, and Windows (and common Wine paths).

```sh
# Preview only — nothing is written
ros device import --from winbox --dry-run

# Import hosts + usernames; apply RouterOS API port 8728 to every host
# (Winbox stores the GUI port — 8291/… — which is wrong for the API)
ros device import --from winbox

# Also move Winbox passwords into the OS keyring
ros device import --from winbox --with-passwords

# Custom API port for the whole batch (e.g. MSP fleets that listen on 7777)
ros device import --from winbox --with-passwords --api-port 7777

# Rare: keep the port literally stored in Winbox
ros device import --from winbox --keep-winbox-port
```

Important details:

- Default **`--api-port 8728`** replaces whatever port Winbox had. Override per fleet with `--api-port`, or pass `--keep-winbox-port` only when you know the stored port is already the API.
- Without `--with-passwords`, only address + username are imported. Finish with `ros device auth set <name>`.
- Winbox stores secrets in cleartext; `--with-passwords` moves them into the keyring and prints a warning.
- Use `--file /path/to/Addresses.cdb` when the book is not in the default location, and `--force` to refresh an existing inventory name.

---

## How commands work

Shape:

```text
ros -d <DEVICE> <verb> <domain|/raw/path> [params...]
```

| Verb | Meaning |
|------|---------|
| `get` | Read |
| `create` / `set` / `delete` | Mutate (honor `--dry-run`) |
| `enable` / `disable` | Toggle |
| `audit` / `doctor` | Multi-domain snapshot / hygiene FINDINGS |
| `plan` | YAML preview / apply / rollback |
| `exec` | Raw API escape hatch (policy-gated) |

Curated domain aliases (shortcuts to API paths):

```sh
ros domains
```

Examples: `firewall/filter` → `/ip/firewall/filter`, `dns/static` → `/ip/dns/static`, `wg/peers` → `/interface/wireguard/peers`.

Params: `key=value` becomes RouterOS `=key=value`; target a row with `.id=*1` or (filter/mangle) `--comment`; filter with `?=disabled=false` / `get --where`.

Full reference: [docs/COMMANDS.md](docs/COMMANDS.md).

---

## Examples

Device name `router-edge` is a placeholder — use your inventory name.

### Doctor / hygiene FINDINGS

```sh
ros -d router-edge --read-only doctor
ros -d router-edge --read-only audit --profile hygiene
```

`doctor` is the thin hygiene pass agents should run before writes. FINDINGS cover cloud DDNS, backup clutter, iface drops, FastTrack, DHCP lease hygiene, WireGuard peer staleness, netwatch down, DNS static clutter, and more.

### Audit (human)

Profiles: `full`, `network`, `security`, and `hygiene`. Human mode is a compact boxed summary (SYSTEM, interfaces with cumulative RX/TX, routes, firewall, DHCP, …). PPP/PPPoE faces are hidden by default (`--show-ppp` to show). Skip CPU sample with `--skip-cpu-profile`.

```sh
ros -d router-edge --read-only audit --profile full
ros -d router-edge --read-only audit --profile hygiene
```

### Audit (JSON)

Stable envelope for agents: `{ "ok", "data", "meta" }` with `meta.request_id`. Exit code `4` means a read-only violation.

```sh
ros -d router-edge --read-only audit --profile security -o json
```

### Interfaces / firewall / RADIUS

```sh
ros -d router-edge get interface
ros -d router-edge get firewall/filter
ros -d router-edge get radius
```

### WireGuard / logs

```sh
ros -d router-edge wg peers --stale-after 5m
ros -d router-edge diag log --topics error,warning --since 1h --limit 50
```

---

## Safe writes, plans & sessions

**Spine:** `doctor` → mutate `--dry-run` → approve → `session begin --safe` → apply → verify → `commit` or `rollback`.

```sh
ros -d router-edge doctor
ros -d router-edge create firewall/address-list list=blacklist address=203.0.113.10 --dry-run

ros -d router-edge session begin --safe
ros -d router-edge create firewall/address-list list=blacklist address=203.0.113.10
ros -d router-edge session status
ros -d router-edge session commit
# or: ros -d router-edge session rollback
```

- **`--dry-run`** — no write; prints a semantic preview / JSON `action=dry_run`.
- **Safe session** — journals inverses for rollback; prod often requires a local text backup first.
- **`session watch`** — heartbeat + best-effort auto-rollback on link loss (not RouterOS terminal Safe Mode).
- **Destructive ops** (`reboot`, `file remove`, …) need `--confirm <exact-inventory-name>`.
- **YAML plans:**

```sh
ros -d router-edge plan preview --file change.yaml
ros -d router-edge session begin --safe
ros -d router-edge plan apply --file change.yaml
ros -d router-edge plan rollback   # alias of session rollback
```

Details: [COMMANDS.md](docs/COMMANDS.md).

---

## Backups

**Text export** — writes `/export file=…` on the router and downloads the `.rsc` (SFTP by default). The API stream is empty on many RouterOS 7 devices; this path works.

```sh
ros -d router-edge backup export --file ~/router-edge-$(date +%F).rsc

# Already on LAN with SSH allowlisted:
ros -d router-edge backup export --file ~/edge.rsc --ephemeral-ssh=false
```

**Binary backup + local download** (default transport is **SFTP**):

1. Create `.backup` on the router  
2. Detect your local/public IP  
3. Temporarily merge those IPs into `/ip/service ssh` allowlist  
4. Download over SFTP  
5. **Always** restore the previous SSH `disabled` + `address`

```sh
ros -d router-edge backup binary --file nightly --output ~/backups/
```

Already on VPN to the client LAN? Skip the ephemeral open:

```sh
ros -d router-edge backup binary --output ~/backups/ --ephemeral-ssh=false
```

Override detection with `--source-ip`, or pull an existing file with `ros file get <name>`. Prefer SFTP over FTP; `--via api` only works for small text files that expose `contents`.

---

## AI agents

`ros` ships two skill packs (**0.5.0**) that teach the safe workflow (audit/doctor first; writes only inside sessions). **No MCP server** — agents run the CLI.

```sh
ros skills list
ros skills install --agent all --scope user --force   # after upgrading ros
```

| Pack | Use |
|------|-----|
| `ros` | Inventory, audit/doctor, read-only `get`, diagnose |
| `ros-safe-apply` | Mutations inside `session begin --safe` |

Recommended agent environment:

```sh
export ROS_READ_ONLY=1
export ROS_DEFAULT_OUTPUT=json
# optional: ROS_PROFILE=agent   # requires safe session for all writes
```

### Example prompts

```text
Audit router-edge with ros (read-only). Run doctor/hygiene, summarize FINDINGS,
propose changes without applying. Load skill ros only.
```

```text
Using ros-safe-apply on router-edge: doctor, dry-run the firewall change, then
session begin --safe, apply the minimal create/set, verify with --read-only get,
and session commit (or rollback). Unset ROS_READ_ONLY before writes.
```

```text
Show WireGuard peers on router-edge with ros wg peers --stale-after 5m.
Flag peers with empty/old handshakes. Do not delete anything.
```

```text
Preview this YAML plan on router-edge with ros plan preview --file change.yaml.
Explain risks; wait for approval before plan apply under a safe session.
```

Full prompt library, exit codes, and recovery map: [docs/AGENTS.md](docs/AGENTS.md).

| Exit | Meaning |
|------|---------|
| 0 | OK |
| 1 | Command / API error |
| 2 | Connection / auth |
| 3 | Config |
| 4 | Read-only violation |

---

## FAQ

### `ros` vs MCP servers for MikroTik?

MCP tool catalogs burn context and often wrap REST/SSH poorly. `ros` is a **CLI + skill packs** contract: agents call real commands, get stable JSON (`meta.request_id`, write outcomes), and stay inside dry-run / safe-session / guardrail rails. If you want MCP, that is a different product — this repo is intentionally not one.

### `ros` vs raw RouterOS API / Winbox?

Winbox is excellent for humans and terrible for automation. The raw API is powerful but untyped for LLMs. `ros` adds inventory + keyring, curated verbs/aliases, human tables, agent JSON, and change safety on top of the **binary API**.

### `ros` vs SSH scraping?

SSH screen-scraping breaks across versions and locales. `ros` uses the API for structured reads/writes. SSH/SFTP appears only where needed (backup download, ephemeral allowlist). Diagnostics like ping/traceroute still go through the router API tools.

### Is it safe for production?

Safer than improvising `/export` + paste — **if** you set `env_class`, run `doctor`, prefer dry-run + safe sessions, and use least-privilege RouterOS users. Break-glass flags (`--skip-doctor-gate`, `--force-no-backup`, …) are audited/warned for a reason. Start on lab gear (`home`), then promote.

---

## Config & secrets

| What | Where |
|------|--------|
| Inventory | `~/.config/ros/config.toml` |
| Passwords | OS keyring (service name `ros`) |
| Sessions | `~/.config/ros/sessions/` |
| Write audit | `~/.config/ros/audit/writes-YYYY-MM-DD.ndjson` |
| Doctor stamp | `~/.config/ros/state/<device>.doctor` |

Legacy `~/.config/routeros-cli/` is migrated automatically on first run.

---

## Docs & license

| Doc | Purpose |
|-----|---------|
| [docs/COMMANDS.md](docs/COMMANDS.md) | Full command reference + guardrails |
| [docs/AGENTS.md](docs/AGENTS.md) | Skills, prompts, exit codes |
| [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) | Common failures |
| [CHANGELOG.md](CHANGELOG.md) | What shipped (incl. v0.5.0) |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute |

MIT — see [LICENSE](LICENSE).

Maintainer: [nic0der-im](https://github.com/nic0der-im) · [github.com/nic0der-im/routeros-cli](https://github.com/nic0der-im/routeros-cli)
