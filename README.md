# App Store reviews

![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)
![TypeScript](https://img.shields.io/badge/TypeScript-5.x-3178C6?logo=typescript&logoColor=white)
![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black)
![Bun](https://img.shields.io/badge/runtime-Bun-f7f8f5?logo=bun&logoColor=black)
![License](https://img.shields.io/badge/license-MIT-green)

A Go service that polls Apple's iTunes customer-reviews feed for any number of iOS apps,
stores what it finds, and serves recent reviews over HTTP — plus a React app that displays
them: the last 48 hours by default, newest first, with content, author, score, and submission
time.

The emphasis is a small, well-seamed service on Go's standard library — easy to read, easy to
test, and easy to repoint at a different feed or datastore.

<!-- Add a screenshot of the UI here, e.g.  ![reviews-api](docs/screenshot.png) -->

## Why These Choices

The brief asks about the design as much as the behaviour, so the reasoning in brief:

- **Standard library only on the backend.** No router, HTTP framework, or feed parser. Go
  1.22's `ServeMux` does method + wildcard routing (`GET /apps/{appID}/reviews`),
  `encoding/json` handles both the feed and the API, and a small custom `UnmarshalJSON`
  absorbs the one awkward shape in the iTunes payload. The frontend takes the usual typed-SPA
  tools — React, Vite, Tailwind, Zod.
- **Three layers, dependencies pointing inward.** `domain` is pure types; `service` holds the
  logic and *declares* the interfaces it needs (`ReviewStore`, `ReviewFetcher`, `AppRegistry`);
  the edges implement them; `cmd/server` wires the concrete edges together. Swapping the feed
  is one new `ReviewFetcher`; moving to Postgres is one new `ReviewStore` — the service and its
  tests don't change. The service suite runs entirely on in-memory fakes.
- **Crash-safe persistence.** Reviews are stored one file per app under `DATA_DIR`; each write
  is atomic (temp → fsync → rename → directory fsync) and deduped by review id, so a crash
  mid-write can't corrupt a file and re-polling never double-stores. The registry lives in
  `apps.json`. Everything loads on boot, so the process stops and restarts without losing data.
- **The window is a read-time decision.** The fetcher stores roughly a week of reviews per app;
  each request applies its own window (`?hours=`, default 48), and the summary endpoint uses the
  same window as the list, so the stats always match what's on screen.
- **Name + icon come from the right place.** The `/json` reviews feed carries only reviews — no
  app name or icon (verified against the live feed; the first entry is a real review, not
  metadata). App identity comes from the iTunes **Lookup API**, called once when an app is
  added — best-effort and concurrent with validation, so a slow lookup never blocks the add and
  the app falls back to its id. Icon URLs are checked to be `https` on Apple's `*.mzstatic.com`
  CDN before they're stored.
- **OpenAPI is the contract; Zod mirrors it.** `api/openapi.yaml` is the hand-maintained source
  of truth, embedded in the binary and served at `/openapi.yaml` with Swagger UI at `/docs`. A
  backend test fails if a route lacks a matching spec path, so routes and spec stay in
  lock-step; the frontend's Zod schemas enforce response shapes at runtime, so an unexpected
  change surfaces as a parse error rather than a silent `undefined`.
- **Any number of apps.** Apps are added and removed at runtime through the API, not baked into
  config; the scheduler fans out across all tracked apps each tick with bounded concurrency
  (`POLL_CONCURRENCY`) and honours cancellation for a clean shutdown. Adding the hundredth app
  is a `POST /apps`, and one slow app can't starve the others.

## Requirements

- Go 1.22 or newer
- Bun 1.x (for the frontend)
- No external services — reviews and the registry are just files on disk

## Run It

### Backend

```sh
go run ./cmd/server          # listens on :8080
```

Seed apps to track at boot (optional):

```sh
APP_IDS=389801252,447188370 go run ./cmd/server
```

### Frontend

```sh
cd web
bun install
bun run dev                  # Vite dev server, talks to the API on :8080
```

Point the frontend at a different API with `VITE_API_BASE_URL` (see `web/.env.example`).

## API

| Method | Path                       | Purpose                                   |
| ------ | -------------------------- | ----------------------------------------- |
| GET    | `/healthz`                 | Liveness check                            |
| GET    | `/apps`                    | List tracked apps                         |
| POST   | `/apps`                    | Track an app (`{"id":"<numeric>"}`)       |
| DELETE | `/apps/{appID}`            | Stop tracking an app                      |
| GET    | `/apps/{appID}/reviews`    | Reviews in the window (`?hours=`)         |
| GET    | `/apps/{appID}/summary`    | Aggregate stats in the window (`?hours=`) |
| GET    | `/openapi.yaml`            | The OpenAPI 3.1 spec                       |
| GET    | `/docs`                    | Swagger UI                                |

```sh
curl -X POST localhost:8080/apps -d '{"id":"389801252"}'        # track Instagram
curl localhost:8080/apps
curl 'localhost:8080/apps/389801252/reviews?hours=48'
curl 'localhost:8080/apps/389801252/summary?hours=48'
curl -X DELETE localhost:8080/apps/389801252
```

Interactive docs: <http://localhost:8080/docs>.

## Configuration

All backend configuration is environment variables, all optional:

| Variable               | Default | Meaning                                          |
| ---------------------- | ------- | ------------------------------------------------ |
| `PORT`                 | `8080`  | Listen port                                      |
| `DATA_DIR`             | `data`  | Directory for review files and `apps.json`       |
| `APP_IDS`              | —       | Comma-separated numeric app ids to seed at boot  |
| `POLL_INTERVAL`        | `5m`    | How often to re-poll every tracked app           |
| `DEFAULT_WINDOW_HOURS` | `48`    | Window used when a request omits `?hours=`       |
| `POLL_CONCURRENCY`     | `4`     | Max apps polled at once                          |
| `CORS_ORIGIN`          | `*`     | `Access-Control-Allow-Origin` for the API        |

## Project Structure

```
cmd/server/        composition root, scheduler, graceful shutdown
internal/
  domain/          Review, App, Summary — pure types
  service/         application logic + the store / fetcher / registry interfaces
  appstore/        iTunes feed fetcher + Lookup-API enrichment
  storage/         atomic file-backed review store and app registry
  httpapi/         HTTP handlers, CORS, OpenAPI + Swagger UI
api/               embedded openapi.yaml
web/               React + Vite + Tailwind frontend
```

Where to start reading: `internal/service` is the core — the application logic and the
interfaces it depends on. `cmd/server/main.go` shows how the concrete edges are wired in.

## How It Works

1. On boot, the service loads stored reviews and the app registry from `DATA_DIR`, seeding any
   ids from `APP_IDS`.
2. A scheduler ticks every `POLL_INTERVAL`, fanning out across all tracked apps with bounded
   concurrency.
3. For each app, the fetcher pages the iTunes `/json` feed newest-first back about a week and
   returns parsed reviews.
4. The store appends only review ids it hasn't seen, writing each app's file atomically.
5. `POST /apps` validates an id, enriches it via the Lookup API (name + icon), persists it, and
   kicks off an immediate poll so reviews appear without waiting for the next tick.
6. `GET /apps/{id}/reviews?hours=` returns the stored reviews inside the window, newest-first;
   `…/summary` aggregates the same window.
7. The React app lists tracked apps, shows the selected app's banner and reviews, and silently
   refreshes on an interval.

## Tests

```sh
go test ./... -race          # backend: parser, file stores, service (fakes), HTTP, spec/route drift
cd web && bun run test       # frontend: ReviewCard render + useReviews hook (Vitest + Testing Library)
./.claude/gate.sh            # full gate: gofmt/vet/build/test -race + web biome/tsc/vitest/build
```

## Beyond the Brief

- **Runtime-editable registry** — add/remove tracked apps via the API, with a background initial
  poll on add so reviews appear immediately.
- **Summary endpoint** — per-app totals, average score, per-star counts, and last-updated, over
  the same window as the reviews list.
- **App banner** — name + icon from the Lookup API, with an id fallback.
- **UI niceties** — a selectable time window with silent auto-refresh, and client-side pagination.
- **Swagger UI** at `/docs`.

## Extend It

- Generate the frontend Zod schemas from `openapi.yaml` so response shapes are contract-checked
  mechanically, not mirrored by hand.
- Swap the file stores for a real database behind the existing interfaces.
- Conditional polling (ETag / `If-None-Match`) to skip unchanged pages and cut bandwidth.
- Per-app poll cadence instead of one global interval.
- Auth on the write endpoints (`POST` / `DELETE`).
- Structured logging and metrics around poll latency and failure rates.

## License

MIT
