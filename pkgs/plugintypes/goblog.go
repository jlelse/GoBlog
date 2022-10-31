package plugintypes

import (
	"context"
	"database/sql"

	"go.goblog.app/app/pkgs/htmlbuilder"
)

// App is used to access GoBlog's app instance.
type App interface {
	GetDatabase() Database
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

// Blog
type Blog interface {
	GetBlog() string
}

// RenderType
type RenderType string

// RenderData
type RenderData interface {
	// Empty
}

// RenderNextFunc
type RenderNextFunc func(*htmlbuilder.HtmlBuilder)

// Render main element content on post page, data = PostRenderData
const PostMainElementRenderType RenderType = "post-main-content"

// PostRenderData is RenderData containing a Post
type PostRenderData interface {
	RenderData
	GetPost() Post
}

// Render footer element on every blog page, data = BlogRenderData
const BlogFooterRenderType RenderType = "blog-footer"

// BlogRenderData is RenderData containing a Blog
type BlogRenderData interface {
	RenderData
	GetBlog() Blog
}
