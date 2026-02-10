# AI Coding Agent Instructions for GoBlog

## Project Overview

GoBlog is a single-user, multi-blog platform written in Go. Single binary, SQLite database, minimal dependencies. Supports IndieWeb protocols (IndieAuth, Micropub, Webmention, Microformats2) and ActivityPub for Fediverse integration.

- **Module**: `go.goblog.app/app`
- **Go version**: 1.25+
- **Primary target OS**: Linux
- **Router**: `chi` (`github.com/go-chi/chi/v5`)
- **Database**: SQLite with FTS5 (full-text search)
- **UI**: Server-side HTML rendering via `pkgs/htmlbuilder` (no templates engine, no frontend framework)
- **Styles**: SCSS compiled to CSS (no inline styles — enforced by CSP headers)

## Commands

### Build

```bash
# With system SQLite (recommended, faster build)
go build -tags=linux,libsqlite3,sqlite_fts5 -o GoBlog

# With embedded SQLite (no system dependency, slower build)
go build -tags=linux,sqlite_fts5 -o GoBlog
```

### Test

```bash
# All tests
go test -tags=linux,libsqlite3,sqlite_fts5 -timeout 600s ./...

# Single package/file
go test -tags=linux,libsqlite3,sqlite_fts5 -run TestName ./...
```

The `-tags=linux,libsqlite3,sqlite_fts5` flags are **required** for all build and test commands. Without them, compilation will fail.

### Lint

```bash
golangci-lint run
```

Linter config is in `.golangci.yml`. Key enabled linters: `errcheck`, `gosimple`, `govet`, `staticcheck`, `gosec`, `bodyclose`, `sqlclosecheck`.

### Styles (SCSS → CSS)

```bash
# Rebuild CSS after editing SCSS
./original-assets/build/buildStyles.sh
```

Requires Node.js/npx. Source: `original-assets/styles/styles.scss` → Output: `templates/assets/css/styles.css`.

### Bundle Frontend Assets

```bash
# Download pinned versions of Leaflet, MarkerCluster, hls.js
./bundle-assets.sh
```

Updates files in `leaflet/`, `hlsjs/`, and version references in `templates/assets/js/`.

### Run

```bash
./GoBlog --config ./config/config.yml
```

## Project Structure

All Go source files live in the **root directory** (flat layout, no `cmd/` or `internal/`):

```
├── app.go                    # Main app struct and initialization
├── main.go                   # Entry point
├── config.go                 # Configuration types and loading
├── database.go               # SQLite database setup
├── databaseMigrations.go     # Schema migrations (uses embedded SQL files)
├── http.go                   # HTTP server setup
├── httpRouters.go            # All route definitions (chi router)
├── httpMiddlewares.go        # HTTP middleware stack
├── ui.go                     # Page rendering functions
├── uiComponents.go           # Reusable UI components
├── posts.go / postsDb.go     # Post types and database operations
├── editor.go                 # Web editor
├── activityPub.go            # ActivityPub protocol
├── micropub.go               # Micropub protocol
├── indieAuth.go              # IndieAuth protocol
├── webmention.go             # Webmention send/receive
├── comments.go               # Comment system
├── feeds.go                  # RSS/Atom/JSON feeds
├── search.go                 # Full-text search (FTS5)
├── cache.go                  # HTTP response caching
├── media.go                  # Media uploads and serving
├── plugins.go                # Plugin system (Yaegi-based)
├── *_test.go                 # Tests (same directory)
│
├── dbmigrations/             # Embedded SQL migration files (00001.sql - 00038.sql)
├── pkgs/                     # Internal reusable packages
│   ├── htmlbuilder/          #   HTML generation (used for all UI)
│   ├── activitypub/          #   ActivityPub utilities
│   ├── plugintypes/          #   Plugin interface definitions
│   ├── plugins/              #   Plugin loading (Yaegi)
│   ├── httpcompress/         #   HTTP compression middleware
│   ├── bodylimit/            #   Request body size limiting
│   ├── cache/                #   Caching abstractions
│   ├── minify/               #   HTML/CSS/JS minification
│   ├── maprouter/            #   URL routing utilities
│   └── ...                   #   Other utilities
│
├── strings/                  # i18n translation files (YAML)
│   ├── default.yaml          #   English (default)
│   ├── de.yaml               #   German
│   ├── es.yaml               #   Spanish
│   └── pt-br.yaml            #   Portuguese (Brazil)
│
├── templates/assets/         # Frontend assets served to browser
│   ├── css/styles.css        #   Compiled CSS (do not edit directly)
│   └── js/*.js               #   JavaScript files
│
├── original-assets/          # Source assets
│   ├── styles/styles.scss    #   SCSS source (edit this)
│   └── build/buildStyles.sh  #   SCSS build script
│
├── plugins/                  # Example/bundled plugins
├── leaflet/                  # Leaflet map library (vendored)
├── hlsjs/                    # HLS.js video library (vendored)
├── static/                   # Static files served at /s/
├── logo/                     # Logo assets
├── config/                   # Runtime configuration (YAML)
├── data/                     # Runtime data directory (DB, media, logs)
└── testdata/                 # Test fixtures
```

