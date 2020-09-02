package database

import (
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
func NewDatabase(dsn Config) (db *DB, err error) {
	PG := pg.Connect(&pg.Options{
		Addr:            dsn.Host,
		User:            dsn.User,
		Password:        dsn.Password,
		Database:        dsn.DatabaseName,
		ApplicationName: version.FullName,
	})

	db = &DB{
		PG: PG,
	}

	return
}

// Rollback transaction and ignore error
func (db *DB) Rollback(tx *pg.Tx) {
	_ = tx.Rollback()
}
