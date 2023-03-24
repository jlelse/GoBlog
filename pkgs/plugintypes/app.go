package plugintypes

import (
	"context"
	"database/sql"
	"io"
	"net/http"
)

// App is used to access GoBlog's app instance.
type App interface {
	// Get access to GoBlog's database
	GetDatabase() Database
	// Get a post from the database or an error when there is no post for the given path
	GetPost(path string) (Post, error)
	// Get a blog and a bool whether it exists
	GetBlog(name string) (Blog, bool)
	// Purge the rendering cache
	PurgeCache()
	// Get the HTTP client used by GoBlog
	GetHTTPClient() *http.Client
	// Compile an asset (like CSS, JS, etc.) and add it to use when rendering, for some filetypes, it also get's compressed
	CompileAsset(filename string, reader io.Reader) error
	// Get the asset path with the filename used when compiling the assert
	AssetPath(filename string) string
	// Set parameter values for a post path
	SetPostParameter(path string, parameter string, values []string) error
	// Render markdown as text (without HTML)
	RenderMarkdownAsText(markdown string) (text string, err error)
}

// Database is used to provide access to GoBlog's database.
type Database interface {
	Exec(string, ...any) (sql.Result, error)
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	Query(string, ...any) (*sql.Rows, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) (*sql.Row, error)
	QueryRowContext(context.Context, string, ...any) (*sql.Row, error)
}

// Post contains methods to access the post's data.
type Post interface {
	// Get the post path
	GetPath() string
	// Get the blog name
	GetBlog() string
	// Get a string array map with all the post's parameters
	GetParameters() map[string][]string
	// Get a single parameter array (a parameter can have multiple values)
	GetParameter(parameter string) []string
	// Get the first value of a post parameter
	GetFirstParameterValue(parameter string) string
	// Get the post section name
	GetSection() string
	// Get the published date string
	GetPublished() string
	// Get the updated date string
	GetUpdated() string
	// Get the post content (markdown)
	GetContent() string
	// Get the post title
	GetTitle() string
}

// Blog contains methods to access the blog's configuration.
type Blog interface {
	// Get the language
	GetLanguage() string
}

// RenderContext gives some context of the current rendering.
type RenderContext interface {
	// Get the path of the request
	GetPath() string
	// Get the URL
	GetURL() string
	// Get the blog name
	GetBlog() string
}
