# Agent integration notes

- Default agent mode: read-only + JSON.
- Prefer `audit` over dumping full `/export` into the model context.
- Pass **base** API paths only — never `…/print` or `…/get` on `ros get`.
- After audit, run the hygiene checklist (resources, FastTrack, leases, files, cloud, services, unused ports, drops).
- ROS ≥7.17 cloud: `ddns-enabled` is `yes|auto` only; `auto` ≈ off unless Back To Home; use `update-time=false` with NTP.
- Low-RAM boards: prefer `--skip-cpu-profile` when iterating audits.
- Prefer non-`--raw`; WireGuard private keys are redacted in normal output but still treat as sensitive.
- Device names may contain spaces: `-d "central hub BA"`.
- Lookup also accepts id slug and unique IP.
- Winbox import defaults API port `:8728` (not Winbox GUI port).
- Safe sessions are best-effort API journals (RouterOS terminal Safe Mode is not exposed on the binary API).