## Architecture Patterns

### How Features Are Structured

Most features follow this pattern:
1. **Config**: Feature toggle and options defined in `config.go`
2. **Routes**: Endpoints registered in `httpRouters.go`
3. **Handler**: HTTP handler in its own file (e.g., `comments.go`)
4. **UI**: Rendering in `ui.go` or `uiComponents.go`
5. **Database**: Queries in a `*Db.go` file or inline

### Key Patterns

- **Opt-in by default**: Most features are disabled unless explicitly enabled in config
- **Receiver pattern**: Nearly all methods use `(a *goBlog)` receiver on the main app struct
- **HTML builder**: UI is built programmatically using `pkgs/htmlbuilder` — there are no HTML template files
- **Database migrations**: SQL files in `dbmigrations/` are embedded and run automatically on startup. New migrations must be numbered sequentially (e.g., `00039.sql`)
- **Plugins**: Loaded at runtime via Yaegi (Go interpreter). Plugin interfaces defined in `pkgs/plugintypes/`
- **i18n**: Translation strings in `strings/*.yaml`, accessed via template string functions

## Common Development Tasks

### Adding a new feature
1. Add config fields to `config.go`
2. Register routes in `httpRouters.go`
3. Implement handler in a new or existing file
4. Add UI rendering in `ui.go` / `uiComponents.go`
5. If database changes needed, add a new migration file in `dbmigrations/`

### Adding a new endpoint
Add route definition to `httpRouters.go`. The router uses `chi`.

### Modifying the UI
Edit `ui.go` (page-level rendering) or `uiComponents.go` (reusable components). All HTML is generated via `pkgs/htmlbuilder` — never use raw HTML strings or template files.

### Changing styles
Edit `original-assets/styles/styles.scss`, then run `./original-assets/build/buildStyles.sh`. Never edit `templates/assets/css/styles.css` directly. Inline styles in HTML are forbidden (CSP enforced).

### Adding a database migration
Create a new numbered SQL file in `dbmigrations/` (next number after `00038.sql`). Migrations are embedded via `//go:embed` and run automatically on startup.

### Adding translations
Add keys to `strings/default.yaml` (English), then add translations to other locale files (`de.yaml`, `es.yaml`, `pt-br.yaml`).

### Testing
- New code should be covered by tests, preferably integration tests
- Run `go test -tags=linux,libsqlite3,sqlite_fts5 -timeout 600s ./...` to verify

### Documentation and configuration
- New features must be documented in `README.md`, or existing documentation must be updated to reflect changes
- New configurable features must be added to `example-config.yml` with descriptive comments
- Translation strings in `strings/*.yaml` must be kept sorted alphabetically by key

## Code Style

- Standard Go formatting (`gofmt`)
- No code comments unless the code is genuinely complex
- Use existing utility functions from `utils.go` and `pkgs/utils/`
- Use `pkgs/htmlbuilder` for all HTML generation
- Use `carlmjohnson/requests` for outgoing HTTP requests (already a dependency)
- Error handling: return errors up the call chain; use `a.serveError(w, r, ...)` for HTTP error responses
- Context: pass `context.Context` through where available
