---
name: ros-safe-apply
description: "Trigger: ros apply, safe session, rollback, firewall change, rotate password, MikroTik write, commit session. Apply RouterOS changes only via ros safe sessions."
license: MIT
metadata:
  author: nic0der-im
  version: "0.2.0"
---

## Activation Contract

Load when the user approves applying RouterOS changes with `ros` (firewall, IP, DHCP, users, RADIUS, services, etc.).

## Hard Rules

- NEVER write while `ROS_READ_ONLY=1` or `--read-only` is set — unset first.
- ALWAYS wrap multi-step writes: `ros -d DEV session begin --safe` → changes → `session commit` (or `rollback`).
- Prefer verb+domain: `create|set|delete|enable|disable <domain|/path> key=value...`.
- Destructive ops (`reboot`, bulk deletes) require explicit user confirmation.
- Passwords: `--password-stdin` or interactive `device auth set` / `set user .id=*N password=...` — never echo secrets in chat.
- After apply, verify with read-only `get` / `device test`.

## Decision Gates

| Change type | Pattern |
|-------------|---------|
| Add firewall rule | `create firewall/filter chain=... action=...` |
| Fix NAT WAN | `set firewall/nat .id=*N out-interface=ether1` |
| DHCP lease-time | `set dhcp/server .id=*1 lease-time=1d` |
| Remove stale lease | `delete dhcp/lease .id=*F9` |
| Rotate user password | `set user .id=*2 password=...` (stdin) |
| Unknown menu | raw path from `ros domains` or `/path` |

## Execution Steps

1. Confirm target: `ros device get DEV` / `device test`.
2. Optional pre-backup: `ros -d DEV backup export --file ./DEV-$(date +%F).rsc`.
3. `ros -d DEV session begin --safe`.
4. Optionally start `ros -d DEV session watch` in another terminal for link-loss auto-rollback.
5. Apply minimal commands; capture `.id` from create output or `--raw` gets.
6. `ros -d DEV session status` — review journal.
7. `ros -d DEV session commit` on success, else `session rollback`.
8. Verify with `--read-only get ... -o json`.

## Output Contract

- List exact commands applied and session id/status.
- State whether committed or rolled back.
- Note remaining manual steps (e.g. Winbox-only UI).

## References

- `references/commands.md`
- `references/agents.md`
