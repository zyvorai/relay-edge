#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs
# SPDX-License-Identifier: Apache-2.0
# Backup EDGE_DATA_DIR (JSON stores + optional TLS material).
# Usage: EDGE_DATA_DIR=./data ./scripts/backup-data.sh [outfile.tgz]
set -euo pipefail
DATA_DIR="${EDGE_DATA_DIR:-./data}"
OUT="${1:-}"
TS=$(date -u +%Y%m%dT%H%M%SZ)
if [[ -z "$OUT" ]]; then
  OUT="relay-edge-data-${TS}.tgz"
fi
if [[ ! -d "$DATA_DIR" ]]; then
  echo "error: EDGE_DATA_DIR not a directory: $DATA_DIR" >&2
  exit 1
fi
ABS=$(cd "$DATA_DIR" && pwd)
tar -C "$(dirname "$ABS")" -czf "$OUT" "$(basename "$ABS")"
echo "wrote $OUT (from $ABS)"
