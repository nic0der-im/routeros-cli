# RouterOS official documentation pointers

Canonical official doc index (MikroTik help links, API vs REST, topic entry points, version-caveat style):

→ **`../ros/references/routeros-docs.md`** (same content in installed packs: `ros/references/routeros-docs.md`)

Quick reminders for apply flows:

- `ros` uses the **binary API**, not REST — see the API row in the ros pack ref.
- Before mutating unfamiliar menus, open the topic page linked there; confirm enums on-device with `--read-only get`.
- Recovery / gates: `references/safety-and-recovery.md`.
