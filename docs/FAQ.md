---
hero:
  eyebrow: FAQ
  title: FAQ
---

Questions people evaluating relay-edge actually ask, before they've
decided to adopt it.

## Licensing & cost

**Is it really free?** Yes. Apache-2.0 — use, modify, and run it for
personal, lab, and commercial production use at no charge, subject to
preserving notices. See the README's [License](https://github.com/zyvorai/relay-edge/blob/main/README.md#license)
section.

**What does "Enterprise" mean here?** Production support, SLAs, and
Zyvor's other commercial products are licensed separately. Contact
sales@zyvor.dev. Nothing in this repository requires it.

## Support

**What if I find a bug?** Open a GitHub issue.

**What if I find a security vulnerability?** See [`SECURITY.md`](https://github.com/zyvorai/relay-edge/blob/main/SECURITY.md)
— report to security@zyvor.ai. Its own text warns: without
`EDGE_API_TOKEN` set, relay-edge "exposes an unauthenticated HTTP API
(lab-friendly). Harden before production."

## Scope

**Does relay-edge manage real IoT devices?** No — it's a synthetic
site/topology/event generator for developing and demoing against Zyvor
Relay, not a real device-fleet management product. Sites, zones, devices,
and contacts are JSON-on-disk domain objects it stamps events for, not
connections to physical hardware.

**Does relay-edge talk to Forge?** No — "relay-edge only publishes
events — it never calls Forge" (README). At sites that also run Forge,
it's Relay itself that optionally gates critical acts behind Forge
Decision Records; relay-edge's role stops at publishing events. See
[`docs/FORGE.md`](FORGE.md).

**How does it relate to relay-pubsub?** Optional but preferred — events
can publish through [relay-pubsub](https://github.com/zyvorai/relay-pubsub)'s
Google Pub/Sub-compatible gateway, or go straight to Relay's
`POST /v1/events` when `GATEWAY_BASE_URL` is unset.

## Production readiness

**What version is this?** Current tagged line is **v0.1.2** (see
[`CHANGELOG.md`](https://github.com/zyvorai/relay-edge/blob/main/CHANGELOG.md)
and GitHub Releases). This is an engineering preview / lab-proven companion,
not a 1.0 device-management product. Lab host runs with `EDGE_REQUIRE_AUTH=1`
when used as an intentional feeder; defaults without token stay lab-open.

**Before calling it a production feeder?** Run `make qualify` and complete
[`docs/PRODUCTION.md`](PRODUCTION.md) / [`docs/QUALIFICATION.md`](QUALIFICATION.md).

**Is the default configuration production-safe?** No — read
[`SECURITY.md`](https://github.com/zyvorai/relay-edge/blob/main/SECURITY.md)'s hardening checklist first. The
unauthenticated-by-default HTTP API is explicitly called out as
"lab-friendly," not a production default.

## What it actually does

**What are the "four IoT simulators"?** Farm, Firewater (an NFPA-style
plant simulator), Remote-edge (satellite/compute/UAV/vision NOC), and
Fleet — each with its own browser control room under `/ui`. See
[`docs/SIMULATORS.md`](SIMULATORS.md).
