# Contributing

1. Fork / branch from `main`.
2. `make vet test` and keep smokes green when touching HTTP or simulators.
3. Prefer small PRs; update `docs/` when behaviour or env vars change.
4. Releases are tag-gated — maintainers cut `v*` via Actions → **Release**.

## Local checks

```bash
make vet test
go run ./cmd/relay-edge   # then:
make smoke-all            # EDGE=http://127.0.0.1:18086
```
