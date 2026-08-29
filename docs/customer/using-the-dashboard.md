# Using the dashboard

## Purpose

| Area | Purpose |
|------|---------|
| **Configure** | Runtime gateway/Relay URLs, JWT, browser API token |
| **Self-test lab** | Health, Store, Send, Receive, simulator shortcuts |
| **Logs** | Admin ring buffer |
| **Fire-water / Remote edge / Fleet** | Simulator control rooms |
| **Docs** | In-app stack docs + first-time setup link |

## When to use it

- Open **Using the dashboard** when the job matches this screen
- Prefer the product home / Get started panel if you are unsure where to begin
- Confirm health and auth tokens if probes fail

## How to get there

- UI path: `/ui/` → **Using the dashboard** (or matching nav tab)
- Spotlight / in-app links when available

## Operate from the console (UX)

1. Open the relay-edge UI (`/ui/`) on `https://<host>:…` (see Admin basics for the default port).
2. Navigate to **Using the dashboard**.
3. Complete the on-screen fields / actions for this surface (| Area | Purpose |
|------|---------|
| **Configure** | Runtime gateway/Relay URLs, JWT, browser API token |
| **Self-te…).
4. Use **Probe** / **Save** / **Send** (or the primary button on the page) and watch status chips.
5. **Empty / fail:** Check Admin basics env vars, JWT/`API_TOKEN`, TLS insecure for lab certs, and backend reachability.
6. **Success:** Status shows healthy / accepted; related Lab or Logs surfaces reflect the change.

Never publish lab IPs — use `<host>`.

## Related pages

- [Getting Started](../../getting-started.md)
- [Using the Dashboard](../../using-the-dashboard.md)
- [Admin basics](../../admin-basics.md)
- [Page index](../../PAGE_INDEX.md)
