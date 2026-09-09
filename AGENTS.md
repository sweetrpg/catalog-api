# AGENTS.md

This file provides guidance to Claude Code, Codex, GitHub Copilot, and other AI coding agents
working in this repository.

## About This Project

`catalog-api` is the HTTP microservice for the SweetRPG Catalog domain (licenses, volumes,
contributions, persons, publishers, reviews, studios, systems). It's a thin Gin-based layer:
`server/*.go` wires JSON:API routes to `catalog-data.go`'s data-access functions.

## Known gaps

- `catalog-data.go`'s Volume entity is the only one with a write path, and `Update`/`Delete`
  are still `// TODO` stubs there. Every other entity is read-only.
- No test coverage in `server/` - the handler bugs found in this repo's initial hardening pass
  (wrong function names, cross-entity mix-ups, missing `return` after 404) were caught by code
  review, not by tests. Testing these handlers meaningfully needs either MongoDB-backed
  integration tests (matching `catalog-data.go`'s pattern) or refactoring for dependency
  injection - neither has been done yet. `cmd/catalog-api` and `cachettl` do have unit test
  coverage (`cachettl` uses `alicebob/miniredis` rather than a real Redis).
- The volumes tag cloud endpoint (`GET /volumes/tags`) depends on `catalog-data.go >= v0.18.0`,
  which introduced `GetVolumeTags`. Older published versions of the module lack that function
  and fail at build time.

## Dependencies

Depends on `api-core.go`, `catalog-data.go`, `common.go`, and `mongodb.go`. Nothing depends on
this repo - it's the top of the catalog dependency chain.

## Logging

HTTP access logs output in JSON format via `slog-gin` middleware, configured in `cmd/catalog-api/main.go`.
Application logs remain under `common.go/logging` control. This provides structured logs suitable for
log aggregation systems while keeping HTTP and application concerns separate.

## Caching and Rate Limiting

See `platform/docs/service-conventions.md`'s Caching and Rate limiting sections for the
Redis-backed cache readiness check and per-route TTL (`CACHE_TTLS`/`CACHE_DEFAULT_TTL`).

Per-client/IP rate limiting is the default, via the shared `api-core.go/ratelimit` middleware:
Redis-backed counters keyed by `X-API-Key` else client IP, `cheap` tier for `/status/*` and
`standard` for everything else, fail-closed 503 when the Redis backend is unreachable, 429 on
exceed. Tune with `RATE_LIMIT_CHEAP`/`RATE_LIMIT_CHEAP_WINDOW_SECONDS`/`RATE_LIMIT_STANDARD`/
`RATE_LIMIT_STANDARD_WINDOW_SECONDS` (dev overlay keeps `standard` at 300/60 for catalog-web's
server-to-server fan-out). The legacy `golang.org/x/time/rate` process-wide limiter and the
`DISTRIBUTED_RATE_LIMIT_ENABLED` toggle were removed - see `platform`'s
`openspec/changes/fix-rate-limiting-per-client-ip`.

## Committing Code

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>
```

## Branches and Workflow

* `develop` - integration branch, default branch, target for all PRs.
* `master` - latest released state, nothing committed directly.
* `feature/*`, `fix/*` branched from `develop`; `hotfix/*` branched from `master`.

See `CONTRIBUTING.md` for the full workflow.

## Running Checks Locally

```bash
go build -v ./...
go vet ./...
go test -v -coverprofile coverage.out ./...
```

## Releases

See `RELEASE.md`. Summary: trigger `prepare-release.yaml` (`workflow_dispatch` against
`develop`), which computes the next version from conventional commits via git-cliff and opens
a `release/<version>` PR into `master`. Merging that PR tags the release
(`tag-release.yaml`), which triggers `release.yaml` - re-runs tests, creates a GitHub
Release, and merges `master` back into `develop`.
