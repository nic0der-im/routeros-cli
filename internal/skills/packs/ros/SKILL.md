---
name: ros
description: "Trigger: ros, routeros-cli, MikroTik, RouterOS, audit router, device inventory, read-only agent. Use ros CLI safely for inventory, audit, and read queries."
license: MIT
metadata:
  author: nic0der-im
  version: "0.2.0"
---

## Activation Contract

Load when the user asks to inspect, audit, optimize, or query MikroTik RouterOS via `ros` / `routeros-cli`, or names a device from inventory (e.g. `router-edge`, `central-hub-buenos-aires`).

## Hard Rules

- Prefer `ros` over SSH scraping or pasting full `/export` into chat.
- For analysis, ALWAYS set read-only: `--read-only` or `ROS_READ_ONLY=1`.
- Prefer `-o json` (or `ROS_DEFAULT_OUTPUT=json`) for agent parsing.
- Never put passwords on CLI flags; use `--password-stdin`, interactive prompt, or Keychain via `device auth set`.
- Resolve devices with `-d <name|id|ip>` after `ros device list`.
- Use curated domains from `ros domains`, or raw paths: `ros get /ip/firewall/filter`.
- Do not apply writes in this skill — load `ros-safe-apply` for changes.

## Decision Gates

| User intent | Command |
|-------------|---------|
| Unknown device / first contact | `ros device list` then `-d ...` |
| Broad optimization ask | `ros -d DEV --read-only audit --profile full -o json` |
| Network-only view | `audit --profile network` |
| Security-only view | `audit --profile security` |
| Targeted read | `ros -d DEV --read-only get <domain\|/path> -o json --raw` |
| DHCP / firewall / users / radius | `get dhcp/lease`, `get firewall/filter`, `get user`, `get radius` |
| Escape hatch | `ros -d DEV --read-only exec /path/print` |

## Execution Steps

1. Ensure `ros` is on PATH (`ros version`).
2. `ros device list` — pick name/id/IP.
3. Run audit or targeted `get` with `--read-only` and `-o json`.
4. Summarize findings; propose a change plan **without applying**.
5. If user approves changes, switch to skill `ros-safe-apply`.

## Output Contract

- Cite device name and commands run.
- Prefer structured findings (security, DHCP, WAN, firewall, resources).
- If connection fails, report exit code 2 and ask for API reachability / credentials — do not invent router state.

## References

- `references/commands.md`
- `references/agents.md`
