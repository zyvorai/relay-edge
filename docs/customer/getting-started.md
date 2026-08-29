# Getting started — relay-edge

Full stack order: **[Relay stack day-0 onboarding](/docs/relay-stack-onboarding)**.

1. Open `https://<edge-host>:18086/ui/` (accept self-signed TLS).
2. Complete the **Get started** panel: Health → Auth (if required) → Wire publish → Probe → Lab.
3. In **Configure**, set `gateway_base_url` to your **pubsub host** (or clear it and set `relay_base_url` for direct). Paste JWT. Enable TLS insecure for lab certs. **Save**, then **Probe**.
4. Run Lab **Store → Send → Receive**.

Do **not** put laptop `127.0.0.1` in peer URLs when Relay or pubsub run on another machine.
