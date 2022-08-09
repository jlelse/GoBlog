package plugintypes

import (
	"context"
	"database/sql"
	"net/http"
)

// Interface to GoBlog

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

// Plugin types

// SetApp is used in all plugin types to allow
// GoBlog set it's app instance to be accessible by the plugin.
type SetApp interface {
	SetApp(App)
}

// SetConfig is used in all plugin types to allow
// GoBlog set plugin configuration.
type SetConfig interface {
	SetConfig(map[string]any)
}

type Exec interface {
	SetApp
	SetConfig
	Exec()
}

type Middleware interface {
	SetApp
	SetConfig
	Handler(http.Handler) http.Handler
	Prio() int
}
