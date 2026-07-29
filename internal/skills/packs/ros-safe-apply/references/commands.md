# ros command cheat sheet (agent)

```text
ros -d DEV [--read-only] [-o json] [--raw] VERB DOMAIN|PATH [params...]
```

Verbs: `get` `create` `set` `delete` `enable` `disable`  
Domains: run `ros domains`  
Params: `key=value`, `.id=*N`, filters `?=key=value`  
Paths: base menus only — do **not** append `/print` or `/get`.

## Inventory

```sh
ros device list
ros device add                  # interactive
echo "$PASS" | ros device add NAME --address host:8728 --username admin --password-stdin
ros device auth set NAME
ros device test
ros device import --from winbox [--dry-run] [--with-passwords]
```

## Agent read path

```sh
export ROS_READ_ONLY=1 ROS_DEFAULT_OUTPUT=json
ros -d DEV audit --profile full          # also: network|security|hygiene
ros -d DEV audit --skip-cpu-profile --show-ppp
ros -d DEV get firewall/filter
ros -d DEV get user
ros -d DEV file list
```

## Safe write path

```sh
unset ROS_READ_ONLY
ros -d DEV session begin --safe
# WAN-facing / risky: ros -d DEV session watch   # other terminal
ros -d DEV create firewall/filter chain=forward action=accept ...
ros -d DEV set ip/cloud ddns-enabled=auto update-time=false   # ROS ≥7.17; not =no
ros -d DEV file get auto-before-reset.backup --output ./local/
ros -d DEV file remove auto-before-reset.backup
ros -d DEV session status
ros -d DEV session commit
```

Prefer `set` (journaled) over `exec …/set` for singleton menus like `/ip/cloud`.  
Confirm with the user before disabling management services (`api-ssl`, www, …).  
`backup binary` default remote name: UTC `ros-backup-YYYYMMDD-HHMMSS`.

Exit codes: 0 OK · 1 cmd · 2 conn · 3 config · 4 read-only violation
