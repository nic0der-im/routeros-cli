# Agent integration notes

- Default agent mode: read-only + JSON until the user approves writes.
- Prefer `audit` over dumping full `/export` into the model context.
- Writes only inside `session begin --safe` → commit/rollback.
- Singleton menus (`/ip/cloud`): prefer `set` so the journal records inverse; avoid `exec /ip/cloud/set` unless forced.
- Cloud (ROS ≥7.17): `ddns-enabled=auto` (not `no`); `update-time=false` with NTP.
- File cleanup: `file get` locally first, then `file remove` on the router.
- Confirm before disabling management services; use `session watch` for WAN-facing or lockout-risk changes.
- Device names may contain spaces: `-d "central hub BA"`.
- Lookup also accepts id slug and unique IP.
- Winbox import defaults API port `:8728` (not Winbox GUI port).
- Safe sessions are best-effort API journals (RouterOS terminal Safe Mode is not exposed on the binary API).
