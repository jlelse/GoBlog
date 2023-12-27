package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/spf13/cast"
	"go.goblog.app/app/pkgs/builderpool"
)

const dbHooksBegin contextKey = "begin"

func (db *database) dbBefore(ctx context.Context, _ string, _ ...any) context.Context {
	if !db.debug {
		return ctx
	}
	return context.WithValue(ctx, dbHooksBegin, time.Now())
}

func (db *database) dbAfter(ctx context.Context, query string, args ...any) {
	if !db.debug {
		return
	}
	dur := time.Since(ctx.Value(dbHooksBegin).(time.Time))
	argsBuilder := builderpool.Get()
	if len(args) > 0 {
		for i, arg := range args {
			if i > 0 {
				argsBuilder.WriteString(", ")
			}
			if named, ok := arg.(sql.NamedArg); ok && named.Name != "" {
				argsBuilder.WriteString("(")
				argsBuilder.WriteString(named.Name)
				argsBuilder.WriteString(`) '`)
				argsBuilder.WriteString(argToString(named.Value))
				argsBuilder.WriteString(`'`)
			} else {
				argsBuilder.WriteString(`'`)
				argsBuilder.WriteString(argToString(arg))
				argsBuilder.WriteString(`'`)
			}
		}
	}
	db.a.debug("Database query", "query", query, "args", argsBuilder.String(), "duration", dur.String())
	builderpool.Put(argsBuilder)
}

func argToString(arg any) string {
	val := cast.ToString(arg)
	if val == "" {
		val = fmt.Sprintf("%v", arg)
	}
	return val
}
