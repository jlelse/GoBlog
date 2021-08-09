package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/spf13/cast"
)

const dbHooksBegin contextKey = "begin"

func (db *database) dbBefore(ctx context.Context, query string, args ...interface{}) context.Context {
	if !db.debug {
		return ctx
	}
	return context.WithValue(ctx, dbHooksBegin, time.Now())
}

func (db *database) dbAfter(ctx context.Context, query string, args ...interface{}) {
	if !db.debug {
		return
	}
	dur := time.Since(ctx.Value(dbHooksBegin).(time.Time))
	var logBuilder strings.Builder
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
}

func argToString(arg interface{}) string {
	val := cast.ToString(arg)
	if val == "" {
		val = fmt.Sprintf("%v", arg)
	}
	return val
}
