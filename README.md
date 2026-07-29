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

`ros` speaks the native RouterOS API (not SSH scraping). It returns clean tables or JSON, keeps passwords in the OS keyring, and ships agent skills so LLMs can audit and change routers safely.

> **BETA warning.** Do **not** use this in production without the right guardrails.
> The rules are clear: prefer `--read-only` / `ROS_READ_ONLY=1`, use **safe sessions** for writes, and start with a RouterOS user that has **read-only** permissions until you trust your agent to mutate state.

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
7. [Safe writes & sessions](#safe-writes--sessions)
8. [Backups](#backups)
9. [AI agents](#ai-agents)
10. [Config & secrets](#config--secrets)
11. [Docs & license](#docs--license)

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
| `create` / `set` / `delete` | Mutate |
| `enable` / `disable` | Toggle |
| `audit` | Multi-domain read-only snapshot |
| `exec` | Raw API escape hatch |

Curated domain aliases (shortcuts to API paths):

```sh
ros domains
```

Examples of aliases: `firewall/filter` → `/ip/firewall/filter`, `radius` → `/radius`, `interface` → `/interface`.

Params: `key=value` becomes RouterOS `=key=value`; target a row with `.id=*1`; filter with `?=disabled=false`.

Full reference: [docs/COMMANDS.md](docs/COMMANDS.md).

---

## Examples

These are the everyday reads you will use most. Device name `router-edge` is a placeholder — use your inventory name.

### Audit (human)

`audit` pulls a structured snapshot so agents (and humans) do not need a full `/export`. Profiles: `full`, `network`, `security`.

Human mode is a **compact boxed summary**: SYSTEM (memory/storage in MB/GB), optional TOP CPU (`/tool/profile`), then column-aligned tables for running interfaces, addresses, routes, DNS, firewall, DHCP, users, and services. Each section closes with a long `└────` bar.

Interface **RX/TX** are cumulative byte counters from the API (`rx-byte` / `tx-byte`), shown as GB/MB — not live Mbps (RouterOS does not expose average rates without sampling).

PPPoE/PPP/L2TP dynamic interfaces (and their addresses) are **hidden by default** so ISP boxes stay readable. Use `--show-ppp` to list them; PPP active sessions appear as a count unless `--show-ppp` is set. Skip the CPU sample with `--skip-cpu-profile` for a faster run.

```sh
ros -d router-edge --read-only audit --profile full
ros -d router-edge --read-only audit --show-ppp           # include PPPoE ifaces / session names
ros -d router-edge --read-only audit --skip-cpu-profile   # skip /tool/profile sample
```

```text
────────────────────────────────────────────────────────
  AUDIT  router-edge  ·  profile=full
────────────────────────────────────────────────────────
┌─ SYSTEM
│  Edge Router
│  CCR2004 · arm64 · 7.18.2 (stable)
│  uptime 1w4d · cpu 6% (4x 1500 MHz · …)
│  memory 1.20 GB free / 4.00 GB total
│  storage 110 MB free / 128 MB total · bad-blocks 0%
└───────────────────────────────────────────────────────

┌─ INTERFACES
│  214 total · RX/TX = cumulative since counter reset (not live Mbps) · 2 shown, 200 ppp/pppoe omitted (--show-ppp)
│  NAME    TYPE    RX         TX        COMMENT
│  ether1  ether   295.34 GB  43.35 GB  WAN
│  bridge  bridge  43.14 GB   292.62 GB LAN
└───────────────────────────────────────────────────────

┌─ PPP ACTIVE
│  200 sessions
│  (names hidden — use --show-ppp to list)
└───────────────────────────────────────────────────────
```

### Audit (JSON)

Same data as a stable envelope for agents: `{ "ok", "data", "meta" }`. Exit code `4` means a read-only violation.

```sh
ros -d router-edge --read-only audit --profile security -o json
```

```json
{
  "ok": true,
  "data": {
    "firewall_filter": [ { ".id": "*1", "chain": "forward", "action": "accept" } ],
    "users": [ { "name": "admin", "group": "full" } ]
  },
  "meta": {
    "device": "router-edge",
    "command": "audit",
    "timestamp": "2026-07-29T05:00:00Z"
  }
}
```

### Interfaces

```sh
ros -d router-edge get interface
```

```text
.ID  NAME     TYPE       RUNNING  DISABLED  COMMENT
*1   ether1   ether      true     false     WAN
*2   bridge   bridge     true     false     LAN
*E   wg-msp   wireguard  true     false     MSP tunnel
```

### Firewall filter rules

```sh
ros -d router-edge get firewall/filter
```

```text
.ID  CHAIN    ACTION                  COMMENT
*1   forward  fasttrack-connection    FastTrack Established/Related
*3   forward  accept                  Accept Established/Related
*A   input    drop                    Drop all other input
```

### RADIUS servers

```sh
ros -d router-edge get radius
```

```text
.ID  SERVICE  ADDRESS        TIMEOUT  COMMENT
*1   login    10.0.0.50      300ms    Central AAA
*2   ppp      10.0.0.50      300ms    Central AAA
```

---

## Safe writes & sessions

For anything that changes the router, prefer a **safe session**. Changes are journaled so you can `rollback` if something goes wrong.

```sh
ros -d router-edge session begin --safe
ros -d router-edge create firewall/filter chain=forward action=accept protocol=tcp dst-port=443
ros -d router-edge session status
ros -d router-edge session commit
# or: ros -d router-edge session rollback
```

`session watch` can heartbeat the link and auto-rollback when connectivity is lost (useful for remote applies).

---

## Backups

**Text export** — writes `/export file=…` on the router and downloads the `.rsc`
(SFTP by default). The API stream is empty on many RouterOS 7 devices; this path works.

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

`ros` ships two skill packs that teach agents the safe workflow (audit first; writes only inside sessions).

```sh
ros skills list
ros skills install --agent all --scope user
```

| Pack | Use |
|------|-----|
| `ros` | Inventory, audit, read-only `get` |
| `ros-safe-apply` | Mutations inside `session begin --safe` |

Recommended agent environment:

```sh
export ROS_READ_ONLY=1
export ROS_DEFAULT_OUTPUT=json
```

Details, exit codes, and JSON error kinds: [docs/AGENTS.md](docs/AGENTS.md).

| Exit | Meaning |
|------|---------|
| 0 | OK |
| 1 | Command / API error |
| 2 | Connection / auth |
| 3 | Config |
| 4 | Read-only violation |

---

## Config & secrets

| What | Where |
|------|--------|
| Inventory | `~/.config/ros/config.toml` |
| Passwords | OS keyring (service name `ros`) |
| Sessions | `~/.config/ros/sessions/` |

Legacy `~/.config/routeros-cli/` is migrated automatically on first run.

---

## Docs & license

| Doc | Purpose |
|-----|---------|
| [docs/COMMANDS.md](docs/COMMANDS.md) | Full command reference |
| [docs/AGENTS.md](docs/AGENTS.md) | Agent / skill workflow |
| [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) | Common failures |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute |

MIT — see [LICENSE](LICENSE).

Maintainer: [nic0der-im](https://github.com/nic0der-im) · [github.com/nic0der-im/routeros-cli](https://github.com/nic0der-im/routeros-cli)
