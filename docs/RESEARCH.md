# Research: IoT simulation, digital twins and event streaming

How the surrounding field compares with relay-edge, what relay-edge lacked,
and the ranked ideas that shaped the 0.8 feature set. Claims marked **[U]**
were not read from a fetched page in this pass (recalled or inferred) and
should be verified before being quoted externally.

## What the field does

| Product / standard | What it does | Source |
|---|---|---|
| AWS IoT Device Simulator | GUI engine simulating thousands of user-defined devices to assess AWS IoT services without hardware. Marked **deprecated as of 2025-01-29**, test-only, AWS-native stack. Device types and replay features were not described on the fetched page. | https://github.com/aws-solutions/iot-device-simulator |
| Eclipse Ditto | Digital twins as "Things" holding last-known state and metadata, cached so apps can read state while a device is offline; HTTP/WebSocket/messaging APIs; per-Thing policies for authorization; assumes devices are already connected (for example via Eclipse Hono). | https://eclipse.dev/ditto/intro-overview.html |
| Azure Digital Twins | Twin graphs from DTDL models (state, components, relationships), event routes to Event Hubs / Event Grid / Service Bus, a query API, and data history into Azure Data Explorer. | https://learn.microsoft.com/en-us/azure/digital-twins/overview |
| CloudEvents 1.0 | Vendor-neutral event envelope. JSON format: single event `application/cloudevents+json`, batch `application/cloudevents-batch+json` (an array); `data` is stored as a JSON value when the content type is JSON; RFC 3339 `time`. Latest listed release v1.0.2. The required attribute list (`id`, `source`, `specversion`, `type`) was not enumerated on the pages fetched, so treat it as **[U]**. | https://cloudevents.io/ , https://github.com/cloudevents/spec/blob/v1.0.2/cloudevents/formats/json-format.md |
| NATS CLI bench | Load generation with `--msgs` (count) and `--clients` (concurrency) confirmed; a message-size option exists (default 128 B) but its flag name was not shown. | https://docs.nats.io/using-nats/nats-tools/nats_cli/natsbench |
| ThingsBoard | Device APIs over MQTT, CoAP, HTTP and SNMP plus device SDKs; whether it ships a first-party simulator was **not confirmed** on the page fetched **[U]**. | https://thingsboard.io/docs/samples/ |
| Sparkplug B, OPC UA PubSub | Industrial publish/subscribe payload and topic conventions (birth/death certificates; UADP/JSON over MQTT or UDP). Not fetched **[U]**. relay-edge already exposes a `/v1/firewater/sparkplug` view. | (not fetched) |
| Kafka / MQTT load generators, chaos tools for pipelines, deterministic replay of recorded streams | Common practice: seeded generators, fault injection (drop, duplicate, delay, reorder) and recorded-stream replay with time scaling. Not fetched **[U]**. | (not fetched) |

## What relay-edge lacked (before 0.8)

1. **Not reproducible.** Every simulator drew from the global `math/rand`
   source and idempotency keys embedded `time.Now().UnixNano()`, so two runs
   of the same scenario never produced the same event stream. A regression in
   Relay policy matching could not be bisected against a stable input.
2. **No memory of what was sent.** Published events were fire-and-forget; the
   per-family event logs held simulator-side events but not the stamped
   payload as sent, and nothing could be exported.
3. **No replay.** A recorded or hand-written stream could not be pushed back
   into Relay with its original pacing.
4. **No fault injection on the publish path.** Relay idempotency, ordering and
   retry behaviour could only be tested with a real flaky network.
5. **No standard envelope.** No way to export events as CloudEvents for other
   tools in the pipeline.

## Ranked ideas

