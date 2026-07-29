# ros command cheat sheet (agent)

```text
ros -d DEV [--read-only] [-o json] [--raw] VERB DOMAIN|PATH [params...]
```

Verbs: `get` `create` `set` `delete` `enable` `disable`  
Domains: run `ros domains`  
Params: `key=value`, `.id=*N`, filters `?=key=value`

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
ros -d DEV audit --profile full
ros -d DEV get firewall/filter --raw
ros -d DEV get user
```

## Safe write path

```sh
unset ROS_READ_ONLY
ros -d DEV session begin --safe
ros -d DEV create firewall/filter chain=forward action=accept ...
ros -d DEV session commit
```

Exit codes: 0 OK · 1 cmd · 2 conn · 3 config · 4 read-only violation
