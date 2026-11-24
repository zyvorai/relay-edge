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
| `types` | all | keep only these event types; a trailing `*` is a prefix (`["fleet.*"]`). Query form: `types=a.*,b.c` |
| `severities` | all | keep only these severities (`["critical"]`). Query form: `severities=critical,warning` |
| `repeat` | `1` | play the stream N times back to back (1 to 100, events x repeat at most 100000). Pass 1 keys are `replay/<run_id>/<n>`, later passes `replay/<run_id>/p<pass>/<n>`, so each pass is a distinct but repeatable set of events: a cheap load and soak mode |

| `rate_per_sec` | off | open-model pacing: publish at a constant 0.01 to 1000 events/s and ignore recorded timestamps. Exclusive with `speed`. Query form: `rate_per_sec=50` |
| `rate_stages`, `start_rate` | off | ramp profile, k6 ramping-arrival-rate style: `[{"duration_ms":10000,"rate_per_sec":50}]` moves linearly from `start_rate` (default: the first stage's rate) to each stage's rate in turn and holds the last rate once the stages end (at most 20 stages, 1 ms to 1 h each). The clock runs across `repeat` passes. Each gap is still capped by `max_gap_ms` |
| `shuffle_window`, `shuffle_seed` | off | permute events inside consecutive windows of N (2 to 1000) with a seeded shuffle to test Relay's handling of out-of-order arrival. Pacing and each event's key (by original position) are unchanged, so the same seed replays the same disorder |

`GET /v1/replay` also reports `pacing` (`recorded` or `rate`), `repeat`, `rate_per_sec` and `latency_ms`
(`min`, `p50`, `p95`, `max` of the publish call, nearest-rank) once events have
been attempted, which makes a repeated fast replay a small throughput check of
the path to Relay. This is not a load generator: it is one sequential publisher.

```bash
# ramp from 1 to 100 events/s over 30 s, then hold, shuffled in windows of 8
curl -X POST $EDGE/v1/replay -d '{"from_journal":true,"start_rate":1,
  "rate_stages":[{"duration_ms":30000,"rate_per_sec":100}],"shuffle_window":8,"shuffle_seed":1}'
```

### Journal stats

`GET /v1/journal/stats?after=&origin=&types=&severities=` counts the retained
events by type, severity, origin and outcome (`pending` = outcome not yet
recorded, for example a delayed chaos send) with first/last sequence, span and
events per second. Read-only.

### Resending one event, with edits

`POST /v1/journal/resend` re-publishes a single retained event, optionally
edited, through the same chaos-aware publish path (Stripe CLI `resend` /
`trigger --override` idea). New events are journaled with origin `resend`.

```bash
curl -X POST $EDGE/v1/journal/resend -d '{"seq":17,"severity":"critical",
  "set":{"pose.x":5,"flag":true},"remove":["debug"],"idempotency_key":"fix-17","count":1}'
```

`set` / `remove` take dotted paths into `data` (objects only, up to 8 levels
and 64 edits; a path through a scalar is a 400). `type`, `severity` and
`source` can be overridden and are validated like imported events. Without an
`idempotency_key`, keys are `resend/<seq>/<ts>` (or seeded and deterministic
when a sim seed is set); with `count` 2 to 100 an explicit key gets `-<n>`
appended. Sending the same explicit key twice tests Relay dedup. `404` if the
sequence number has rotated out, `502` if every send failed. It never
touches the original entry.

### Diffing runs

`POST /v1/journal/diff` is the regression check: did this run publish the same
events as the baseline. It is read-only and publishes nothing.

```bash
curl -s $EDGE/v1/journal/export?origin=sim > baseline.ndjson   # earlier run
# ... re-run the scenario with the same seed ...
jq -Rs "{expected: ., origin: \"sim\"}" baseline.ndjson | curl -s -X POST $EDGE/v1/journal/diff -d @-
```

`expected` (and optionally `actual`) accept anything the replay body accepts, or
a JSON string holding NDJSON. Without `actual`, the live journal slice chosen by
`from_seq` / `to_seq` / `origin` is used. Events are aligned by position and
compared on type, severity, source and data; `seq`, time and outcome are
ignored, and idempotency keys only with `"include_key": true`. The result has
`identical`, counts of `changed` / `missing` / `unexpected`, `reordered` (same
events, different order) and up to 100 itemised differences with the differing
field names.

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
| `correlation_pct` | 0 to 100 | Bursty loss (Toxiproxy `packet_loss` correlation idea): the drop probability is mixed with "the previous event was dropped", so 0 is independent loss and 100 keeps dropping once a drop starts |
| `types` | up to 32 | Scope the faults to matching event types (exact or trailing `*`); everything else passes untouched, is counted in `skipped` and consumes no randomness |
| `seed` | any int64 | Same seed and same event order give the same decisions |

Unknown fields and out-of-range values are rejected with 400. At most 1000
delayed events are held; beyond that they are sent immediately. Counters are
also exported as `relay_edge_publish_decisions_total{action}`.

## UI

`/ui` -> **Pipeline** has buttons for seed, chaos presets, journal export and
replay-from-journal, a bursty scoped-chaos preset, journal stats, resend-last-event, filtered / ramped / shuffled replay and cancel.
