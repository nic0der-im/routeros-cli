# ros command cheat sheet (agent)

```text
ros -d DEV [--read-only] [-o json] [--raw] VERB DOMAIN|PATH [params...]
```

Verbs: `get` `create` `set` `delete` `enable` `disable`  
Domains: run `ros domains`  
Params: `key=value`, `.id=*N`, filters `?=key=value`  
Paths: base menus only — do **not** append `/print` or `/get` (`ros get` adds print).

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
ros -d DEV doctor                        # hygiene + FINDINGS; run before proposing/prod writes
ros -d DEV audit --skip-cpu-profile      # low-RAM / faster iteration
ros -d DEV audit --show-ppp              # include PPPoE/PPP in human summary
ros -d DEV get firewall/filter           # prefer without --raw (secrets redacted)
ros -d DEV get ip/cloud
ros -d DEV get user
ros -d DEV file list
```

Human audit: boxed columns; iface RX/TX = cumulative bytes (not live Mbps).  
`--raw` / unredacted secrets: avoid for WireGuard `private-key` and passwords.

## Diagnose loop

```sh
ros -d DEV doctor                        # FINDINGS first (WG stale, netwatch down, DNS clutter, …)
ros -d DEV diag log --topics firewall,error --since 15m
ros -d DEV diag ping 1.1.1.1 --count 4
ros -d DEV diag neighbors
ros -d DEV diag traceroute 8.8.8.8
ros -d DEV wg peers --stale-after 15m    # read-only annotate; no auto-delete
ros -d DEV get netwatch
ros -d DEV get dns/static
```

## Files / backup

```sh
ros -d DEV file list
ros -d DEV file get NAME.backup --output ./local/
ros -d DEV file remove stale.backup
ros -d DEV backup binary --output ./backups/   # default remote name ros-backup-YYYYMMDD-HHMMSS UTC
ros -d DEV backup export --file ./DEV.rsc
```

LAN with SSH already allowlisted: add `--ephemeral-ssh=false` on SFTP downloads.

## Safe write path

```sh
unset ROS_READ_ONLY
ros -d DEV doctor                        # prod freshness gate
ros -d DEV session begin --safe
ros -d DEV create firewall/filter chain=forward action=accept ...
ros -d DEV session commit
```

Exit codes: 0 OK · 1 cmd · 2 conn · 3 config · 4 read-only violation  

Recovery map: `references/safety-and-recovery.md`
