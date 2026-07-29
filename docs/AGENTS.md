# Agent guide for `ros`

## Install skills into your coding agents

```sh
# Recommended: all agents, user-global
ros skills install --agent all --scope user

# Only Cursor, project-local
ros skills install --agent cursor --scope project --force

# Inspect paths
ros skills path --agent all --scope user
ros skills list
```

### Bundled packs

Bundled skill pack metadata version: **0.5.0** (install/update with `ros skills install … --force` after upgrading `ros`).

| Pack | When the agent loads it |
|------|-------------------------|
| `ros` | Audit, inventory, read-only queries, MikroTik inspection |
| `ros-safe-apply` | Applying firewall/IP/DHCP/user changes via safe sessions |

### Recommended parameters

| Flag | Default | Recommendation |
|------|---------|----------------|
| `--agent` | `all` | Use `all` on your workstation; `cursor` in CI/project templates |
| `--scope` | `user` | `user` for personal tech laptop; `project` to ship skills with a repo |
| `--force` | off | Use when updating after `ros` upgrades |
| `--pack` | both | Install both unless you only want read-only guidance |

## Agent runtime defaults

```sh
export ROS_READ_ONLY=1
export ROS_DEFAULT_OUTPUT=json
# optional: keep agent context small
# export ROS_MAX_OUTPUT_BYTES=512000   # default
# ros --limit 100 get interface
```

JSON envelopes include `meta.request_id` on success and error (correlate with `-v` logs). List dumps honor `--limit N` and `ROS_MAX_OUTPUT_BYTES` (`meta.truncated=true` when capped).

## Safe sessions

During writes, prefer:

```sh
ros -d router-edge session begin --safe
# optional second terminal:
ros -d router-edge session watch
# ... apply changes ...
ros -d router-edge session commit
```

`session watch` probes the API and auto-rollbacks on link loss (best-effort; not RouterOS terminal Safe Mode).

JSON errors use stable `error.code` kinds: `connection`, `auth`, `config`, `read_only`, `session`, `api`, `not_found`, `conflict`, `timeout`, `busy` (plus message-level situations such as doctor stale, outside maintenance, path/exec denied). Agent recovery map: skill `references/safety-and-recovery.md`.

## Prompt library

Copy-paste prompts. Replace `DEV` with the inventory name. Prefer `-o json` / `ROS_DEFAULT_OUTPUT=json`. Command details: [COMMANDS.md](COMMANDS.md). Official MikroTik links: skill `references/routeros-docs.md`. Recovery map: `references/safety-and-recovery.md`.

### Inspect / inventory / read-only audit

```
Audit DEV with ros (read-only). Run device list if needed, then
ros -d DEV --read-only audit --profile full -o json
(and hygiene/doctor if optimizing). Summarize FINDINGS; propose changes without applying.
Load skill ros only; do not write.
```

```
Inventory DEV: interfaces, addresses, routes, DHCP servers/leases, and firewall filter
counts via ros --read-only get … -o json. Prefer curated domains from ros domains.
Cite exact commands; do not invent router state.
```

```
Read-only security pass on DEV: audit --profile security -o json, then get ip/service
and get firewall/filter. Flag unused management listeners and risky accepts. No writes.
```

### Firewall change via safe session

```
Using ros-safe-apply on DEV: refresh doctor, dry-run the firewall change, then
session begin --safe, apply the minimal create/set/delete, verify with --read-only get,
and session commit (or rollback on failure). For WAN-facing rules, start session watch.
Unset ROS_READ_ONLY before writes. See COMMANDS.md firewall/session sections.
```

```
On DEV, add/adjust this firewall filter rule via ros safe session only:
<describe chain/action/match>. Preview with --dry-run first. Prefer --comment for stable id
on filter/mangle. Do not use raw REST examples — ros uses the binary API
(references/routeros-docs.md).
```

### WireGuard peer diagnose

```
On DEV, diagnose WireGuard peers read-only:
ros -d DEV --read-only wg peers --stale-after 15m -o json
List stale (empty/never/old handshake) peers; do not delete. Cross-check FINDINGS from doctor
if present. Official WG docs: references/routeros-docs.md (WireGuard entry).
```

### Diagnose loop (doctor → diag log → ping/neighbors)

```
Diagnose connectivity on DEV: (1) ros -d DEV --read-only doctor
(2) ros -d DEV --read-only diag log --topics <error,firewall,…> --since 15m --limit 50
(3) diag ping <target> and/or diag neighbors as needed.
Report FINDINGS, log hits, and reachability; propose next steps only — no writes unless I approve.
```

### Plan preview / apply / rollback

```
Preview this YAML change plan against DEV (read-only dry-run path):
ros plan preview --file plan.yaml
Summarize risk notes and outcomes. Do not apply yet. Schema: docs/COMMANDS.md Plans.
```

```
Apply plan.yaml on DEV: ensure doctor is fresh (prod), session begin --safe,
then ros plan apply --file plan.yaml (--confirm DEV if any delete).
Verify with read-only get; session commit on success, else plan rollback / session rollback.
```

### Production guardrails

```
Before any write on DEV (env_class=prod or ROS_STRICT): run doctor (≤60m freshness),
respect maintenance_windows, use session begin --safe, prefer --dry-run, and pass
--confirm <exact-inventory-name> for reboot/file remove/device remove/lease cleanup-waiting.
Do not use --skip-doctor-gate / ROS_SKIP_* / --force unless I explicitly break-glass.
See COMMANDS.md Prod write protocol and skill references/safety-and-recovery.md.
```

```
Explain which guardrails apply to DEV (env_class, ROS_PROFILE, doctor gate,
maintenance_windows, allowed/denied write paths, --confirm). Inspect config/device as needed;
do not bypass gates.
```

## Manual docs

- [COMMANDS.md](COMMANDS.md)
- Bundled skill references installed next to each `SKILL.md`: `commands.md`, `agents.md`, `safety-and-recovery.md`, `routeros-docs.md`
