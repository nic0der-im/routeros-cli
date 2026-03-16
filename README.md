# routeros-cli

A fast, structured CLI for managing MikroTik RouterOS routers. Built for network engineers and AI agents.

[![CI](https://github.com/nic0der-im/routeros-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/nic0der-im/routeros-cli/actions/workflows/ci.yml)
[![Release](https://github.com/nic0der-im/routeros-cli/actions/workflows/release.yml/badge.svg)](https://github.com/nic0der-im/routeros-cli/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/nic0der-im/routeros-cli)](https://goreportcard.com/report/github.com/nic0der-im/routeros-cli)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

```
$ routeros-cli system info
IDENTITY    BOARD       PLATFORM  VERSION          UPTIME       CPU LOAD  MEMORY FREE/TOTAL    HDD FREE/TOTAL
MikroTik    RB5009UG+S  MikroTik  7.18.2 (stable)  45d12h30m    8%       768000000/1073741824  900000000/1073741824

$ routeros-cli -o json dhcp lease list --active
{
  "ok": true,
  "data": [
    { "Address": "192.168.88.254", "MAC Address": "34:60:F9:0C:00:28", "Host Name": "ArcherAX10", "Status": "bound" },
    { "Address": "192.168.88.51",  "MAC Address": "A8:80:55:9E:98:C4", "Host Name": "wlan0",      "Status": "bound" }
  ],
  "meta": { "device": "home-rb", "command": "/ip/dhcp-server/lease/print", "timestamp": "2026-03-16T12:00:00Z", "count": 2 }
}
```

## Features

- **Multi-device inventory** — manage multiple routers from a single config
- **Dual output** — human-readable tables (default) or structured JSON for scripting/AI
- **Native RouterOS API** — uses the binary API protocol (port 8728/8729), not SSH scraping
- **Secure credentials** — passwords stored in your OS keyring (GNOME Keyring, macOS Keychain, Windows Credential Manager)
- **Full CRUD** — interfaces, IP addresses, routes, DNS, firewall rules, DHCP, backups
- **Real-time monitoring** — poll traffic and resource stats with configurable intervals
- **AI-ready** — stable JSON schema with `{ok, data, meta}` envelope, deterministic exit codes, `schema` command for self-describing output
- **Cross-platform** — single binary for Linux, macOS, and Windows (amd64/arm64)

## Installation

### Quick install (Linux/macOS)

```sh
curl -sSL https://raw.githubusercontent.com/nic0der-im/routeros-cli/main/install.sh | sh
```

### Homebrew

```sh
brew install nic0der-im/tap/routeros-cli
```

### Go install

```sh
go install github.com/nic0der-im/routeros-cli@latest
```

### From source

```sh
git clone https://github.com/nic0der-im/routeros-cli.git
cd routeros-cli
go build -o routeros-cli .
sudo mv routeros-cli /usr/local/bin/
```

### Arch Linux (AUR)

```sh
yay -S routeros-cli-bin
```

### GitHub Releases

Download pre-built binaries from [Releases](https://github.com/nic0der-im/routeros-cli/releases).

## Quick Start

### 1. Enable the API on your router

The CLI uses the RouterOS native API (not SSH). Enable it on your router:

```
# Via SSH or Winbox terminal:
/ip/service/set api disabled=no address=192.168.88.0/24

# For TLS (recommended):
/ip/service/set api-ssl disabled=no address=192.168.88.0/24
```

> **Note:** The API service must have a certificate assigned for TLS to work. If using a self-signed cert, set `insecure_skip_verify = true` in the config.

### 2. Add your router

```sh
# Add a device (password is read from stdin, never from a flag)
echo 'your-password' | routeros-cli device add home \
  --address 192.168.88.1:8728 \
  --username admin \
  --tls=false \
  --password-stdin

# Set it as default
routeros-cli device use home

# Verify connectivity
routeros-cli device test
# Connected to "home" (identity: MikroTik)
```

### 3. Start managing

```sh
routeros-cli system info
routeros-cli interface list
routeros-cli ip address list
routeros-cli firewall filter list
routeros-cli dhcp lease list --active
```

## Command Reference

### Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--device` | `-d` | Target device from inventory (overrides default) |
| `--output` | `-o` | Output format: `table` (default) or `json` |
| `--config` | | Config file path (default: `~/.config/routeros-cli/config.toml`) |
| `--timeout` | | Connection timeout (default: `10s`) |
| `--verbose` | `-v` | Verbose output |
| `--no-color` | | Disable color output |

### Device Management

```sh
# Add devices
echo 'password' | routeros-cli device add home --address 192.168.88.1:8728 --username admin --tls=false --password-stdin
echo 'password' | routeros-cli device add office --address 10.0.1.1:8729 --username api-user --password-stdin

# List all devices
routeros-cli device list
# DEFAULT  NAME    ADDRESS            USERNAME  TLS
# *        home    192.168.88.1:8728  admin     false
#          office  10.0.1.1:8729      api-user  true

# Set default device
routeros-cli device use office

# Test connectivity
routeros-cli device test          # tests default device
routeros-cli device test home     # tests specific device

# Remove a device
routeros-cli device remove office --force
```

### System

```sh
# Combined identity + resource summary
routeros-cli system info

# Get/set identity
routeros-cli system identity
routeros-cli system identity --set "Office-Router"

# Detailed resources
routeros-cli system resource

# Reboot (asks confirmation unless --force)
routeros-cli system reboot
routeros-cli system reboot --force
```

### Interfaces

```sh
# List all interfaces
routeros-cli interface list

# Filter by type
routeros-cli interface list --type ether
routeros-cli interface list --type bridge

# Enable/disable
routeros-cli interface enable ether5
routeros-cli interface disable ether5

# Monitor traffic in real time (Ctrl+C to stop)
routeros-cli interface monitor ether1
routeros-cli interface monitor ether1 --interval 2s
```

### IP Management

```sh
# Addresses
routeros-cli ip address list
routeros-cli ip address list --interface ether1
routeros-cli ip address add --address 10.0.0.1/24 --interface ether3 --comment "Management"
routeros-cli ip address remove '*5'

# Routes
routeros-cli ip route list
routeros-cli ip route add --dst-address 10.10.0.0/16 --gateway 192.168.88.254 --distance 1
routeros-cli ip route remove '*A'

# DNS
routeros-cli ip dns get
routeros-cli ip dns set --servers 1.1.1.1,8.8.8.8
```

### Firewall

```sh
# Filter rules
routeros-cli firewall filter list
routeros-cli firewall filter add --chain input --action accept --protocol tcp --dst-port 443 --comment "Allow HTTPS"
routeros-cli firewall filter enable '*5'
routeros-cli firewall filter disable '*5'
routeros-cli firewall filter remove '*5' --force

# NAT rules
routeros-cli firewall nat list
routeros-cli firewall nat add --chain dstnat --action dst-nat --protocol tcp --dst-port 8080 --to-addresses 192.168.88.100 --to-ports 80
routeros-cli firewall nat remove '*3' --force
```

### DHCP

```sh
# Leases
routeros-cli dhcp lease list
routeros-cli dhcp lease list --active    # only bound leases

# Servers and pools
routeros-cli dhcp server list
routeros-cli dhcp pool list
```

### Backup

```sh
# Text export (RouterOS config format)
routeros-cli backup export                      # print to stdout
routeros-cli backup export --file backup.rsc    # save to local file

# Binary backup (saved on the router)
routeros-cli backup binary                               # default name
routeros-cli backup binary --file my-backup-2026-03-16   # custom name
```

### Monitoring

```sh
# Traffic monitoring (Ctrl+C to stop)
routeros-cli monitor traffic ether1
routeros-cli monitor traffic ether1 --interval 500ms

# Resource monitoring
routeros-cli monitor resources
routeros-cli monitor resources --interval 5s -o json
```

### Arbitrary Commands

```sh
# Execute any RouterOS API command
routeros-cli exec /system/package/print
routeros-cli exec /interface/print =type=ether
routeros-cli exec /ip/firewall/filter/print ?chain=input
routeros-cli exec /system/clock/print
```

### Schema (AI Integration)

```sh
# List available schemas
routeros-cli schema

# Get schema for a specific command output
routeros-cli schema interface
routeros-cli schema dhcp-lease
routeros-cli schema envelope    # the {ok, data, meta} wrapper
routeros-cli schema error       # the {ok, error} wrapper
```

### Shell Completions

```sh
# Bash
routeros-cli completion bash > /etc/bash_completion.d/routeros-cli

# Zsh
routeros-cli completion zsh > "${fpath[1]}/_routeros-cli"

# Fish
routeros-cli completion fish > ~/.config/fish/completions/routeros-cli.fish
```

## Configuration

Config is stored at `~/.config/routeros-cli/config.toml`:

```toml
default_device = "home"
default_output = "table"       # "table" or "json"

[devices.home]
address = "192.168.88.1:8728"
username = "admin"
tls = false

[devices.office]
address = "10.0.1.1:8729"
username = "api-user"
tls = true

[tls]
insecure_skip_verify = false   # set true for self-signed certs
ca_cert = ""                   # path to custom CA certificate
```

Passwords are **never** stored in the config file. They are kept in your OS keyring (GNOME Keyring, macOS Keychain, or Windows Credential Manager).

## JSON Output Format

All JSON output follows a stable envelope format designed for programmatic consumption:

**Success:**
```json
{
  "ok": true,
  "data": [ { "Name": "ether1", "Type": "ether", "Running": "true" } ],
  "meta": {
    "device": "home",
    "command": "/interface/print",
    "timestamp": "2026-03-16T12:00:00Z",
    "count": 1
  }
}
```

**Error:**
```json
{
  "ok": false,
  "error": {
    "code": "connection_failed",
    "message": "dial tcp 192.168.88.1:8728: i/o timeout",
    "device": "home"
  }
}
```

**Exit codes:**

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Command error (bad input, API error) |
| 2 | Connection error (unreachable, auth failed) |
| 3 | Configuration error (bad config, missing device) |

## RouterOS API Setup

The CLI uses the RouterOS native API protocol, not SSH. You need to enable the API service on your router.

### Enable API (plain, port 8728)

```
/ip/service/set api disabled=no
# Restrict to your LAN:
/ip/service/set api address=192.168.88.0/24
```

### Enable API-SSL (TLS, port 8729)

```
# First create or import a certificate
/certificate/add name=local-cert common-name=router key-size=2048
/certificate/sign local-cert

# Then assign it to the api-ssl service
/ip/service/set api-ssl disabled=no certificate=local-cert
/ip/service/set api-ssl address=192.168.88.0/24
```

### Firewall

If you have an input chain firewall, add a rule to allow API access:

```
/ip/firewall/filter/add chain=input action=accept protocol=tcp \
  src-address=192.168.88.0/24 dst-port=8728 \
  comment="LAN API Access" place-before=0
```

## Using with AI Agents

routeros-cli is designed to work as a tool/skill for AI agents (Claude, GPT, etc.):

```sh
# All commands support JSON output
routeros-cli -o json system info | jq '.data[0].Version'

# Self-describing schema
routeros-cli schema interface | jq .

# Deterministic exit codes for error handling
routeros-cli -o json -d office system info
echo $?  # 0=ok, 1=cmd error, 2=conn error, 3=config error

# Non-interactive by default (no prompts in JSON mode)
# Destructive operations skip confirmation with --force
routeros-cli -o json firewall filter remove '*5' --force
```

### Example: Claude MCP Tool Definition

```json
{
  "name": "routeros",
  "description": "Manage MikroTik RouterOS routers",
  "input_schema": {
    "type": "object",
    "properties": {
      "command": { "type": "string", "description": "routeros-cli command to execute" },
      "device": { "type": "string", "description": "Target device name" }
    }
  }
}
```

## Architecture

```
routeros-cli/
├── cmd/                    # Cobra commands (1 file per command group)
├── internal/
│   ├── config/             # TOML config loading/saving
│   ├── credential/         # OS keyring credential store
│   ├── client/             # RouterOS API client wrapper
│   ├── output/             # Table + JSON renderers
│   └── rosapi/             # Type mapper + RouterOS data types
├── pkg/schema/             # Public JSON Schema definitions
├── main.go                 # Entry point
├── .goreleaser.yaml        # Cross-platform release builds
└── .github/workflows/      # CI + release automation
```

## Building

```sh
# Development build
go build -o routeros-cli .

# With version info
go build -ldflags "-X main.version=1.0.0 -X main.commit=$(git rev-parse HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o routeros-cli .

# Cross-compile
GOOS=darwin GOARCH=arm64 go build -o routeros-cli-darwin-arm64 .
GOOS=linux GOARCH=amd64 go build -o routeros-cli-linux-amd64 .

# Run tests
go test ./...
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Run tests (`go test ./...`)
4. Commit changes
5. Open a Pull Request

## License

[MIT](LICENSE)
