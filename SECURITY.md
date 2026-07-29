# Security Policy

## Supported versions

| Version | Supported |
|---------|-----------|
| 0.2.x   | Yes |
| 0.1.x   | Best effort |

## Reporting a vulnerability

Please open a [private security advisory](https://github.com/nic0der-im/routeros-cli/security/advisories/new) or contact the maintainer via GitHub.

Do **not** file public issues for credential leaks or remote code execution.

## Credential handling

- Passwords are never accepted as CLI flags and never written to `config.toml`
- Passwords are stored in the OS keyring (`ros` service)
- Prefer `--password-stdin` when adding or rotating credentials
- For agent automation, keep `ROS_READ_ONLY=1` unless a human approved a write plan

## Network exposure

RouterOS API (8728/8729) should be restricted to management networks. Prefer API-SSL with a trusted CA when exposed beyond a lab LAN.
