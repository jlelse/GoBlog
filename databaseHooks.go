package main

import (
	"context"
	"log"
	"time"
)

type dbHooks struct{}

const dbHooksBegin contextKey = "begin"

func (h *dbHooks) Before(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
	return context.WithValue(ctx, dbHooksBegin, time.Now()), nil
}

func (h *dbHooks) After(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
	begin := ctx.Value(dbHooksBegin).(time.Time)
	log.Printf("SQL: %s %q (%s)\n", query, args, time.Since(begin))
	return ctx, nil
}
