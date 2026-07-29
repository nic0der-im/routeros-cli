# Safety and recovery (agent)

When a write or session step fails, match the situation below, then follow the recovery spine. Prefer the message/`suggested_action` over inventing retries.

## Exit situations → next step

| Situation | Typical `error.code` / kind | Next step |
|-----------|----------------------------|-----------|
| `read_only` | `read_only` | Unset `ROS_READ_ONLY` / drop `--read-only`; load `ros-safe-apply` only after user approval |
| Safe session required | `session` (`ErrSafeSessionRequired`) | `ros -d DEV session begin --safe` (agent/`agent-strict` and prod/staging) |
| Doctor stale / missing | `session` (`ErrDoctorStale`) | `ros -d DEV doctor` (or `audit --profile hygiene`); wait ≤60m freshness on prod |
| Outside maintenance | `session` (`ErrOutsideMaintenanceWindow`) | Wait for `maintenance_windows`, or break-glass only (below) |
| Conflict (ambiguous id/comment) | `conflict` | Disambiguate with exact `.id` / unique `--comment`; re-`get` before mutate |
| Timeout (dial/read) | `timeout` / `connection` | Check reachability (`device test`, `diag ping`); reads may auto-retry — do not invent state |
| Busy | `busy` | Brief backoff; re-run read; never hammer writes |
| Ambiguous write | `timeout` + `suggested_action` | **Verify** with `--read-only get` before any retry — never blind re-write (A5) |
| Confirm required | plain refuse (often `api`) | Pass `--confirm <exact-inventory-name>`; `--force` does **not** replace it |
| Blast radius (session cap) | `session` (`ErrMaxChanges`) | `session commit` or `session rollback` before more writes |
| Path denied | `session` (`ErrPathDenied`) | Use an allowed path; do not `--force` past path policy |
| Exec denied | usually `api` (`ErrExecDenied`) | Prefer curated `set`/`create`/`delete`; adjust `exec_allow`/`exec_deny` only with user intent |

Details live in message text (doctor hint, window list, path reason). JSON may include `error.suggested_action`.

## Recovery spine

```text
doctor → dry-run → session begin --safe → apply → verify → commit | rollback
```

1. **doctor** — `ros -d DEV --read-only doctor` (or hygiene audit); fix FINDINGS before prod writes.
2. **dry-run** — mutate with `--dry-run` to preview outcomes (`dry_run` / `no_change` / …).
3. **session begin --safe** — required on prod/staging and agent profiles; prod also needs pre-session backup unless break-glass.
4. **apply** — minimal curated verbs; journaled `set` over raw `exec` when possible.
5. **verify** — `--read-only get … -o json` / `device test` / targeted `diag`.
6. **commit | rollback** — `session commit` on success; else `session rollback`.

## Break-glass and extras

| Tool | When |
|------|------|
| `session watch` | WAN/uplink/firewall changes that can cut API; remote VPN path; auto-rollback on link loss |
| Backups | Before risky writes: `backup export` / `backup binary`; prod safe-session may require local export |
| `--skip-doctor-gate` / `ROS_SKIP_DOCTOR_GATE=1` / `--force` | **Break-glass only** — skips doctor freshness (and `--force` / `App.Force` also bypasses maintenance). Prefer re-running `doctor` |
| `ROS_SKIP_MAINTENANCE_GATE=1` | **Break-glass only** — outside `maintenance_windows` |

Do not use break-glass flags to “unblock” agents by default. Escalate to the user when prod gates fire.
