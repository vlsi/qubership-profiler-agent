# Profiler UI (Stage 5)

Single-page app over the query service's external API (`/api/v1`), served by the query binary at `/ui`
via `go:embed` (07 §6). React 19 + TypeScript (strict) + Ant Design 6, built with Vite.

Design docs are the source of truth:

- [`backend/docs/design/07-ui-design.md`](../../docs/design/07-ui-design.md) — architecture and the tree engine
- [`backend/docs/design/09-ui-screens.md`](../../docs/design/09-ui-screens.md) — per-screen spec, states, URL scheme
- [`backend/docs/design/02-read-contract.md`](../../docs/design/02-read-contract.md) — endpoint shapes

## Commands

```bash
npm install
npm run dev        # dev server at http://localhost:5173/ui/, /api/v1 proxied to :8080
npm run dev:mock   # same, but /api/v1 answered by the MSW mock — no backend needed
npm run test       # vitest, once
npm run typecheck  # tsc, no emit
npm run build      # tsc + vite build into dist/
```

`VITE_QUERY_URL=http://host:8080 npm run dev` points the proxy at a remote query.

## Layout

| Path | Contents |
|------|----------|
| `src/api/` | Wire types mirroring `backend/libs/query/model/wire.go`, PK path codec, typed fetch |
| `src/msgpack/` | Hand-written decoder for the `/tree` merged-v1 envelope (02 §2.5), mirror encoder for tests/mock |
| `src/url/` | URL-as-state: parse/serialise the 09 §6 query scheme |
| `src/mocks/` | MSW handlers + deterministic synthetic dataset; the mock mirrors the backend's RFC 7807 bodies |
| `src/shell/`, `src/pages/` | App shell and the three routes: `/ui/calls`, `/ui/pods`, `/ui/tree/:pk` |

## Decisions

- **Thin typed fetch, not RTK Query.** Data loads only on an explicit Apply (09 §2.2), the keyset cursor
  freezes the query server-side (02 §2.3.1), and `/tree` is immutable binary — a declarative cache layer
  would manage none of that, and the bundle ships inside the query image. The trade-off is written down in
  `src/api/client.ts`.
- **The mock is the contract.** Handlers reproduce the backend's validation order, problem titles, and
  details (`api.go`, `guard.go`, `cursor.go`, `tree.go`). If the real service disagrees with the mock,
  report the backend bug — do not adapt the mock.
- **No committed fixtures.** The mock dataset derives from hashes of (pod, time bucket); tests generate
  synthetic trees (WORKFLOW.md §6).
