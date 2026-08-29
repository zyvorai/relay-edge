# Self-test lab

## Purpose

Steps: Health, Store, Send, Receive, then firewater / remote-edge / fleet shortcuts.

## When to use it

- Open **Self-test lab** when the job matches this screen
- Prefer the product home / Get started panel if you are unsure where to begin
- Confirm health and auth tokens if probes fail

## How to get there

- UI path: `/ui/` → **Self-test lab** (or matching nav tab)
- Spotlight / in-app links when available

## Operate from the console (UX)

1. Open the relay-edge UI (`/ui/`) on `https://<host>:…` (see Admin basics for the default port).
2. Navigate to **Self-test lab**.
3. Complete the on-screen fields / actions for this surface (Steps: Health, Store, Send, Receive, then firewater / remote-edge / fleet shortcuts.).
4. Use **Probe** / **Save** / **Send** (or the primary button on the page) and watch status chips.
5. **Empty / fail:** Check Admin basics env vars, JWT/`API_TOKEN`, TLS insecure for lab certs, and backend reachability.
6. **Success:** Status shows healthy / accepted; related Lab or Logs surfaces reflect the change.

Never publish lab IPs — use `<host>`.

## Related pages

- [Getting Started](../../getting-started.md)
- [Using the Dashboard](../../using-the-dashboard.md)
- [Admin basics](../../admin-basics.md)
- [Page index](../../PAGE_INDEX.md)
