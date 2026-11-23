---
hero:
  eyebrow: PIPELINE
  title: Deterministic runs, journal, replay and chaos
---

relay-edge can make a simulator run **reproducible**, **record** every stamped
event it hands to the publish path, **replay** a recorded or hand-written
stream into Relay, and **inject seeded faults** on the way out. Together they
let you bisect a Relay policy regression against a stable input and test
Relay's idempotency and ordering without a flaky network.

← [Docs hub](index.md) · Research background: [RESEARCH.md](RESEARCH.md)

---

## Boundaries

- relay-edge still only stamps and publishes. The journal is an **in-memory
  ring** (default 5000 events, `EDGE_JOURNAL_MAX`, hard max 100000), not a
  durable log; Relay owns durability. It is lost on restart.
- Replay and chaos apply to the **simulator** publish path and to replay itself.
  Direct farm calls (`POST /v1/seasons/{id}/events`, advisories) still return
  Relay's answer to the caller and are not journaled or fault-injected.
- Nothing here calls Zynera or actuates anything.

## 1. Deterministic simulation

Set a seed and the three simulators draw their noise from per-engine seeded
sources, and idempotency keys become `edge/<family>/<type>/<device>/s<seed>-<n>`
instead of embedding the wall clock.

```bash
curl -X POST $EDGE/v1/sim/seed -d '{"seed":42}'     # deterministic
curl -X POST $EDGE/v1/sim/seed -d '{"seed":null}'   # back to non-deterministic
curl $EDGE/v1/sim/seed
```

`EDGE_SIM_SEED=42` sets it at start. Re-posting the same seed restarts the
sequence: the same seed, the same scenario and the same sequence of ticks give
identical values and keys. Timestamps (`created_at`) and the running ticker's
wall-clock cadence are not part of the guarantee, so drive runs with
`POST /v1/<family>/tick` when you need byte-identical output.

## 2. Journal and export

Every event that goes through the simulator publish path is journaled with its
stamped payload, its origin (`sim` or `replay`) and its outcome (`published`,
`dropped`, `duplicated`, `error`).

| Method | Path | Purpose |
|---|---|---|
| GET | `/v1/journal?after=N&limit=100&origin=sim` | Page through entries (limit max 1000) |
| GET | `/v1/journal/export?format=ndjson\|cloudevents&from=&to=&limit=&origin=` | Download (limit max 10000) |
| POST | `/v1/journal/clear` | Drop entries; sequence numbers keep increasing |

`format=cloudevents` emits a **CloudEvents 1.0** batch
(`application/cloudevents-batch+json`): `id` is the idempotency key, `source`
is `relay-edge/<url-escaped source>`, plus `severity` and `sequence` extension
attributes and the stamped payload as `data`.

## 3. Replay

```bash
# replay a recording at 10x, key namespace "golden"
curl -X POST $EDGE/v1/replay -d '{"from_journal":true,"origin":"sim","speed":10,"run_id":"golden"}'

# replay an exported file (NDJSON or a CloudEvents batch)
curl -X POST "$EDGE/v1/replay?speed=100&run_id=golden" \
     -H 'Content-Type: application/x-ndjson' --data-binary @events.ndjson

# plan only
curl -X POST $EDGE/v1/replay -d '{"events":[{"type":"fleet.power.island","severity":"critical"}],"dry_run":true}'

curl $EDGE/v1/replay                  # status
curl -X POST $EDGE/v1/replay/cancel
```

| Field | Default | Bound |
|---|---|---|
| `events` / `from_journal` (+`from_seq`, `to_seq`, `origin`) | required | 10000 events per run (`max_events` lowers it; 413 above) |
| `speed` | `1` | 0.01 to 1000 |
| `max_gap_ms` | `5000` | 0 to 30000, cap on any single pause after scaling |
| `run_id` | content hash | letters, digits, `-_.`, up to 64 chars |
| `dry_run` | `false` | returns the first 100 planned steps, publishes nothing |

Events keep their recorded relative timing (`at` / CloudEvents `time`);
missing or backwards timestamps replay with no pause. Keys are
`replay/<run_id>/<position>`, so **replaying the same stream with the same
`run_id` twice must not create new events in a Relay that deduplicates** on
idempotency key. One run at a time (a second returns 409). Imported events are
validated: the type must match `^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$` (it becomes
a gateway URL segment), severity is a lowercase word, source and key have no
control characters, `data` must be a JSON object, each event is at most 64 KiB,
and the whole body is bounded by `EDGE_MAX_BODY_BYTES`.

## 4. Chaos on the publish path

```bash
curl -X POST $EDGE/v1/chaos/config -d '{"enabled":true,"seed":7,"drop_pct":10,"duplicate_pct":5,"error_pct":2,"delay_ms":50,"jitter_ms":200}'
curl $EDGE/v1/chaos        # config, counters, delayed events still pending
curl -X POST $EDGE/v1/chaos/reset
```

| Knob | Bound | Effect |
|---|---|---|
| `drop_pct` | 0 to 100 | Event journaled as `dropped`, never sent |
| `error_pct` | 0 to 100 | Publish reported failed, never sent |
| `duplicate_pct` | 0 to 100 | Sent twice with the same key (tests dedup) |
| `delay_ms`, `jitter_ms` | 0 to 10000 each | Sent later on a timer; different delays reorder events |
| `seed` | any int64 | Same seed and same event order give the same decisions |

Unknown fields and out-of-range values are rejected with 400. At most 1000
delayed events are held; beyond that they are sent immediately. Counters are
also exported as `relay_edge_publish_decisions_total{action}`.

## UI

`/ui` -> **Pipeline** has buttons for seed, chaos presets, journal export and
replay-from-journal.
