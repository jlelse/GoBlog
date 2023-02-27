package main

import (
	"context"
	"errors"

	"golang.org/x/crypto/acme/autocert"
)

// Make sure the httpsCache type implements the Cache interface
var _ autocert.Cache = (*httpsCache)(nil)

type httpsCache struct {
	db *database
}

func (c *httpsCache) check() error {
	if c.db == nil {
		return errors.New("no database")
	}
	return nil
}

func (c *httpsCache) Get(ctx context.Context, key string) ([]byte, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	d, err := c.db.retrievePersistentCacheContext(ctx, "https_"+key)
	if d == nil && err == nil {
		return nil, autocert.ErrCacheMiss
	} else if err != nil {
		return nil, err
	}
	return d, nil
}

func (c *httpsCache) Put(ctx context.Context, key string, data []byte) error {
	if err := c.check(); err != nil {
		return err
	}
	return c.db.cachePersistentlyContext(ctx, "https_"+key, data)
}

func (c *httpsCache) Delete(ctx context.Context, key string) error {
	if err := c.check(); err != nil {
		return err
	}
	return c.db.clearPersistentCacheContext(ctx, "https_"+key)
}
