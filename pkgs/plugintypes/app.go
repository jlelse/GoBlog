// Package plugintypes defines the plugin interface types for GoBlog.
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
	// Get posts matching the query
	GetPosts(query *PostsQuery) ([]Post, error)
	// Count posts matching the query
	CountPosts(query *PostsQuery) (int, error)
	// Get a blog and a bool whether it exists
	GetBlog(name string) (Blog, bool)
	// Get all blog names
	GetBlogNames() []string
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
	// Create a new post with just the content (parameters are extracted from the content)
	CreatePost(content string) (Post, error)
	// Upload a file to the media storage and return the URL
	UploadMedia(file io.Reader, filename string, mimetype string) (string, error)
	// Render markdown as text (without HTML)
	RenderMarkdownAsText(markdown string) (text string, err error)
	// Check if user is logged in
	IsLoggedIn(req *http.Request) bool
	// Check if the given password matches a stored app password
	CheckAppPassword(password string) bool
	// Get the full address of a path (including the domain)
	GetFullAddress(path string) string
	// Get sections for a blog
	GetSections(blog string) ([]Section, error)
	// Get all taxonomy values for a blog (e.g. tag names for a taxonomy)
	GetTaxonomyValues(blog string, taxonomy string) ([]string, error)
	// Get comments matching the query
	GetComments(query *CommentsQuery) ([]Comment, error)
	// Count comments matching the query
	CountComments(query *CommentsQuery) (int, error)
	// Get webmentions matching the query
	GetWebmentions(query *WebmentionsQuery) ([]Webmention, error)
	// Count webmentions matching the query
	CountWebmentions(query *WebmentionsQuery) (int, error)
	// Get blog statistics
	GetBlogStats(blog string) (*BlogStats, error)
}

// PostsQuery defines the parameters for querying posts.
type PostsQuery struct {
	// Full-text search query
	Search string
	// Filter by blog name
	Blog string
	// Filter by section name
	Section string
	// Filter by post status: "published", "draft", "scheduled"
	Status string
	// Filter by post visibility: "public", "unlisted", "private"
	Visibility string
	// Filter for posts that have this parameter with a non-empty value
	Parameter string
	// Filter for posts where the parameter has exactly this value
	ParameterValue string
	// Maximum number of posts to return
	Limit int
	// Number of posts to skip for pagination
	Offset int
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
	// Get the post status (e.g. "published", "draft", "scheduled")
	GetStatus() string
	// Get the post visibility (e.g. "public", "unlisted", "private")
	GetVisibility() string
}

// Blog contains methods to access the blog's configuration.
type Blog interface {
	// Get the language
	GetLanguage() string
	// Get the blog title
	GetTitle() string
	// Get the blog description
	GetDescription() string
}

// Section contains information about a blog section.
type Section struct {
	Name        string
	Title       string
	Description string
}

// CommentsQuery defines the parameters for querying comments.
type CommentsQuery struct {
	// Filter by target post path
	Target string
	// Maximum number of comments to return
	Limit int
	// Number of comments to skip for pagination
	Offset int
}

// Comment contains data about a single comment.
type Comment struct {
	ID      int
	Target  string
	Name    string
	Website string
	Comment string
}

// WebmentionsQuery defines the parameters for querying webmentions.
type WebmentionsQuery struct {
	// Filter by target URL or path
	Target string
	// Filter by webmention status: "new", "verified", "approved"
	Status string
	// Maximum number of webmentions to return
	Limit int
	// Number of webmentions to skip for pagination
	Offset int
}

// Webmention contains data about a single webmention.
type Webmention struct {
	Source  string
	Target  string
	Url     string //revive:disable-line:var-naming
	Created string
	Title   string
	Content string
	Author  string
	Status  string
}

// BlogStatsRow contains statistics for a specific period
type BlogStatsRow struct {
	Name, Posts, Chars, Words, WordsPerPost string
}

// BlogStats contains the complete blog statistics
type BlogStats struct {
	Total  BlogStatsRow
	NoDate BlogStatsRow
	Years  []BlogStatsRow
	Months map[string][]BlogStatsRow
}

// RenderContext gives some context of the current rendering.
type RenderContext interface {
	// Get the path of the request
	GetPath() string
	// Get the URL
	GetURL() string
	// Get the blog name
	GetBlog() string
	// Check if user is logged in
	IsLoggedIn() bool
}