| Rank | Idea | Why | Built in 0.8 |
|---|---|---|---|
| 1 | Seeded deterministic simulation (`EDGE_SIM_SEED`, `POST /v1/sim/seed`) | Reproducibility is table stakes for regression-testing event pipelines; cheap, touches only the three engines. | yes |
| 2 | Event journal with NDJSON and CloudEvents export | Prerequisite for replay and for diffing runs; CloudEvents is the neutral envelope (cloudevents.io). | yes |
| 3 | Time-scaled replay with deterministic idempotency keys | Replaying the same journal twice with the same `run_id` must be a no-op for a correct Relay; a speed factor mirrors recorded-stream replay tools **[U]**. | yes |
| 4 | Seeded chaos on the publish path (drop, duplicate, delay/jitter, error) | Exercises Relay dedup and ordering without a network lab; seeded so a failing run can be repeated. | yes |
| 5 | Twin state store with policies (Ditto style) | Valuable, but relay-edge deliberately publishes events only and Relay owns state; it would blur the stated division of labor. | rejected |
| 6 | DTDL model import (Azure style) | Large surface, new parser or dependency, no identified user. | rejected |
| 7 | MQTT / Kafka / NATS / OPC UA PubSub transports | relay-pubsub owns the wire; adding transports would break the "relay-pubsub or direct" boundary and add dependencies. | rejected |
| 8 | Sparkplug B birth/death certificate emulation | Plausible next step for the firewater family; needs a protobuf dependency. | deferred |
| 9 | Throughput benchmark mode (NATS bench style) | Replay with `speed` up to 1000 covers moderate rates; a real load generator needs concurrency control and backpressure design. | deferred |

## Round 2 additions

Built on the deferred and rejected list above, staying inside the same
boundaries (no new dependency, no durable state, publish-only):

- **Journal diff** (baseline vs run): the stated purpose of the journal was
  "diffing runs"; seeded runs make a content diff meaningful. Read-only.
- **Replay filters** (`types`, `severities`): replaying only the critical or
  one-family slice of a recording is the common way to isolate a Relay policy.
- **Replay `repeat` and latency stats**: a modest step toward the deferred
  throughput mode, without concurrency or backpressure design.

## Round 3 additions

Pages read in this round: Toxiproxy README (github.com/Shopify/toxiproxy),
Grafana k6 ramping-arrival-rate docs, Stripe CLI `trigger` docs. Search
results only (page not opened): Azure IoT telemetry simulators, Eclipse Hono
load-test posts, Svix "send example", Stripe resend.

| Source | Takeaway | Built |
|---|---|---|
| Toxiproxy: `packet_loss` has `loss_rate` and `correlation` (bursts); every toxic has `toxicity` and stream scope | Loss in real networks is bursty; faults should be scopable | `correlation_pct`, chaos `types` scope |
| k6 `ramping-arrival-rate`: stages of target rate and duration, linear ramp, start rate, open model (starts independent of response time) | Load shape matters as much as volume; a rate profile beats a fixed speed factor | `rate_per_sec`, `rate_stages`, `start_rate` |
| Stripe CLI `trigger --override/--add/--remove`, `events resend` | Re-send one real event with targeted edits is the fastest way to probe a consumer | `POST /v1/journal/resend` with dotted-path `set` / `remove` |
| Toxiproxy has no reorder toxic (none found in its toxic list) | Out-of-order delivery is still a gap in common tooling | `shuffle_window` / `shuffle_seed` on replay |
| Telemetry simulators (Azure, Hono tests) report volume and latency | Operators want a quick summary of what was sent | `GET /v1/journal/stats` |

Still **[U]** (not fetched): Azure/Hono feature details beyond search snippets.
Open model here means pacing by schedule; relay-edge remains a single
sequential publisher, so a slow Relay stretches the schedule rather than
queueing (no coordinated-omission correction).

## Boundaries respected

relay-edge only stamps and publishes; it never calls Zynera, owns no durable
event log and never actuates. The journal is an in-memory bounded ring (not a
durable log), replay publishes through the same Relay/gateway client, and no
new dependency was added.
