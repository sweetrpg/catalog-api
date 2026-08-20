
## 0.19.0 - 2026-08-20

### Added
- Add admin-only DELETE/restore routes for catalog entities
- Invalidate GET response cache on delete/restore


### Fixed
- Add game system association, cache invalidation, studio volume patching
- Invalidate response cache on entity create/patch/accept



## 0.18.0 - 2026-08-19

### Added
- Extend /stats to per-entity-type count/most-recent cards



## 0.17.1 - 2026-08-19

### Fixed
- Fix Deployment fields that never matched ArgoCD's applied manifest



## 0.17.0 - 2026-08-19

### Added
- Add create route, license tags field, license-volumes association



## 0.16.0 - 2026-08-19

### Added
- Add GET /stats endpoint for catalog-wide summary



## 0.15.3 - 2026-08-19

### Fixed
- Scope write-invalidation flush to the response cache DB only



## 0.15.2 - 2026-08-18


## 0.15.2 - 2026-08-18

### Fixed
- Invalidate response cache after a successful write



## 0.15.1 - 2026-08-18

### Fixed
- Bump catalog-data.go to v0.8.2 (QueryVolumes now defaults to title sort, so editing a
  volume can't push it out of the default-limit browse listing)


## 0.15.0 - 2026-08-18

### Added
- Add GET /staged-assets/pending for the reclaim job cross-check


### Documentation
- Note why website field accessors dropped url.Parse


### Fixed
- Drop url.Parse round-trip for publisher/studio/license website



## 0.14.2 - 2026-08-18

### Fixed
- Set AUTH_API_URL and ASSETS_WEB_URL in dev



## 0.14.1 - 2026-08-18

### Fixed
- Propagate trace context to auth-api and assets-web
- Bump api-core.go to v0.1.1



## 0.14.0 - 2026-08-18

### Added
- Version model for all entity types, remove proposed_changes mechanism



## 0.13.0 - 2026-08-17

### Added
- Add debug logging for authorization checks



## 0.12.1 - 2026-08-14

### Fixed
- Filename and date of license



## 0.12.0 - 2026-08-14


## 0.12.0 - 2026-08-14

### Added
- Rewrite volume PATCH/accept/reject against the version model
- Add migrate-volumes command for the version-model cutover



## 0.11.0 - 2026-08-13

### Added
- Allow editors to set properties, publishers, and studios via PATCH
- Add shared contribution-type/property-name/format lists
- Support format and contributor credits on PATCH
- Sample/cover PATCH support, finalize-session, staged-asset promotion
- Submission cap, retract, and pull-back



## 0.10.0 - 2026-08-12

### Added
- Add associated-volumes endpoints for publishers, studios, persons
- Add PATCH and proposed-change review endpoints for entities



## 0.9.0 - 2026-08-12

### Added
- Add role-gated volume editing with submitter review workflow


### Fixed
- Simplify negated OR to De Morgan's form in acceptVolumeProposedChange
- Correct INGRESS_BASE_PATH to match actual Ingress routes



## 0.8.0 - 2026-08-11

### Added
- Gate continuous profiling behind the profiling-enabled feature flag



## 0.7.1 - 2026-08-10

### Fixed
- Feature flag format


## 0.7.0 - 2026-08-10

### Added
- Add slog-gin for JSON HTTP access logging


### Documentation
- Document slog-gin JSON HTTP logging


### Fixed
- Authenticate AtlasDatabaseUser against admin, not app db



## 0.6.8 - 2026-08-06

### Fixed
- Version-prefixed Ingress paths with a latest-alias Service



## 0.6.7 - 2026-08-06

### Fixed
- Point readinessProbe at /status/ping, not /status/health



## 0.6.6 - 2026-08-06

### Fixed
- Widen readinessProbe timeout margin above Mongo's worst case



## 0.6.5 - 2026-08-06

### Fixed
- Run the cache readiness check concurrently, not after Mongo



## 0.6.4 - 2026-08-06

### Fixed
- Live-ping the cache backend instead of caching a boot-time result



## 0.6.3 - 2026-08-06

### Fixed
- Target DB_PASSWORD explicitly in the catalog-api-db rewrite
- Remove startup log dump of the full process environment



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
