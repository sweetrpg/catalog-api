
## 0.1.36 - 2026-07-23

### Documentation
- Update README


### Fixed
- Repair handler wiring bugs that prevented this service from ever compiling (#93)


# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added

- CONTRIBUTING.md, CODE_OF_CONDUCT.md, AGENTS.md/CLAUDE.md repo scaffolding.

### Fixed

- Every list handler (`listContributions`, `listReviews`, `listLicenses`, `listPersons`,
  `listPublishers`, `listStudios`, `listSystems`) called a nonexistent `data.Get<Plural>`
  function - this repo could never actually compile once its dependency chain was otherwise
  working, since these are compile errors, not runtime bugs. Corrected to `data.Query<Plural>`
  (the functions that actually exist in `catalog-data.go`).
- `getLicense` called `data.GetVolume` instead of `data.GetLicense`; `getPerson` called
  `data.GetLicense` instead of `data.GetPerson`. Both cross-entity mix-ups, presumably from
  copy-pasting `volumes.go` as a template without updating every call site.
- `getLicenseVolumes` built its filter with `apiutil.Filter{Operation: bson.D{...}}`, a type
  mismatch (`Operation` is `*string`) that would not compile either. Corrected to set
  `Operation`/`Value` per the actual `Filter` struct shape.
- Every single-item getter (`getContribution`, `getReview`, `getLicense`, `getPerson`,
  `getPublisher`, `getStudio`, `getSystem`, `getVolume`) was missing a `return` after writing
  the 404 response, so a not-found lookup would fall through and also write a (likely broken)
  200-with-nil-body response after the 404 headers were already sent.
- `main.go` imported `github.com/sweetrpg/db.go/database` - that GitHub repo was renamed to
  `mongodb.go`; updated the import path and `go.mod` require accordingly.
- Drops the PR workflow's `golint` step (unconditionally broken - pulls a transitive dep
  requiring Go >=1.25).

### Known blocker

This repo still cannot build against the currently *published* versions of its dependencies:
`catalog-data.go@v0.0.20` has a bug (calls `modelcorevo.FromPropertyModels`/`FromTagModels`,
which live in `model-core.go`'s `util` package, not `vo`) that's already fixed on
`catalog-data.go`'s `develop` branch but not yet tagged/released. Verified the full fix chain
(common.go -> mongodb.go -> api-core.go -> model-core.go -> catalog-objects.go ->
catalog-data.go -> this repo) builds cleanly end-to-end using local `replace` directives; once
those repos' PRs are merged and released, bump this repo's `go.mod` to the new versions and the
build should go green.
