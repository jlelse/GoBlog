package plugintypes

import (
	"context"
	"database/sql"
)

// App is used to access GoBlog's app instance.
type App interface {
	// Get access to GoBlog's database
	GetDatabase() Database
	// Get a post from the database or an error when there is no post for the given path
	GetPost(path string) (Post, error)
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

// Post
type Post interface {
	// Get the post path
	GetPath() string
	// Get a string array map with all the post's parameters
	GetParameters() map[string][]string
	// Get the post section name
	GetSection() string
	// Get the published date string
	GetPublished() string
	// Get the updated date string
	GetUpdated() string
}

// RenderContext
type RenderContext interface {
	// Get the path of the request
	GetPath() string
	// Get the blog name
	GetBlog() string
}
