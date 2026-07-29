# Troubleshooting

## macOS Keychain

Passwords are stored via the system keyring (`ros` service).

- First `device add --password-stdin` may prompt for Keychain access — allow it.
- Headless SSH sessions without an unlocked keychain can fail; run from a local Terminal or unlock with `security unlock-keychain`.
- Legacy credentials under service `routeros-cli` are migrated on first read.

## Connection fails

1. Confirm API is enabled: `/ip/service/print where name=api`
2. Confirm firewall allows your client subnet on 8728 (or 8729 for TLS)
3. Match TLS to port:
   - `host:8728` → plain API (TLS inferred off)
   - `host:8729` → API-SSL (TLS inferred on)
4. For self-signed certs set in `~/.config/ros/config.toml`:

```toml
[tls]
insecure_skip_verify = true
# or:
# ca_cert = "/path/to/ca.pem"
```

## Config location

- New: `~/.config/ros/config.toml`
- Legacy `~/.config/routeros-cli/config.toml` is migrated automatically on first run

## Read-only unexpected

Check `ROS_READ_ONLY` is unset when applying changes. Exit code `4` means a write was blocked.

## Safe session stuck

```sh
ros session status
ros session rollback   # or commit
```

Only one active session per device is allowed.
