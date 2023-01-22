package plugintypes

import (
	"context"
	"database/sql"
)

// App is used to access GoBlog's app instance.
type App interface {
	GetDatabase() Database
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
	GetParameters() map[string][]string
}

// RenderContext
type RenderContext interface {
	GetPath() string
	GetBlog() string
}
