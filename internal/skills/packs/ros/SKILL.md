---
name: ros
description: "Trigger: ros, routeros-cli, MikroTik, RouterOS, audit router, device inventory, read-only agent. Use ros CLI safely for inventory, audit, and read queries."
license: MIT
metadata:
  author: nic0der-im
  version: "0.5.0"
---

## Activation Contract

Load when the user asks to inspect, audit, optimize, or query MikroTik RouterOS via `ros` / `routeros-cli`, or names a device from inventory (e.g. `router-edge`, `central-hub-buenos-aires`).

## Hard Rules

- Prefer `ros` over SSH scraping or pasting full `/export` into chat.
- For analysis, ALWAYS set read-only: `--read-only` or `ROS_READ_ONLY=1`.
- Prefer `-o json` (or `ROS_DEFAULT_OUTPUT=json`) for agent parsing.
- Never put passwords on CLI flags; use `--password-stdin`, interactive prompt, or Keychain via `device auth set`.
- Resolve devices with `-d <name|id|ip>` after `ros device list`.
- Use curated domains from `ros domains`, or raw **base** paths: `ros get /ip/firewall/filter`.
- NEVER append `/print` or `/get` to paths — `ros get` already runs print. Wrong: `get /interface/print` → RouterOS `…/print/print`.
- Prefer non-`--raw` output. Table/JSON redact known secrets (e.g. WireGuard `private-key`) as `***`; `--raw` can still leak secrets — avoid dumping WG/user secrets into chat.
- Do not apply writes in this skill — load `ros-safe-apply` for changes.

## Decision Gates

| User intent | Command |
|-------------|---------|
| Unknown device / first contact | `ros device list` then `-d ...` |
| Broad optimization ask | `ros -d DEV --read-only audit --profile full -o json` |
| Network-only view | `audit --profile network` |
| Security-only view | `audit --profile security` |
| Hygiene / optimize pass | `audit --profile hygiene` or `doctor` (then checklist below); use `full` for broader snapshot |
| Quick FINDINGS pass (before writes) | `ros -d DEV --read-only doctor` (hygiene + FINDINGS; prod write gate) |
| Router log slice | `ros -d DEV --read-only diag log --topics … --since …` |
| Reachability / L2 / path | `diag ping`, `diag neighbors`, `diag traceroute` when relevant |
| Stale WG / down hosts / DNS clutter | `wg peers --stale-after`, `get netwatch`, `get dns/static` (also in FINDINGS) |
| Low-RAM board (e.g. RB2011 64MB) | add `--skip-cpu-profile` while iterating |
| Include PPPoE/PPP ifaces in human audit | `--show-ppp` |
| Targeted read | `ros -d DEV --read-only get <domain\|/path> -o json` (base path only) |
| DHCP / firewall / users / radius | `get dhcp/lease`, `get firewall/filter`, `get user`, `get radius` |
| On-router files | `ros -d DEV --read-only file list` |
| Escape hatch | `ros -d DEV --read-only exec /path/print` |
| Write/session failure recovery | See `references/safety-and-recovery.md` (then hand off to `ros-safe-apply`) |

## Optimization / hygiene checklist (after audit)

Walk these before proposing writes (read-only):

1. **Resources** — free RAM, CPU load, `bad-blocks` if present.
2. **FastTrack / ip settings** — `get ip/settings` (or `/ip/settings`); note active FastTrack flags.
3. **DHCP leases** — waiting/stale duplicates (same MAC two leases).
4. **Files** — `file list`; stale `.backup` / smoke exports on flash.
5. **Cloud (ROS ≥7.17)** — `get ip/cloud`: `ddns-enabled` is `yes\|auto` only (`auto` = off unless Back To Home). Prefer `update-time=false` when NTP is in use. Do not send `ddns-enabled=no`.
6. **Services** — `get ip/service`; unused management listeners (api-ssl, www, …).
7. **Unused ports** — disabled/empty ethernet, orphan bridges.
8. **Iface drops** — TX-QUEUE-DROP / error counters on running interfaces.

Compact lab flow: audit (with `--skip-cpu-profile` on small boards) → checklist → `file list` → propose plan → hand off to `ros-safe-apply`.

## Execution Steps

1. Ensure `ros` is on PATH (`ros version`).
2. `ros device list` — pick name/id/IP.
3. Run audit or targeted `get` with `--read-only` and `-o json`.
4. Run the hygiene checklist; summarize findings; propose a change plan **without applying**.
5. If user approves changes, switch to skill `ros-safe-apply`.

## Output Contract

- Cite device name and commands run.
- Prefer structured findings (security, DHCP, WAN, firewall, resources, cloud, files).
- If connection fails, report exit code 2 and ask for API reachability / credentials — do not invent router state.

## References

- `references/commands.md`
- `references/agents.md`
- `references/safety-and-recovery.md` — exit situations → next step; recovery spine; break-glass
- `references/routeros-docs.md` — official MikroTik help pointers (API vs REST; topic entry points)
