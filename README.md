# SweetRPG Catalog API

[![CI](https://github.com/sweetrpg/catalog-api/actions/workflows/ci.yaml/badge.svg)](https://github.com/sweetrpg/catalog-api/actions/workflows/ci.yaml)
[![Coverage](https://img.shields.io/endpoint?url=https://sweetrpg.github.io/catalog-api/coverage-badge.json)](https://sweetrpg.github.io/catalog-api/)
[![License](https://img.shields.io/github/license/sweetrpg/catalog-api.svg)](https://img.shields.io/github/license/sweetrpg/catalog-api.svg)
[![Issues](https://img.shields.io/github/issues/sweetrpg/catalog-api.svg)](https://img.shields.io/github/issues/sweetrpg/catalog-api.svg)
[![PRs](https://img.shields.io/github/issues-pr/sweetrpg/catalog-api.svg)](https://img.shields.io/github/issues-pr/sweetrpg/catalog-api.svg)
[![Dependabot](https://badgen.net/github/dependabot/sweetrpg/catalog-api)](https://badgen.net/github/dependabot/sweetrpg/catalog-api)
[![Deployment](https://argocd.dev.pilgrimagesoftware.com/api/badge?name=sweetrpg-catalog-api&revision=true&showAppName=true&namespace=sweetrpg-system)](https://argocd.dev.pilgrimagesoftware.com/applications/sweetrpg-catalog-api)

HTTP microservice for the SweetRPG Catalog domain (licenses, volumes, contributions, persons,
publishers, reviews, studios, systems). A thin Gin-based layer: `server/*.go` wires JSON:API
routes to [catalog-data.go](https://github.com/sweetrpg/catalog-data.go)'s data-access functions.

## Run locally

```bash
scripts/run-docker-local.sh
```

Brings up the service plus its MongoDB and Redis dependencies via `docker/docker-compose.yml`.
Swagger UI is served at `/swagger/index.html` once running.

## Known gaps

- `catalog-data.go`'s Volume entity is the only one with a write path, and `Update`/`Delete`
  are still `// TODO` stubs there. Every other entity is read-only.
- No test coverage in `server/` or `cmd/` yet.

## Documentation

Package documentation: [pkg.go.dev/github.com/sweetrpg/catalog-api](https://pkg.go.dev/github.com/sweetrpg/catalog-api).

Swagger UI (`swaggo/swag` + `gin-swagger`, generated from handler annotations, not
hand-written): [api.catalog.dev.sweetrpg.com/swagger/index.html](https://api.catalog.dev.sweetrpg.com/swagger/index.html)
in dev, or `/swagger/index.html` against whatever host you're running locally.

Test coverage reports are published to
[sweetrpg.github.io/catalog-api/coverage.html](https://sweetrpg.github.io/catalog-api/coverage.html)
on every merge to `develop`.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the development workflow and
[RELEASE.md](RELEASE.md) for how versions get cut.
