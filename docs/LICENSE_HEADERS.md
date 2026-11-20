---
hero:
  eyebrow: LICENSE HEADERS
  title: Source license headers
---

All **source files** in this repository carry SPDX metadata (`LicenseRef-Zyvor-Production-1.0`, Zyvor Production License v1.0) at the top of the file.

← [Docs hub](index.md)

---

## Required header

**Go, Rust-style comments on other languages:**

```text
Copyright 2026 Zyvor AI Labs · https://zyvor.dev
SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
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
        if "SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0" not in h:
            print(p)
PY
```

Full license text: [LICENSE](https://github.com/zyvorai/relay-edge/blob/main/LICENSE) in repo root.

---

## Related projects

| Repo | License |
|------|---------|
| relay | Zyvor Production License v1.0 |
| [relay-pubsub](https://github.com/zyvorai/relay-pubsub) | Zyvor Production License v1.0 |
| Zynera | Proprietary (different header) · [ZYNERA.md](ZYNERA.md) in this repo |
