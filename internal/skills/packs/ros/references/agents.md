# Agent integration notes

- Default agent mode: read-only + JSON.
- Prefer `audit` over dumping full `/export` into the model context.
- Device names may contain spaces: `-d "EOC FRONTERA"`.
- Lookup also accepts id slug and unique IP.
- Winbox import defaults API port `:8728` (not Winbox GUI port).
- Safe sessions are best-effort API journals (RouterOS terminal Safe Mode is not exposed on the binary API).
