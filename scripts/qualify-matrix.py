#!/usr/bin/env python3
# Copyright 2026 Zyvor AI Labs · https://zyvor.dev
# SPDX-License-Identifier: Apache-2.0
"""Software qualification matrix for relay-edge."""
from __future__ import annotations

import json
import os
import pathlib
import subprocess
import sys
from datetime import datetime, timezone

ROOT = pathlib.Path(__file__).resolve().parents[1]
EVIDENCE = ROOT / "evidence" / "qualification"
VERSION = "0.1.1"


def run(cmd, **kwargs):
    return subprocess.run(cmd, cwd=ROOT, text=True, capture_output=True, **kwargs)


def row(results, name, status, detail=""):
    results.append({"id": name, "status": status, "detail": detail, "class": "software"})
    mark = "PASS" if status == "pass" else ("SKIP" if status == "skip" else "FAIL")
    print(f"[{mark}] {name}" + (f" — {detail}" if detail else ""))


def main():
    EVIDENCE.mkdir(parents=True, exist_ok=True)
    results = []
    started = datetime.now(timezone.utc).isoformat()
    fast = os.environ.get("RELAY_EDGE_QUALIFY_FAST", "") in ("1", "true", "yes")

    chart = (ROOT / "deploy/helm/relay-edge/Chart.yaml").read_text()
    if f'appVersion: "{VERSION}"' in chart and f"version: {VERSION}" in chart:
        row(results, "helm_version_lockstep", "pass", VERSION)
    else:
        row(results, "helm_version_lockstep", "fail", "Chart.yaml must match 0.1.1")

    proc = run(["sh", "-c", 'test -z "$(gofmt -l .)"'])
    row(results, "gofmt", "pass" if proc.returncode == 0 else "fail", (proc.stdout + proc.stderr)[-200:])

    proc = run(["go", "vet", "./..."], timeout=120)
    row(results, "go_vet", "pass" if proc.returncode == 0 else "fail", (proc.stdout + proc.stderr)[-300:])

    if fast:
        row(results, "unit_tests", "skip", "RELAY_EDGE_QUALIFY_FAST=1 — covered by CI")
    else:
        proc = run(["go", "test", "./..."], timeout=300)
        row(results, "unit_tests", "pass" if proc.returncode == 0 else "fail", (proc.stdout + proc.stderr)[-400:])

    proc = run(["make", "build"], timeout=120)
    row(results, "build_binary", "pass" if proc.returncode == 0 else "fail", (proc.stdout + proc.stderr)[-300:])

    for doc in ["docs/PRODUCTION.md", "docs/QUALIFICATION.md", "CHANGELOG.md"]:
        if not (ROOT / doc).is_file():
            row(results, "docs_present", "fail", f"missing {doc}")
            break
    else:
        row(results, "docs_present", "pass")

    for name, detail in [
        ("live_relay_accept", "requires Zyvor Relay on :8443 — lab smoke.sh 502 without Relay is expected"),
        ("auth_https_ci", "CI smoke-auth-tls job"),
        ("multi_replica_ha", "not claimed — single-writer JSON stores"),
    ]:
        row(results, name, "skip", detail)

    report = {
        "generated_at": started,
        "finished_at": datetime.now(timezone.utc).isoformat(),
        "product": "relay-edge",
        "version": VERSION,
        "host": os.uname().sysname if hasattr(os, "uname") else "unknown",
        "results": results,
        "software_pass": all(r["status"] == "pass" for r in results if r["status"] != "skip"),
        "ops_claimed": False,
        "note": "Synthetic companion — not a device manager. Live Relay Accept is operator/lab evidence.",
    }
    out = EVIDENCE / "software-matrix.json"
    out.write_text(json.dumps(report, indent=2) + "\n")
    print(f"\nwrote {out}")
    if not report["software_pass"]:
        sys.exit(1)
    print("software qualification rows passed; live Relay still required for full smoke.sh")


if __name__ == "__main__":
    main()
