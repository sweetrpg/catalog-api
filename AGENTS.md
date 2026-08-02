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
  injection - neither has been done yet. `cmd/catalog-api`, `cachettl`, and `ratelimit` do have
  unit test coverage (the latter two use `alicebob/miniredis` rather than a real Redis).
- This repo currently cannot build against the *published* versions of `catalog-data.go`
  (a bug there is fixed on `develop` but unreleased) - see CHANGELOG.md.
- The `DISTRIBUTED_RATE_LIMIT_ENABLED` per-client rate limiter hasn't been validated against a
  real dev workload yet - the legacy process-wide limiter is still the default. See
  `openspec/changes/catalog-api-caching-rate-limiting` (in the `platform` umbrella repo) for the
  rest of that rollout.

## Dependencies

Depends on `api-core.go`, `catalog-data.go`, `common.go`, and `mongodb.go`. Nothing depends on
this repo - it's the top of the catalog dependency chain.

## Caching and Rate Limiting

See `platform/docs/service-conventions.md`'s Caching and Rate limiting sections for the
Redis-backed cache readiness check, per-route TTL (`CACHE_TTLS`/`CACHE_DEFAULT_TTL`), and the
distributed rate limiter (`DISTRIBUTED_RATE_LIMIT_ENABLED`,
`RATE_LIMIT_CHEAP`/`RATE_LIMIT_CHEAP_WINDOW_SECONDS`,
`RATE_LIMIT_STANDARD`/`RATE_LIMIT_STANDARD_WINDOW_SECONDS`) that replaces it once validated.

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
