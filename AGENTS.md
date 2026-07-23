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
- No test coverage in `server/` or `cmd/` - the handler bugs found in this repo's initial
  hardening pass (wrong function names, cross-entity mix-ups, missing `return` after 404) were
  caught by code review, not by tests. Testing these handlers meaningfully needs either
  MongoDB-backed integration tests (matching `catalog-data.go`'s pattern) or refactoring for
  dependency injection - neither has been done yet.
- This repo currently cannot build against the *published* versions of `catalog-data.go`
  (a bug there is fixed on `develop` but unreleased) - see CHANGELOG.md.

## Dependencies

Depends on `api-core.go`, `catalog-data.go`, `common.go`, and `mongodb.go`. Nothing depends on
this repo - it's the top of the catalog dependency chain.

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
