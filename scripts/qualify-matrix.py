#!/usr/bin/env python3
# Copyright 2026 Zyvor AI Labs · https://zyvor.dev
# SPDX-License-Identifier: Apache-2.0
"""Software qualification matrix for relay-edge."""
from __future__ import annotations

import json
import os
import pathlib
import re
import shutil
import subprocess
import sys
from datetime import datetime, timezone

ROOT = pathlib.Path(__file__).resolve().parents[1]
EVIDENCE = ROOT / "evidence" / "qualification"
VERSION = "0.1.2"


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

    # Three independently-maintained "current version" literals — Makefile,
    # this script, and the Helm chart — must all agree with each other.
    # (The release workflow's git-tag-derived version is orthogonal: tags
    # are computed dynamically at cut time, so there's nothing in-tree to
    # lock them to until a release is actually cut.)
    makefile_text = (ROOT / "Makefile").read_text()
    m = re.search(r"^VERSION\s*\?=\s*(\S+)", makefile_text, re.M)
    makefile_version = m.group(1) if m else None

    chart = (ROOT / "deploy/helm/relay-edge/Chart.yaml").read_text()
    chart_version_m = re.search(r"^version:\s*(\S+)", chart, re.M)
    chart_app_version_m = re.search(r'^appVersion:\s*"?([\w.\-]+)"?', chart, re.M)
    chart_version = chart_version_m.group(1) if chart_version_m else None
    chart_app_version = chart_app_version_m.group(1) if chart_app_version_m else None

    versions = {
        "Makefile": makefile_version,
        "qualify-matrix.py": VERSION,
        "Chart.yaml:version": chart_version,
        "Chart.yaml:appVersion": chart_app_version,
    }
    if len(set(versions.values())) == 1:
        row(results, "version_lockstep", "pass", VERSION)
    else:
        row(results, "version_lockstep", "fail", f"mismatched versions: {versions}")

    proc = run(["sh", "-c", 'test -z "$(gofmt -l .)"'])
    row(results, "gofmt", "pass" if proc.returncode == 0 else "fail", (proc.stdout + proc.stderr)[-200:])

    proc = run(["go", "vet", "./..."], timeout=120)
    row(results, "go_vet", "pass" if proc.returncode == 0 else "fail", (proc.stdout + proc.stderr)[-300:])

    if fast:
        row(results, "unit_tests", "skip", "RELAY_EDGE_QUALIFY_FAST=1 — covered by CI")
        row(results, "unit_race", "skip", "RELAY_EDGE_QUALIFY_FAST=1 — covered by CI")
    else:
        proc = run(["go", "test", "./..."], timeout=300)
        row(results, "unit_tests", "pass" if proc.returncode == 0 else "fail", (proc.stdout + proc.stderr)[-400:])
        proc = run(["go", "test", "-race", "./..."], timeout=600)
        row(results, "unit_race", "pass" if proc.returncode == 0 else "fail", (proc.stdout + proc.stderr)[-400:])

    # govulncheck: prefer installed binary; otherwise go run (network on first use).
    # subprocess raises FileNotFoundError when the binary is absent — check first.
    if shutil.which("govulncheck"):
        vuln = run(["govulncheck", "./..."], timeout=180)
    else:
        vuln = run(
            ["go", "run", "golang.org/x/vuln/cmd/govulncheck@v1.8.0", "./..."],
            timeout=300,
        )
    if vuln.returncode == 0:
        row(results, "govulncheck", "pass")
    elif fast and "network" in ((vuln.stderr or "") + (vuln.stdout or "")).lower():
        row(results, "govulncheck", "skip", "network unavailable in fast mode")
    else:
        detail = ((vuln.stdout or "") + (vuln.stderr or ""))[-400:]
        row(results, "govulncheck", "fail", detail)

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
        ("multi_replica_ha", "not claimed — single-writer JSON stores"),
    ]:
        row(results, name, "skip", detail)

    for name, env_key, detail in [
        ("auth_https_ci", "RELAY_EDGE_AUTH_HTTPS_CI", "CI smoke-auth-tls job"),
        ("ci_backup_restore", "RELAY_EDGE_CI_BACKUP", "scripts/ci/backup-restore.sh"),
        ("ci_helm_manifests", "RELAY_EDGE_CI_HELM", "helm lint/template + kubeconform"),
        ("ci_kind_edge", "RELAY_EDGE_CI_KIND", "kind + chart smoke"),
    ]:
        val = os.environ.get(env_key, "")
        if val in ("1", "true", "pass", "yes"):
            row(results, name, "pass", detail)
        else:
            row(results, name, "skip", f"set {env_key}=1 after CI; {detail}")

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
