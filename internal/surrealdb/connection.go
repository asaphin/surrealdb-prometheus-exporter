package surrealdb

import (
	"context"
	"fmt"
	"time"

	"github.com/surrealdb/surrealdb.go"
)

type Config interface {
	SurrealURL() string
	SurrealNamespace() string
	SurrealDatabase() string
	SurrealUsername() string
	SurrealPassword() string
	SurrealTimeout() time.Duration
}

func NewConnection(ctx context.Context, cfg Config) (*surrealdb.DB, error) {
	ctx, _ = context.WithTimeout(ctx, cfg.SurrealTimeout())

	db, err := surrealdb.FromEndpointURLString(ctx, cfg.SurrealURL())
	if err != nil {
		return nil, fmt.Errorf("unable to connect to SurrealDB: %w", err)
	}

	authData := &surrealdb.Auth{
		Username: cfg.SurrealUsername(),
		Password: cfg.SurrealPassword(),
	}

	token, err := db.SignIn(ctx, authData)
	if err != nil {
		return nil, fmt.Errorf("unable to sign in to SurrealDB: %w", err)
	}

	if err = db.Authenticate(ctx, token); err != nil {
		return nil, fmt.Errorf("unable to authenticate: %w", err)
	}

	if err = db.Use(ctx, cfg.SurrealNamespace(), cfg.SurrealDatabase()); err != nil {
		return nil, fmt.Errorf("unable to use namespace/database: %w", err)
	}

	return db, nil
}
