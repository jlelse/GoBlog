# GitHub Copilot Instructions for GoBlog

## Project Overview

GoBlog is a single-user, multi-blog platform written in Go with IndieWeb and ActivityPub support. See README.md for detailed feature information.

## Package Structure

Single Go module (`go.goblog.app/app`) with all source code in root directory:

- **Core**: `app.go`, `main.go`, `config.go`, `database.go`
- **Web**: `http.go`, `httpRouters.go`, `httpMiddlewares.go`
- **Posts**: `posts.go`, `postsDb.go`, `editor.go`
- **Protocols**: `activityPub.go`, `micropub.go`, `indieAuth.go`, `webmention.go`
- **Features**: `comments.go`, `reactions.go`, `search.go`, `feeds.go`
- **Utils**: `utils.go`, `cache.go`, `log.go`

Test files: `*_test.go` pattern.

Internal packages in `/pkgs/` provide reusable utilities:
- **activitypub**: ActivityPub protocol utilities
- **bodylimit**: HTTP body size limiting
- **bufferpool**: Buffer pooling for performance
- **cache**: Caching abstractions
- **contenttype**: Content type handling
- **gpxhelper**: GPX file processing
- **highlighting**: Syntax highlighting
- **htmlbuilder**: HTML generation utilities
- **httpcachetransport**: HTTP caching transport
- **httpcompress**: HTTP compression middleware
- **maprouter**: URL routing
- **minify**: Content minification
- **plugins**: Plugin system utilities
- **plugintypes**: Plugin type definitions
- **tor**: Tor integration
- **utils**: General utilities
- **yaegiwrappers**: Yaegi scripting wrappers

## Development Setup

### Prerequisites
- Go 1.25+
- SQLite3 dev libs (for FTS5)

### Build Commands
```bash
# With system SQLite (recommended)
go build -tags=linux,libsqlite3,sqlite_fts5 -o GoBlog

# With embedded SQLite
go build -tags=linux,sqlite_fts5 -o GoBlog
```

### Testing
```bash
go test -tags=linux,libsqlite3,sqlite_fts5
```

### Running
```bash
./GoBlog --config ./config/config.yml
```

### Styles (SCSS/CSS)

- **Source:** Styles are authored as SCSS in `./original-assets/styles/styles.scss` and are compiled to CSS (`./templates/assets/css/styles.css`) for the site.
- **Rebuild command:** Run the build script to regenerate CSS after editing SCSS:

```bash
./original-assets/build/buildStyles.sh
```

- **Notes:** The build script lives at `./original-assets/build/buildStyles.sh` and requires Node.js and npm/npx. Inspect the script if build failures occur.

Inline styles in the UI are forbidden (also enforced by security headers); all styles must be in SCSS/CSS files.

## Key Development Concepts

- **Opt-in features**: Most features disabled by default
- **Database changes**: Add migrations to `databaseMigrations.go`
- **New endpoints**: Add to `httpRouters.go`
- **UI changes**: Modify `ui.go` and `uiComponents.go`
- **Plugins**: Runtime-loaded Go plugins for extensibility

## Code Patterns

- Features are opt-in by default
- Markdown-first content approach
- SQLite with FTS5 for search
- No themes - customize via config/plugins