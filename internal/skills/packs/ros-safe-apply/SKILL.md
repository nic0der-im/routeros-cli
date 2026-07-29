---
name: ros-safe-apply
description: "Trigger: ros apply, safe session, rollback, firewall change, rotate password, MikroTik write, commit session. Apply RouterOS changes only via ros safe sessions."
license: MIT
metadata:
  author: nic0der-im
  version: "0.4.0"
---

## Activation Contract

Load when the user approves applying RouterOS changes with `ros` (firewall, IP, DHCP, users, RADIUS, services, cloud, files, etc.).

## Hard Rules

- NEVER write while `ROS_READ_ONLY=1` or `--read-only` is set — unset first.
- ALWAYS wrap multi-step writes: `ros -d DEV session begin --safe` → changes → `session commit` (or `rollback`).
- Prefer verb+domain: `create|set|delete|enable|disable <domain|/path> key=value...`.
- Prefer `set` over `exec` for singleton menus (e.g. `/ip/cloud`) — safe sessions journal singleton `/set` (no `.id`) via pre-state snapshot.
- Destructive ops (`reboot`, bulk deletes) require explicit user confirmation.
- Explicit confirm before disabling management services (`api-ssl`, `www`, `www-ssl`, `ftp`, `telnet`, `ssh` address lock-down, etc.).
- Passwords: `--password-stdin` or interactive `device auth set` / `set user .id=*N password=...` — never echo secrets in chat.
- After apply, verify with read-only `get` / `device test`.

## Decision Gates

| Change type | Pattern |
|-------------|---------|
| Add firewall rule | `create firewall/filter chain=... action=...` |
| Fix NAT WAN | `set firewall/nat .id=*N out-interface=ether1` |
| DHCP lease-time | `set dhcp/server .id=*1 lease-time=1d` |
| Remove stale lease | `delete dhcp/lease .id=*F9` |
| Cloud DDNS “off” (ROS ≥7.17) | `set ip/cloud ddns-enabled=auto update-time=false` — not `ddns-enabled=no` |
| Remove on-router file | `file get NAME --output ./local` then `file remove NAME` (or `delete file .id=…`) |
| Fresh on-router binary backup | `backup binary` (default UTC `ros-backup-YYYYMMDD-HHMMSS`) |
| Rotate user password | `set user .id=*2 password=...` (stdin) |
| WAN-facing / risk of lockout | start `session watch` before changes |
| Unknown menu | raw path from `ros domains` or `/path` |

## When to use `session watch`

- WAN / uplink / firewall changes that can cut API reachability.
- Remote routers (CloudISP / VPN path) where a bad rule strands the session.
- Optional for LAN-only hygiene (disable unused ether, delete local backups, cloud DDNS enum) — still fine to run.

## Execution Steps

1. Confirm target: `ros device get DEV` / `device test`.
2. Optional pre-backup: `ros -d DEV backup export --file ./DEV-$(date +%F).rsc` and/or `backup binary --output ./backups/`.
3. `ros -d DEV session begin --safe`.
4. For WAN-facing or risky changes: `ros -d DEV session watch` in another terminal (auto-rollback on link loss).
5. Apply minimal commands; for files: download with `file get` before `file remove` on the router.
6. `ros -d DEV session status` — review journal (singleton `set` should show Changes > 0).
7. `ros -d DEV session commit` on success, else `session rollback`.
8. Verify with `--read-only get ... -o json` / `file list`.

## Output Contract

- List exact commands applied and session id/status.
- State whether committed or rolled back.
- Note remaining manual steps (e.g. Winbox-only UI).

## References

- `references/commands.md`
- `references/agents.md`
