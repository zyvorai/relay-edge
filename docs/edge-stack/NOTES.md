# The Zyvor Edge Stack

A landing page advertising the eight-product Zyvor edge line — Zyvor
Device Agent, Nodra, Zyvor Fleet, Zyvor OTA, relay-edge, relay-pubsub,
Zyvor Relay, and Zynera — with quotes, versions, and diagrams pulled
directly from each product's own docs.

- `index.html` — the standalone page (self-contained, embedded fonts/images)
- `zyvor-edge-stack.pdf` — a print export of the same page

Published copy (updates independently of this checked-in snapshot):
https://claude.ai/artifact/F9gWSGPwWZUtyCLHHLny6a

Regenerate the PDF after editing `index.html`:

```bash
"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
  --headless --disable-gpu --no-pdf-header-footer \
  --print-to-pdf=docs/edge-stack/zyvor-edge-stack.pdf \
  --virtual-time-budget=5000 \
  "file://$(pwd)/docs/edge-stack/index.html"
```

Not wired into the MkDocs nav — this is cross-product marketing content,
not relay-edge's own documentation, checked in here for safekeeping.
