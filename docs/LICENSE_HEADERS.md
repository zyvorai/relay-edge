# Source license headers

All **source files** in this repository carry Apache-2.0 SPDX metadata at the top of the file.

← [Docs hub](README.md)

---

## Required header

**Go, Rust-style comments on other languages:**

```text
Copyright 2026 Zyvor AI Labs
SPDX-License-Identifier: Apache-2.0
```

| File type | Prefix |
|-----------|--------|
| `.go` | `//` |
| `.sh`, `.py`, `Dockerfile` | `#` |
| `.html` | `<!-- … -->` |
| `.ts`, `.tsx`, `.js`, `.mjs` | `//` |

Place the header **before** `package`, shebang, or `<!doctype>` (shebang first on shell scripts, then header lines).

---

## Scope

| Included | Excluded |
|----------|----------|
| Go, shell, HTML UI, Dockerfile | Markdown docs (this file, guides) |
| Tests (`*_test.go`) | Generated code (`zz_generated.*`, `*.pb.go`) |
| Embedded web assets under `web/` | Third-party vendored trees |

---

## Verify locally

```bash
# List Go/shell/HTML files missing SPDX (relay-edge)
python3 - <<'PY'
import os
exts = {".go", ".sh", ".html"}
for root, _, files in os.walk("."):
    if ".git" in root.split(os.sep): continue
    for f in files:
        if os.path.splitext(f)[1] not in exts: continue
        p = os.path.join(root, f)
        h = open(p, encoding="utf-8", errors="ignore").read(800)
        if "SPDX-License-Identifier: Apache-2.0" not in h:
            print(p)
PY
```

Full license text: [LICENSE](../LICENSE) in repo root.

---

## Related projects

| Repo | License |
|------|---------|
| [relay](https://github.com/zyvorai/relay) | Apache-2.0 |
| [relay-pubsub](https://github.com/zyvorai/relay-pubsub) | Apache-2.0 |
| [forge](https://github.com/zyvorai/forge) | Proprietary (different header) · [FORGE.md](FORGE.md) in this repo |
