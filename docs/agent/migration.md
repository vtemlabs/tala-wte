# Migration manifest — gitignored build inputs

tala-wte is a **pure-Go cross-compile** (`CGO_ENABLED=0`) — no Docker, no preseed
data blobs. A fresh clone builds with `make build` / `make linux-amd64` /
`make linux-arm64` once the toolchain is present. **There are no must-copy
gitignored files.**

## Regenerated during the build — no action
- `web/build` (`//go:embed all:web/build`) — built by `make build-web` (pnpm + SvelteKit).
- `internal/eterminal/assets/*` (`all:assets`) — vendored fresh by the `eterminal` target.
- `dist/`, `web/.svelte-kit/` — build outputs.

## Tracked in the clone — no action
- `internal/sim/pixie/hostapd-{amd64,arm64}` (pixie-dust embed) — committed.
- `internal/portal/templates/*.html` — committed.

## Host toolchain (install on the new instance)
- Go 1.26, Node 24 + pnpm.

## Notes
- `kernel/` exists on the source machine (gitignored) but is NOT required to build
  and is absent from clones.
