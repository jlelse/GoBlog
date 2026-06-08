# Testing

GoBlog has an extensive test suite. Tests are written in Go, use the standard library `testing` package, and mostly run against real SQLite databases with all migrations applied.

## Running Tests

```bash
go test -tags=linux,libsqlite3,sqlite_fts5,gomailnotpl -timeout 600s ./...
```

The build tags are required. Without them, compilation fails:

- `linux`: Linux-specific code paths
- `libsqlite3`: System SQLite via cgo
- `sqlite_fts5`: Full-text search support
- `gomailnotpl`: GoMail without templates

Run a single test:

```bash
go test -tags=linux,libsqlite3,sqlite_fts5,gomailnotpl -run TestName ./...
```

## How Tests Work

### Self-Contained Initialization

Each test initializes the app independently using a shared config helper:

```go
app := &goBlog{cfg: createDefaultTestConfig(t)}
_ = app.initConfig(false)
_ = app.initTemplateStrings()
app.d = app.buildRouter()
```

`createDefaultTestConfig(t)` points all file paths (database, caches, profile image) into `t.TempDir()`, so tests are isolated and cleaned up automatically.

### Real SQLite, Not Mocks

Tests run against real SQLite databases on disk, not in-memory databases. On `initConfig(false)`, all embedded migrations from `dbmigrations/*.sql` are applied. This means database code is exercised exactly as it runs in production.

### HTTP Testing

Most HTTP tests route requests directly into the chi router without a TCP layer:

```go
rec := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/path", nil)
app.d.ServeHTTP(rec, req)
```

The helper `newHandlerClient(handler)` in `utils.go` wraps the router into an `*http.Client`, so tests can use the project's fluent HTTP library (`carlmjohnson/requests`) for request construction.

When a test needs to simulate an external service (e.g., a remote server for link checking or webmention fetching), it spins up an `httptest.Server`.

### Test Fixtures

The `testdata/` directory contains HTML pages and GPX track files used as fixture data in parsing and validation tests.

### Libraries

- [`testify`](https://github.com/stretchr/testify): assertions (`assert` and `require`)
- [`carlmjohnson/requests`](https://github.com/carlmjohnson/requests): fluent HTTP test requests

## Integration Tests

Some test files spin up Docker containers to test against real software:

### ActivityPub / GoToSocial

`activityPub_integration_test.go` (gated behind `//go:build !skipIntegration`) launches a real GoToSocial instance in Docker and tests full ActivityPub federation:

- Follow, unfollow
- Post create, update, delete: verified from the federated side
- Likes, reposts (boosts)
- Mentions and replies
- Profile updates
- Moving followers to another account (`Move` activity)
- Domain migration via `altDomain`

The test creates a Docker network, uses `socat` containers for port forwarding, and performs OAuth2 against GoToSocial to obtain Mastodon API tokens. Federation events are verified using `require.Eventually` to wait for asynchronous delivery.

### ACME / Pebble

`acme_integration_test.go` (also gated behind `//go:build !skipIntegration`) tests TLS certificate management against [Pebble](https://github.com/letsencrypt/pebble), Let's Encrypt's ACME test server:

- Certificate issuance via TLS-ALPN-01 challenge
- Certificate renewal
- HTTPS content serving with acquired certificates
- Host whitelist enforcement
- Certificate and account key persistence in the database

### Media Storage (FTP)

`mediaStorage_test.go` spawns a real FTP server (`goftp.io/server/v2`) on a random port and tests the full upload/retrieve/delete cycle against FTP media storage.

These integration tests need a Docker daemon. They are skipped in Dockerfile builds (`skipIntegration` is set in the Dockerfile's `GOFLAGS` to avoid Docker-in-Docker complexity), but run fine in CI and locally. Run them with:

```bash
go test -tags=linux,libsqlite3,sqlite_fts5,gomailnotpl -run Integration -timeout 600s ./...
```

## Fuzz Tests

Currently, there are just very few fuzz tests, but they can be run with:

```bash
go test -tags=linux,libsqlite3,sqlite_fts5,gomailnotpl -fuzz=Fuzz_urlize -fuzztime=30s ./...
```

## Benchmarks

A couple of benchmarks exist, but they are not yet fully fleshed out. Run them with:

```bash
go test -tags=linux,libsqlite3,sqlite_fts5,gomailnotpl -bench=. -benchtime=5s ./...
```

## Test Conventions

- Use `assert` for normal assertions, `require` when the test can't continue on failure
- Tests that can run independently use `t.Parallel()`
- New features should include tests
- Test helpers live alongside the code they test (not in a separate package)

## CI

Continuous integration runs on every push and pull request:

1. **Lint**: `golangci-lint` checks for code quality and security issues
2. **Test**: `go test -timeout 600s -cover ./...` runs all tests including integration tests

## Linting

```bash
golangci-lint run
```

Configuration is in `.golangci.yml`.
