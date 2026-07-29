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

`ros` (binary; repo **routeros-cli**) talks to MikroTik over the **native API** (8728 / 8729) — not SSH scraping.

| For technicians | For AI agents |
|-----------------|---------------|
| Multi-router inventory by name | `--read-only` + JSON envelope |
| Interactive or scripted device add | Bundled **skills** (`ros skills install`) |
| Safe sessions with rollback | `audit` instead of pasting full `/export` |

```sh
ros -d home --read-only audit -o json
```

---

## Install

```sh
# Linux / macOS
curl -sSL https://raw.githubusercontent.com/nic0der-im/routeros-cli/main/install.sh | sh

# From source
git clone https://github.com/nic0der-im/routeros-cli.git
cd routeros-cli && go build -o ros . && sudo mv ros /usr/local/bin/
```

Requires RouterOS **7.x** with API enabled:

```
/ip/service/set api disabled=no address=192.168.88.0/24
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
echo "$PASS" | ros device add "EOC FRONTERA" \
  --address 10.0.0.1:8728 \
  --username admin \
  --id eoc-frontera \
  --password-stdin
```

### Then

```sh
ros device use "EOC FRONTERA"
ros device test
ros device list
```

| Tip | Detail |
|-----|--------|
| Port `8728` | Plain API (TLS inferred off) |
| Port `8729` | API-SSL (TLS inferred on) |
| Lookup | `-d` accepts **name**, **id**, or **IP** |
| Winbox list | `ros device import --from winbox --dry-run` |

Rotate password later:

```sh
ros device auth set "EOC FRONTERA"    # interactive prompt
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

Examples:

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
ros -d home get system info
ros -d home get firewall/filter -o json --raw
ros -d home get dhcp/lease
ros -d home get user
ros -d home --read-only audit --profile full -o json
```

Profiles: `full` · `network` · `security`

### Write (always prefer a safe session)

```sh
ros -d home session begin --safe

ros -d home create firewall/filter chain=forward action=accept protocol=tcp dst-port=443
ros -d home set dhcp/server .id=*1 lease-time=1d
ros -d home delete dhcp/lease .id=*F9

ros -d home session commit
# or: ros -d home session rollback
```

### Diagnostics

```sh
ros -d home diag log
ros -d home diag ping 1.1.1.1 --count 4
ros -d home diag neighbors
```

### Backup

```sh
ros -d home backup export --file ~/home-$(date +%F).rsc
```

Full reference: [docs/COMMANDS.md](docs/COMMANDS.md)

---

## AI agents & skills

`ros` ships **LLM skills** that teach agents the safe workflow (read-only audit first; writes only via safe sessions).

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

ros -d "EOC FRONTERA" audit --profile full -o json
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

- [x] Multi-device inventory + keyring  
- [x] Verb + domain API surface + `domains`  
- [x] Read-only mode + `audit` + agent skills  
- [x] Safe sessions + Winbox import  
- [ ] Homebrew / AUR publish  
- [ ] Backup binary download to local disk  
- [ ] Heartbeat auto-rollback on link loss  

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
