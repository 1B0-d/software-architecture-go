# Software Architecture: Go Services Demonstration

A small educational vehicle catalog and review system illustrating software architecture. It was created as a new demonstration in November 2026; it is not the original Building Microservices API in Go course project.

## Architecture and technology

Two Go HTTP services use separate SQLite databases. The Reviews service calls Catalog over HTTP before accepting a review. Repository interfaces, handlers, middleware and a small HTML view illustrate dependency inversion, persistence and basic MVC concepts; see [ARCHITECTURE.md](ARCHITECTURE.md).

## Run

Install Go 1.25.5 or a compatible version and download the module dependencies. In separate terminals from this directory:

```sh
go run ./cmd/catalog
go run ./cmd/reviews
```

Catalog listens on `127.0.0.1:8081` and Reviews on `127.0.0.1:8082`. Configuration: `CATALOG_ADDR`, `CATALOG_DB`, `REVIEWS_ADDR`, `REVIEWS_DB` and `CATALOG_URL`. Databases are local files and persist across restarts.

## Verification and limitations

Run `go test ./... -v`; `python standalone_check.py` builds and exercises two separate processes. Saved results from 10 November 2026 are in `test-results.txt` and `standalone-results.txt`. Tests cover HTTP communication, persistence, invalid input, an unavailable catalog, and HTML escaping. This demonstration has no authentication, migrations, production deployment, or distributed transactions.
