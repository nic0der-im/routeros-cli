<div align="center">

<img src="assets/logo.png" alt="routeros-cli" width="520"/>

<br/>

**A fast, structured CLI for MikroTik RouterOS — for network engineers and AI agents**

[![CI](https://github.com/nic0der-im/routeros-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/nic0der-im/routeros-cli/actions/workflows/ci.yml)
[![Release](https://github.com/nic0der-im/routeros-cli/actions/workflows/release.yml/badge.svg)](https://github.com/nic0der-im/routeros-cli/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/nic0der-im/routeros-cli)](https://goreportcard.com/report/github.com/nic0der-im/routeros-cli)

[Docs](docs/COMMANDS.md) · [Agents](docs/AGENTS.md) · [Troubleshooting](docs/TROUBLESHOOTING.md) · [Issues](https://github.com/nic0der-im/routeros-cli/issues)

</div>

---

## What it is

`ros` (routeros-cli macro) talks to MikroTik RouterOS devices over the native API (8728 / 8729 by default).

It is **not** SSH scraping — it speaks the binary RouterOS API, returns tables or JSON, and is safe for humans and agents.

| For technicians | For AI agents |
|-----------------|---------------|
| Multi-router inventory by name | `--read-only` + JSON envelope |
| Interactive or scripted device add | Bundled **skills** (`ros skills install`) |
| Safe sessions with rollback | `audit` instead of pasting full `/export` |

```sh
ros -d router-edge --read-only audit -o json
```

```text
{
  "ok": true,
  "data": {
    "firewall_filter": [ { ".id": "*1", "chain": "forward", "action": "fasttrack-connection", ... } ],
    ...
  },
  "meta": { "device": "router-edge", "command": "audit", "timestamp": "..." }
}
```

---

## Install

### Pre-built binary (recommended)

**Homebrew (macOS / Linux)**

```sh
brew tap nic0der-im/tap
brew install ros
```

**Linux / macOS (install.sh)**

```sh
curl -sSL https://raw.githubusercontent.com/nic0der-im/routeros-cli/main/install.sh | sh
```

**Windows** — download the zip for your arch from the [latest release](https://github.com/nic0der-im/routeros-cli/releases/latest), extract `ros.exe`, and put it on your `PATH`.

**Scoop (Windows)**

```powershell
scoop bucket add nic0der-im https://github.com/nic0der-im/scoop-bucket
scoop install nic0der-im/ros
```

**AUR (Arch)**

```sh
yay -S routeros-cli-bin
# or: paru -S routeros-cli-bin
```

**Chocolatey (Windows)** — package template under [`chocolatey/`](chocolatey/); publish to chocolatey.org when ready.

| OS | Arch | Asset |
|----|------|-------|
| macOS | Apple Silicon | `ros_*_darwin_arm64.tar.gz` |
| macOS | Intel | `ros_*_darwin_amd64.tar.gz` |
| Linux | x86_64 | `ros_*_linux_amd64.tar.gz` |
| Linux | aarch64 | `ros_*_linux_arm64.tar.gz` |
| Windows | x86_64 | `ros_*_windows_amd64.zip` |
| Windows | ARM64 | `ros_*_windows_arm64.zip` |

Checksums ship as `ros_*_checksums.txt` in the same release.

Requires RouterOS **7.x** with API enabled:

```
/ip/service/set api disabled=no address=192.168.88.0/24
```

For API-SSL (8729):

```
/ip/service/set api-ssl disabled=no
```

---

## Build from source

Needs **Go 1.26+** (see `go.mod`). The module root builds a single binary named `ros`.

### All platforms (happy path)

```sh
git clone https://github.com/nic0der-im/routeros-cli.git
cd routeros-cli
go test ./...
go build -o ros .
./ros version
```

Optional version stamping (same as release builds):

```sh
go build -ldflags "-s -w -X main.version=0.2.0 -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o ros .
```

Or without cloning:

```sh
go install github.com/nic0der-im/routeros-cli@latest
# installs as $(go env GOPATH)/bin/routeros-cli (module path name)
# symlink if you want the short name:
# ln -sf "$(go env GOPATH)/bin/routeros-cli" "$(go env GOPATH)/bin/ros"
```

`install.sh` respects `INSTALL_DIR` (default `/usr/local/bin`):

```sh
curl -sSL https://raw.githubusercontent.com/nic0der-im/routeros-cli/main/install.sh | INSTALL_DIR="$HOME/.local/bin" sh
```

### macOS

```sh
# Apple Silicon or Intel — Go from Homebrew is fine
brew install go
git clone https://github.com/nic0der-im/routeros-cli.git
cd routeros-cli
go build -o ros .
sudo install -m 755 ros /usr/local/bin/ros
# or: mkdir -p ~/bin && mv ros ~/bin && echo 'export PATH="$HOME/bin:$PATH"' >> ~/.zshrc
```

Cross-compile from macOS if needed:

```sh
GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -o ros-linux-amd64 .
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o ros.exe .
```

### Linux

```sh
# Debian/Ubuntu
sudo apt update && sudo apt install -y golang-go git
# Fedora
# sudo dnf install golang git
# Arch
# sudo pacman -S go git

git clone https://github.com/nic0der-im/routeros-cli.git
cd routeros-cli
go build -o ros .
sudo install -m 755 ros /usr/local/bin/ros
sudo ln -sf /usr/local/bin/ros /usr/local/bin/routeros-cli   # optional legacy alias
```

Static binary (useful for containers / minimal hosts):

```sh
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o ros .
```

### Windows (PowerShell)

```powershell
# Install Go from https://go.dev/dl/ then:
git clone https://github.com/nic0der-im/routeros-cli.git
cd routeros-cli
go test ./...
go build -o ros.exe .
# Put ros.exe somewhere on PATH, e.g.:
# Copy-Item .\ros.exe $env:USERPROFILE\bin\
```

Cross-compile from Windows to Linux:

```powershell
$env:CGO_ENABLED=0; $env:GOOS="linux"; $env:GOARCH="amd64"; go build -o ros-linux-amd64 .
```

### Verify

```sh
ros version
ros device list
```

```text
ros 0.2.0          # "dev" when built without ldflags
  commit: abc1234
  built:  2026-07-29T04:40:00Z
```

### Shell completions

Cobra ships `ros completion` for bash, zsh, fish, and PowerShell:

```sh
# zsh (Homebrew formula also installs completions automatically)
ros completion zsh > "${fpath[1]}/_ros"

# bash
ros completion bash > /usr/local/etc/bash_completion.d/ros

# fish
ros completion fish > ~/.config/fish/completions/ros.fish

# powershell
ros completion powershell | Out-String | Invoke-Expression
```

---

## Quick start — add a router

Passwords go to the **OS keyring**, never to `config.toml`.

### Interactive (recommended on your laptop)

```sh
ros device add
```

Prompts for: name, host, port, username, password, TLS, optional id/tags.

### Agentic / scripted (pipes & secrets)

```sh
echo "$PASS" | ros device add "central-hub-buenos-aires" \
  --address 10.0.0.1:8728 \
  --username admin \
  --id central-hub-ba \
  --password-stdin
```

### Then

```sh
ros device use "central-hub-buenos-aires"
ros device test
ros device list
```

```text
Connected to "central-hub-buenos-aires" (identity: central-hub-buenos-aires)

DEFAULT  NAME                         ID               ADDRESS              USERNAME  TLS
*        central-hub-buenos-aires     central-hub-ba   10.0.0.1:8728        admin     false
         router-edge                  router-edge      192.168.88.1:8728    admin     false
         edge-node-west               edge-west        10.10.20.1:8728      admin     false
```

| Tip | Detail |
|-----|--------|
| Port `8728` | Plain API (TLS inferred off) |
| Port `8729` | API-SSL (TLS inferred on) |
| Lookup | `-d` accepts **name**, **id**, or **IP** |
| Winbox list | `ros device import --from winbox --dry-run` |

Rotate password later:

```sh
ros device auth set "central-hub-buenos-aires"    # interactive prompt
```

---

## How commands are grouped

Everything follows:

```text
ros -d <DEVICE> <VERB> <DOMAIN|/path> [params...]
```

### Verbs

| Verb | Use |
|------|-----|
| `get` | Read |
| `create` | Add |
| `set` | Update |
| `delete` | Remove (needs `.id=*N`) |
| `enable` / `disable` | Toggle |
| `audit` | Read-only multi-domain snapshot |
| `session` | Safe apply journal |
| `diag` | log / ping / neighbors |
| `exec` | Raw API escape hatch |

### Domains (curated aliases)

```sh
ros domains
```

```text
dhcp/lease                 → /ip/dhcp-server/lease
firewall/filter            → /ip/firewall/filter
firewall/nat               → /ip/firewall/nat
interface/wireguard        → /interface/wireguard
ip/address                 → /ip/address
user                       → /user
...
```

| Domain | API path |
|--------|----------|
| `firewall/filter` | `/ip/firewall/filter` |
| `firewall/nat` | `/ip/firewall/nat` |
| `dhcp/lease` | `/ip/dhcp-server/lease` |
| `user` | `/user` |
| `radius` | `/radius` |
| `interface/bridge` | `/interface/bridge` |

Raw paths always work: `ros get /ip/firewall/address-list`

### Params

```text
key=value     →  =key=value
.id=*1        →  target row
?=disabled=false   →  query filter
```

---

## Everyday examples

### Read

```sh
ros -d router-edge get system info
```

```text
IDENTITY         BOARD    PLATFORM  VERSION          UPTIME         CPU LOAD  MEMORY FREE/TOTAL
Edge Router Lab  CCR2004  MikroTik  7.18.2 (stable)  1w4d6h36m30s   6%        27738112/67108864
```

```sh
ros -d router-edge get ip/address
```

```text
.ID  NETWORK         INTERFACE    ADDRESS              DYNAMIC  DISABLED
*2   192.168.88.0    dhcpSwitch   192.168.88.1/24      false    false
*67  100.68.176.0    ether1       100.68.178.110/20    true     false
```

```sh
ros -d router-edge get firewall/filter
```

```text
DYNAMIC  COMMENT                         .ID  CHAIN    ACTION                  BYTES         PACKETS
false    FastTrack Established/Related   *1   forward  fasttrack-connection    2273138543    12394696
false    Accept Established/Related      *3   forward  accept                  2273138543    12394696
false    Drop all other input            *A   input    drop                    38249427      269466
...
```

```sh
ros -d router-edge get dhcp/lease
```

```text
ACTIVE-ADDRESS  HOST-NAME    COMMENT      ADDRESS         MAC-ADDRESS        STATUS  SERVER
192.168.88.29   laptop-ops   Ops laptop   192.168.88.29   FC:B2:14:81:B3:AD  bound   dhcpNetwork
192.168.88.39   lab-server   Lab server   192.168.88.39   E0:B9:A5:D5:18:19  bound   dhcpNetwork
...
```

```sh
ros -d router-edge get user
```

```text
GROUP  DISABLED  .ID  NAME   LAST-LOGGED-IN
full   false     *2   admin  2026-07-29 01:46:52
```

```sh
ros -d router-edge get system info -o json
```

```json
{
  "ok": true,
  "data": [
    {
      "Identity": "Edge Router Lab",
      "Board": "CCR2004",
      "Version": "7.18.2 (stable)",
      "Uptime": "1w4d6h36m40s",
      "CPU Load": "5%"
    }
  ],
  "meta": {
    "device": "router-edge",
    "command": "/system/resource/print",
    "count": 1
  }
}
```

```sh
ros -d router-edge --read-only audit --profile network
```

```text
Audit of "router-edge" (profile=network)
  interfaces:          14 item(s)
  ip_addresses:        3 item(s)
  ip_routes:           3 item(s)
  dns:                 1 item(s)
  dhcp_leases:         8 item(s)
  dhcp_servers:        1 item(s)
```

Profiles: `full` · `network` · `security`

### Write (always prefer a safe session)

```sh
ros -d router-edge session begin --safe
```

```text
Session 1785300485258875000 started on "router-edge" (safe=true)
```

```sh
ros -d router-edge create firewall/filter chain=forward action=accept protocol=tcp dst-port=443
ros -d router-edge set dhcp/server .id=*1 lease-time=1d
ros -d router-edge delete dhcp/lease .id=*F9

ros -d router-edge session status
```

```text
Session 1785300485258875000
  Device:     router-edge
  Status:     active
  Safe:       true
  Started:    2026-07-29T04:48:05Z
  Updated:    2026-07-29T04:48:05Z
  Changes:    3
```

```sh
ros -d router-edge session commit
# or: ros -d router-edge session rollback
```

```text
Session 1785300485258875000 committed on "router-edge" (3 change(s))
```

### Diagnostics

```sh
ros -d router-edge diag ping 1.1.1.1 --count 3
```

```text
SENT  RECEIVED  PACKET-LOSS  AVG-RTT    SEQ  HOST     SIZE  MIN-RTT    MAX-RTT    TTL  TIME
1     1         0            30ms808us  0    1.1.1.1  56    30ms808us  30ms808us  58   30ms808us
2     2         0            30ms482us  1    1.1.1.1  56    30ms157us  30ms808us  58   30ms157us
3     3         0            30ms411us  2    1.1.1.1  56    30ms157us  30ms808us  58   30ms268us
```

```sh
ros -d router-edge diag log
```

```text
.ID  TIME                 TOPICS                 MESSAGE
*0   2026-07-17 02:04:45  system,error,critical  router was rebooted without proper shutdown
*3   2026-07-17 02:05:03  dhcp,info              dhcp-client on ether1 got IP address 100.68.178.110
...
```

```sh
ros -d router-edge diag neighbors
```

### Backup

```sh
ros -d router-edge backup export --file ~/router-edge-$(date +%F).rsc
```

```text
Configuration exported to "/Users/you/router-edge-2026-07-29.rsc" from "router-edge"
```

```sh
# Creates .backup on the router only (local download is still on the roadmap)
ros -d router-edge backup binary --file routeros-cli-backup
```

```text
Backup "routeros-cli-backup.backup" created on "router-edge" (size: 123456)
```

Full reference: [docs/COMMANDS.md](docs/COMMANDS.md)

---

## AI agents & skills

`ros` ships **LLM skills** that teach agents the safe workflow (read-only audit first; writes only via safe sessions).

```sh
ros skills list
```

```text
ros
ros-safe-apply
```

```sh
# Install into Cursor, Codex, Claude Code, OpenCode
ros skills install --agent all --scope user
```

| Pack | Purpose |
|------|---------|
| `ros` | Inventory, audit, read-only `get` |
| `ros-safe-apply` | Firewall / DHCP / users / etc. inside `session begin` |

```sh
export ROS_READ_ONLY=1
export ROS_DEFAULT_OUTPUT=json

ros -d "central-hub-buenos-aires" audit --profile full -o json
```

Details: [docs/AGENTS.md](docs/AGENTS.md)

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | OK |
| 1 | Command / API error |
| 2 | Connection / auth |
| 3 | Config |
| 4 | Read-only violation |

JSON shape: `{ "ok", "data" \| "error", "meta" }` — add `--raw` for `.id` fields.

---

## Config & secrets

| What | Where |
|------|--------|
| Inventory | `~/.config/ros/config.toml` |
| Passwords | OS Keychain (`ros` service) |
| Sessions | `~/.config/ros/sessions/` |

Legacy `~/.config/routeros-cli/` is migrated automatically.

---

## Roadmap

### Done (v0.2.0)

- [x] Binary rename to `ros` (+ `routeros-cli` alias) and config migration to `~/.config/ros/`
- [x] Multi-device inventory + OS keyring (never passwords in TOML)
- [x] Verb + domain API surface + `ros domains` curated aliases
- [x] `--read-only` / `ROS_READ_ONLY=1`, exit code 4, JSON envelope
- [x] `ros audit` profiles (`full` / `network` / `security`)
- [x] Safe sessions (`begin` / `commit` / `rollback` / `status`) with best-effort inverse journal
- [x] Winbox import (v3 `.WBX` / v4 `Addresses.cdb` on macOS, Linux, Windows)
- [x] Bundled agent skills + `ros skills install|uninstall|list|path`
- [x] Diagnostics (`diag log|ping|neighbors`), text `backup export`
- [x] Cross-platform release assets (linux/darwin/windows × amd64/arm64)

### Done (v0.3.0)

- [x] Pre-state journaling for `set` / `delete` (+ curated helpers)
- [x] `session watch` heartbeat + auto-rollback on link loss (`auto_rollback_pending`)
- [x] `backup binary --output` + `file get` (API contents or FTP)
- [x] Homebrew tap (`nic0der-im/homebrew-tap`) + GoReleaser `brews`
- [x] AUR PKGBUILD/`.SRCINFO` with real checksums (live AUR push is maintainer-side)
- [x] Completions documented + Homebrew formula installs them
- [x] Stable `apperr` kinds in JSON `error.code`
- [x] Expanded domains + `nat` / `lease` helpers + richer `diag`
- [x] Opt-in integration tests (`ROS_INTEGRATION_DEVICE`)
- [x] Scoop + Chocolatey package templates

### Still maintainer-side / ongoing

- [x] Push AUR package live (`routeros-cli-bin` on aur.archlinux.org) — v0.3.0
- [x] Publish Scoop bucket (`nic0der-im/scoop-bucket`)
- [ ] Publish Chocolatey.org package from [`chocolatey/`](chocolatey/) template
- [ ] Expand capa linda further as field needs appear
- [ ] Harden skill packs after more production apply workflows

### Not planned soon

- Emulating Winbox GUI parity end-to-end
- Storing passwords in config files or CLI flags
- Targeting RouterOS 6.x as a first-class platform

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). PRs welcome.

```sh
go test ./...
go build -o ros .
```

## License

MIT — see [LICENSE](LICENSE).

## Contact

[nic0der-im](https://github.com/nic0der-im) · [github.com/nic0der-im/routeros-cli](https://github.com/nic0der-im/routeros-cli)
