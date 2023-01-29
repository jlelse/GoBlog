package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
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
	logBuilder := builderpool.Get()
	logBuilder.WriteString("\nQuery: ")
	logBuilder.WriteString(`"`)
	logBuilder.WriteString(query)
	logBuilder.WriteString(`"`)
	if len(args) > 0 {
		logBuilder.WriteString("\nArgs: ")
		for i, arg := range args {
			if i > 0 {
				logBuilder.WriteString(", ")
			}
			if named, ok := arg.(sql.NamedArg); ok && named.Name != "" {
				logBuilder.WriteString("(")
				logBuilder.WriteString(named.Name)
				logBuilder.WriteString(`) "`)
				logBuilder.WriteString(argToString(named.Value))
				logBuilder.WriteString(`"`)
			} else {
				logBuilder.WriteString(`"`)
				logBuilder.WriteString(argToString(arg))
				logBuilder.WriteString(`"`)
			}
		}
	}
	logBuilder.WriteString("\nDuration: ")
	logBuilder.WriteString(dur.String())
	log.Println(logBuilder.String())
	builderpool.Put(logBuilder)
}

func argToString(arg any) string {
	val := cast.ToString(arg)
	if val == "" {
		val = fmt.Sprintf("%v", arg)
	}
	return val
}
