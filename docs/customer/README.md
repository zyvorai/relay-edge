# relay-edge — Customer Documentation

**relay-edge** stamps site context and runs farm / firewater / remote-edge / fleet simulators that publish into Relay (via pubsub or direct).

| You want to… | Open |
|--------------|------|
| Configure the full Relay stack (day-0) | [Stack onboarding](/docs/relay-stack-onboarding) |
| First install on `/ui/` | [Getting Started](getting-started.md) |
| Home, Configure, Lab | [Using the Dashboard](using-the-dashboard.md) |
| Screen guides | [Page-by-page](pages/README.md) |
| URL lookup | [Page index](PAGE_INDEX.md) |
| Env and TLS | [Admin basics](admin-basics.md) |
| Store → Send → sims | [Workflows](workflows.md) |

**→ [Docs one-pager](https://zyvor.dev/docs/relay-edge)** · **[GitHub](https://github.com/zyvorai/relay-edge)** · **[Relay](https://zyvor.dev/docs/zyvor-relay)** · **[pubsub](https://zyvor.dev/docs/relay-pubsub)**

## Printable PDFs

```bash
node scripts/customer-docs/build-customer-pdfs.mjs
```

Output lands in [`pdf/`](pdf/):

- `relay-edge-Customer-README.pdf`
- `relay-edge-Getting-Started.pdf`
- `relay-edge-Page-by-Page.pdf`
- `relay-edge-Admin-Basics.pdf`

---

*Zyvor · relay-edge · Apache-2.0*
