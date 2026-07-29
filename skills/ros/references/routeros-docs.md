# RouterOS official documentation pointers

Pointers only — open the linked MikroTik docs; do not paste or paraphrase manuals into chat.
Prefer `ros` curated verbs / `get` / `audit` over inventing CLI from memory. When docs and live board disagree, trust the device (`ros -d DEV --read-only get …`).

## MikroTik help (RouterOS)

| Entry | URL |
|-------|-----|
| RouterOS doc space | https://help.mikrotik.com/docs/space/ROS |
| Help home | https://help.mikrotik.com/ |

Search within the ROS space for menus you do not see listed below (e.g. `/ip/cloud`, services, packages).

## API vs REST (`ros` uses binary API)

| Topic | URL | Notes for agents |
|-------|-----|------------------|
| **API** (binary / sentence protocol) | https://help.mikrotik.com/docs/spaces/ROS/pages/47579160/API | **`ros` talks this API** (default TCP **8728** / TLS **8729**). Paths map to menu words; `ros get` runs print — never append `/print` or `/get` to paths. |
| REST API | https://help.mikrotik.com/docs/spaces/ROS/pages/332988582/REST+API | HTTP(S) `/rest` wrapper — **not** what `ros` uses. Do not translate REST examples into `ros` flags without mapping back to API menus. |

Safe sessions and journals are a **`ros` client** feature over the binary API (not RouterOS terminal Safe Mode). See `references/safety-and-recovery.md`.

## Topic entry points

| Area | Official entry | Typical `ros` touchpoints |
|------|----------------|---------------------------|
| WireGuard | https://help.mikrotik.com/docs/spaces/ROS/pages/69664792/WireGuard | `wg peers [--stale-after]`, `get wg/peers`, `get interface/wireguard` |
| Firewall (overview) | https://help.mikrotik.com/docs/spaces/ROS/pages/250708066/Firewall | `get firewall/filter\|nat\|mangle`, curated `firewall …` |
| Firewall filter | https://help.mikrotik.com/docs/spaces/ROS/pages/48660574/Filter | filter rules; address-list via `firewall address-list` / `get address-list` |
| DHCP | https://help.mikrotik.com/docs/spaces/ROS/pages/24805500/DHCP | `get dhcp/lease\|server\|network`, curated `dhcp` / `lease` |
| DNS | https://help.mikrotik.com/docs/spaces/ROS/pages/37748767/DNS | `get dns`, `dns static`, `get dns/static` |
| BGP | https://help.mikrotik.com/docs/spaces/ROS/pages/328220/BGP | `bgp sessions`, `get bgp/session` (v7 `/routing/bgp/…`) |
| OSPF | https://help.mikrotik.com/docs/spaces/ROS/pages/9863229/OSPF | `ospf neighbors`, `get ospf/neighbor` (v7 `/routing/ospf/…`) |
| WiFi (wifiwave2 / `/interface/wifi`) | https://help.mikrotik.com/docs/spaces/ROS/pages/224559120/WiFi | `wifi clients`, `get wifi/registration` — menu name varies by ROS/package; confirm on device |

Routing property pages (when needed): [BGP `/routing/bgp`](https://help.mikrotik.com/docs/spaces/ROS/pages/331612228/routing+bgp), [OSPF `/routing/ospf`](https://help.mikrotik.com/docs/spaces/ROS/pages/331612216/routing+ospf).

## Version caveats (verify on device + official changelog)

Do not invent enum values or defaults. Pattern:

1. Note the board’s ROS version (`ros -d DEV --read-only get system/resource` / identity audit).
2. Open the topic page **and** MikroTik release notes / changelog for that major.minor.
3. Prefer live `get` output over training data.

**Known `ros` encoding (re-check docs if unsure):** on RouterOS **≥7.17**, `/ip/cloud` `ddns-enabled` is treated as **`yes` \| `auto`** (`auto` ≈ off unless Back To Home); `ros` rejects `ddns-enabled=no` and normalizes `false` → `auto`. Confirm against current Cloud / changelog pages before proposing other values.

WiFi: modern boards use `/interface/wifi` (wave2+); older `wireless` / `wifiwave2` package layouts differ — use `ros domains` and a read-only `get` before writing.

## Related local refs

- `references/commands.md` — `ros` command cheat sheet
- `references/safety-and-recovery.md` — write/session failure map
- Repo docs: `docs/COMMANDS.md`, `docs/AGENTS.md`
