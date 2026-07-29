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
| `--raw` | | Include raw RouterOS fields in JSON |
| `-v, --verbose` | | Verbose (reserved) |
| `--no-color` | | Disable color (reserved) |

## Verb + domain model

```
ros -d DEV get|create|set|delete|enable|disable <domain|/path> [params...]
```

- **Curated aliases:** `ros domains`
- **Raw path:** any RouterOS API path, e.g. `/ip/firewall/filter`
- **Params:** `key=value` → `=key=value`; filters `?=key=value`; ids `.id=*N`

Examples:

```sh
ros -d router-edge get user
ros -d router-edge get /ip/firewall/filter
ros -d router-edge get firewall/nat -o json --raw
ros -d router-edge create firewall/address-list list=blacklist address=1.2.3.4
ros -d router-edge set dhcp/server .id=*1 lease-time=1d
ros -d router-edge delete dhcp/lease .id=*F9
ros -d router-edge enable interface .id=*E
ros -d router-edge disable interface/wireguard .id=*E
```

## Device inventory

```
ros device add                          # interactive wizard
ros device add NAME --address host:port --password-stdin   # agentic
ros device auth set <name>
ros device list|get|use|test|remove
ros device discover                     # MNDP neighbors via current device
ros device import --from winbox [--file ...] [--dry-run] [--with-passwords] [--force]
```

Winbox import auto-detects OS paths (macOS/Linux/Windows) for Winbox 4 `Addresses.cdb` and Winbox 3 `.WBX`.

## Diagnostics

```
ros diag log
ros diag ping 1.1.1.1 --count 4
ros diag neighbors
```

## Agent skills

```
ros skills list
ros skills path --agent all --scope user
ros skills install --agent all --scope user          # recommended
ros skills install --agent cursor --scope project --force
ros skills uninstall --agent cursor --scope user
```

Bundled packs: `ros` (read/audit) and `ros-safe-apply` (writes via safe sessions).

## Sessions / agents

```
ros --read-only audit --profile full|network|security
ros session begin|commit|rollback|status|watch
ros file get <name> --output ./local [--via auto|api|ftp]
ros backup binary --file name --output ./dir
ros nat set-out-interface --id '*1' --interface ether1
ros lease cleanup-waiting [--dry-run]
```

### Exit codes / error kinds

| Code | Kind examples |
|------|----------------|
| 0 | OK |
| 1 | `api`, `session`, `not_found` |
| 2 | `connection`, `auth` |
| 3 | `config` |
| 4 | `read_only` |
