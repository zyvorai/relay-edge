# Page-by-page guides

Each guide follows: Purpose → When to use it → How to get there → Operate from the console (UX) → Related pages.

Every route is also listed in the [complete page index](../PAGE_INDEX.md).

## Guides

| Page | What it covers |
|------|----------------|
| [Configure](guides/configure.md) | Set gateway or direct Relay URLs, project id, TLS insecure, tokens. Probe backends. Reopen setup guide from here. |
| [Docs](guides/docs.md) | Embedded verification notes and link back to first-time setup on `/ui/`. |
| [Firewater](guides/firewater.md) | Seed plant inventory, enable Publish into Relay, run scenarios (lowtank, fire, comms, vision, gas). |
| [Fleet](guides/fleet.md) | Multi-class IoT scenarios: blackout, amr_lost, ot_storm, spill, heatwave, intrusion. |
| [Self-test lab](guides/lab.md) | Steps: Health, Store, Send, Receive, then firewater / remote-edge / fleet shortcuts. |
| [Remote edge](guides/remote-edge.md) | NOC scenarios: sat_down, offline, gpu_hot, intrusion, flood, drone_patrol. |

---

6 guides. Regenerate: `node scripts/customer-docs/generate-guide-index.mjs`.
