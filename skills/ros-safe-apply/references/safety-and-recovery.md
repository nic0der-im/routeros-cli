# Safety and recovery

Canonical recovery map (exit situations → next step, recovery spine, break-glass):

→ **`../ros/references/safety-and-recovery.md`** (same content in installed packs: `ros/references/safety-and-recovery.md`)

Summary for apply flows:

1. `ros -d DEV doctor` / hygiene FINDINGS before prod writes.
2. `--dry-run` → `session begin --safe` → apply → verify → `commit` \| `rollback`.
3. Use `session watch` for WAN/lockout risk; backups before risky changes.
4. `--skip-doctor-gate` / `--force` / `ROS_SKIP_*` are **break-glass only**.
