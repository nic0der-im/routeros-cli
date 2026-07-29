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
```

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

JSON errors use stable `error.code` kinds: `connection`, `auth`, `config`, `read_only`, `session`, `api`, `not_found`.

## Typical prompts

- "Audit `router-edge` with ros and propose optimizations (read-only)."
- "Using ros-safe-apply, set DHCP lease-time to 1d on router-edge inside a safe session."

## Manual docs

- [COMMANDS.md](COMMANDS.md)
- Bundled skill references are installed next to each `SKILL.md`
