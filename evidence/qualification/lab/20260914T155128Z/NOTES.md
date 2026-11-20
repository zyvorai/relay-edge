# relay-edge lab drill 20260914T155128Z

- Redeployed zombie process (deleted binary + missing data dir) via `scripts/deploy-remote.sh`.
- Set `EDGE_API_TOKEN` + `EDGE_REQUIRE_AUTH=1` (token not stored in git; sha256 fingerprint only).
- `auth_required: true`, TLS on, backup archive written.
