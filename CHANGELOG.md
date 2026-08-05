
## 0.6.2 - 2026-08-05


## 0.6.2 - 2026-08-05

### Fixed
- Point atlas-db-password ExternalSecrets at new Akeyless path/key



## 0.6.1 - 2026-08-05

### Fixed
- Correct dev REDIS_HOST to match Valkey chart's Service name



## 0.6.0 - 2026-08-05

### Added
- Connect via catalog-api's own Atlas database user


### Fixed
- Database name
- Correct catalog-api's actual database name to 'catalog'



## 0.5.1 - 2026-08-05

### Fixed
- Ingress path and middlewares



## 0.5.0 - 2026-08-02


## 0.5.0 - 2026-08-02

### Added
- Add overlays/local for the shared Tailscale front door (#141)


### Documentation
- Add coverage badge to README (#138)
- Fix coverage badge URL to point at GitHub Pages (#140)


### Fixed
- Remove HPA and PDB from dev overlay
- Point local overlay at the shared local MongoDB



## 0.4.0 - 2026-07-26

### Added
- Wire up continuous profiling via Pyroscope



## 0.3.1 - 2026-07-26

### Added
- Add CORS middleware, configurable via ALLOWED_ORIGINS (#122)


### Documentation
- Link deployed Swagger UI and fix coverage report URL (#119)


### Fixed
- Bump api-core.go to v0.0.438 (#113)
- Ingress class name
- Recognize prefix-strip middleware as release-worthy
- Use ClusterIP instead of LoadBalancer for api Service (#120)
- Fix Redis connection - wrong host, port env var collision (#121)
- Update secret path for cache auth
- Tracer provider shuts down before the server ever serves a request
- Point OTLP tracing endpoint at Tempo (missed on develop)



## 0.3.0 - 2026-07-25

### Added
- Add CORS middleware, configurable via ALLOWED_ORIGINS (#122)


### Documentation
- Link deployed Swagger UI and fix coverage report URL (#119)


### Fixed
- Bump api-core.go to v0.0.438 (#113)
- Ingress class name
- Recognize prefix-strip middleware as release-worthy
- Use ClusterIP instead of LoadBalancer for api Service (#120)
- Fix Redis connection - wrong host, port env var collision (#121)



## 0.2.3 - 2026-07-24

### Fixed
- Bump api-core.go to v0.0.438 (#113)
- Ingress class name
- Recognize prefix-strip middleware as release-worthy



## 0.2.2 - 2026-07-24

### Fixed
- Bump api-core.go to v0.0.438 (#113)
- Ingress class name



## 0.2.2 - 2026-07-24

### Fixed
- Bump api-core.go to v0.0.438 (#113)
- Ingress class name



## 0.2.1 - 2026-07-24

### Fixed
- Bump api-core.go to v0.0.437 (#109)



## 0.2.0 - 2026-07-24

### Added
- Migrate from Flux to ArgoCD for deployment (#98)


### Documentation
- Update README


### Fixed
- Repair handler wiring bugs that prevented this service from ever compiling (#93)
- Bump ExternalSecret apiVersion to v1 (#104)


## 0.1.0 - 2024-10-26

# Changelog

All notable changes to this project will be documented in this file.

## 0.1.36 - 2026-07-23

### Documentation
- Update README

### Fixed
- Repair handler wiring bugs that prevented this service from ever compiling (#93)

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
- Bumped dependencies to real tagged releases: common.go v0.0.16, mongodb.go v0.0.193,
  api-core.go v0.0.436, model-core.go v0.0.173, catalog-objects.go v0.0.196, catalog-data.go
  v0.0.21. This resolves the previously-known blocker (this service could not build against
  published versions of its dependency chain) - the full chain has now been released and this
  repo builds and tests green against real tags, not local `replace` directives.
