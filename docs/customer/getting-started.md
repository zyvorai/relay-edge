# Getting started — relay-edge

## Purpose

Full stack order: **[Relay stack day-0 onboarding](/docs/relay-stack-onboarding)**.
1. Open `https://<edge-host>:18086/ui/` (accept self-signed TLS).
2. Complete the **Get started** panel: Health → Auth (if required) → Wire publish → Probe → Lab.
3. In **Configure**, set `gateway_base_url` to your **pubsub host** (or clear it and set `relay_base_url` for direct). Paste JWT. Enable TLS insecure for lab certs. **Save**, then **Probe**.
4. Run Lab **Store → Send → Receive**.
Do **not** put laptop `127.0.0.1` in peer URLs when Relay or pubsub run on another machine.

## When to use it

- Open **Getting started — relay-edge** when the job matches this screen
- Prefer the product home / Get started panel if you are unsure where to begin
- Confirm health and auth tokens if probes fail

## How to get there

- UI path: `/ui/` → **Getting started — relay-edge** (or matching nav tab)
- Spotlight / in-app links when available

## Operate from the console (UX)

1. Open the relay-edge UI (`/ui/`) on `https://<host>:…` (see Admin basics for the default port).
2. Navigate to **Getting started — relay-edge**.
3. Complete the on-screen fields / actions for this surface (Full stack order: **[Relay stack day-0 onboarding](/docs/relay-stack-onboarding)**.
1. Open `https://<edge-host>:18086/u…).
4. Use **Probe** / **Save** / **Send** (or the primary button on the page) and watch status chips.
5. **Empty / fail:** Check Admin basics env vars, JWT/`API_TOKEN`, TLS insecure for lab certs, and backend reachability.
6. **Success:** Status shows healthy / accepted; related Lab or Logs surfaces reflect the change.

Never publish lab IPs — use `<host>`.

## Related pages

- [Getting Started](../../getting-started.md)
- [Using the Dashboard](../../using-the-dashboard.md)
- [Admin basics](../../admin-basics.md)
- [Page index](../../PAGE_INDEX.md)
