package database

import (
	"context"
	"fmt"
	"github.com/go-pg/pg/v10"
	"telescope/version"
)

// Config of database
type Config struct {
	Host         string
	User         string
	Password     string
	DatabaseName string
}

// DB is the mother of all database action
type DB struct {
	// Postgres connection pool
	PG *pg.DB
}

// NewDatabase connect to database
func NewDatabase(ctx context.Context, dsn Config) (db *DB, err error) {
	postgres := pg.Connect(&pg.Options{
		Addr:            dsn.Host,
		User:            dsn.User,
		Password:        dsn.Password,
		Database:        dsn.DatabaseName,
		ApplicationName: version.FullName,
	})

	err = postgres.Ping(ctx)
	if err != nil {
		err = fmt.Errorf("postgres.Ping: %w", err)
		return
	}

	db = &DB{
		PG: postgres,
	}

	return
}

// Rollback transaction and ignore error
func (db *DB) Rollback(tx *pg.Tx) {
	_ = tx.Rollback()
}
