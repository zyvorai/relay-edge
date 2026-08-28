# Forge at the edge

**→ Full three-way guide: [relay-edge + Forge + Relay integration](INTEGRATION.md)**

This page is a short index. The authoritative walkthrough for how **relay-edge**, **Forge**, and **Relay** work together — including sequence diagrams, configuration, and demo recipes — is **[INTEGRATION.md](INTEGRATION.md)**.

---

## Quick reference

| Product | Role |
|---------|------|
| **relay-edge** | Stamp and publish site-aware events |
| **Relay** | Notify · Ack · Act · Verify |
| **Forge** | AI/K8s at edge · optional Decision Records |

relay-edge **does not call Forge**. Relay opens Decision Records when policy sets `decision_backend: forge`.

Configure Forge on **Relay**:

```bash
RELAY_FORGE_BASE_URL=http://<forge-host>:30631
RELAY_FORGE_API_KEY=<forge-api-gateway-secret>
```

Cross-repo: [forge RELAY_STACK](https://github.com/zyvorai/forge/blob/main/docs/integrations/RELAY_STACK.md) · [relay ARCHITECTURE](https://github.com/zyvorai/relay/blob/main/docs/ARCHITECTURE.md#approval-backends-native-vs-forge)

← [Docs hub](README.md)
