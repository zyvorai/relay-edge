#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs
# SPDX-License-Identifier: Apache-2.0
# Restore a backup created by scripts/backup-data.sh into EDGE_DATA_DIR.
# Usage: EDGE_DATA_DIR=./data ./scripts/restore-data.sh relay-edge-data-….tgz
set -euo pipefail
DATA_DIR="${EDGE_DATA_DIR:-./data}"
ARCHIVE="${1:-}"
if [[ -z "$ARCHIVE" || ! -f "$ARCHIVE" ]]; then
  echo "usage: EDGE_DATA_DIR=./data $0 <backup.tgz>" >&2
  exit 1
fi
mkdir -p "$DATA_DIR"
PARENT=$(cd "$(dirname "$DATA_DIR")" && pwd)
BASE=$(basename "$DATA_DIR")
# Archive contains a single top-level directory named like the data dir basename.
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
tar -xzf "$ARCHIVE" -C "$tmp"
# Prefer matching basename; else take the single top entry.
SRC="$tmp/$BASE"
if [[ ! -d "$SRC" ]]; then
  entries=("$tmp"/*)
  if [[ ${#entries[@]} -eq 1 && -d "${entries[0]}" ]]; then
    SRC="${entries[0]}"
  else
    echo "error: could not find data directory inside archive" >&2
    exit 1
  fi
fi
rsync -a --delete "$SRC"/ "$PARENT/$BASE"/
echo "restored $ARCHIVE → $PARENT/$BASE"
echo "restart relay-edge to pick up stores"
