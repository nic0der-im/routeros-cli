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

## Typical prompts

- "Audit `home` with ros and propose optimizations (read-only)."
- "Using ros-safe-apply, set DHCP lease-time to 1d on home inside a safe session."

## Manual docs

- [COMMANDS.md](COMMANDS.md)
- Bundled skill references are installed next to each `SKILL.md`
