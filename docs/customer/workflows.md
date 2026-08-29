# Common workflows

1. **Store → Send → Receive** — Lab creates domain rows, publishes stamped `crop.advisory`, shows path.
2. **Simulator** — Seed → Publish into Relay → scenario → SSE stream.
3. **Full stack e2e** — From a workstation with remote URLs:

```bash
set -a && source config/lab-stack.env && set +a
./scripts/e2e-stack.sh
```
