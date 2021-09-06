package database

import (
	"context"
	"fmt"
	"sync"
	"telescope/version"
	"time"

	"github.com/go-pg/pg/v10/orm"

	"github.com/go-pg/pg/v10"
)

// PostgresConfig of database
type PostgresConfig struct {
	Host         string
	User         string
	Password     string
	DatabaseName string
}

// Operator is where database access/write methods implemented
// so that we don't have to write the same method on both DB and Transaction.
type Operator struct {
	core operatorCore
}

// operatorCore here is what we can use to implement our own database access/write methods.
type operatorCore interface {
	ExecContext(c context.Context, query interface{}, params ...interface{}) (result pg.Result, err error)
	ExecOneContext(ctx context.Context, query interface{}, params ...interface{}) (result pg.Result, err error)
	QueryContext(c context.Context, model, query interface{}, params ...interface{}) (result pg.Result, err error)
	QueryOneContext(ctx context.Context, model, query interface{}, params ...interface{}) (result pg.Result, err error)
	ModelContext(c context.Context, model ...interface{}) *orm.Query
}

// DB is the mother of all database action
type DB struct {
	// Operator is where database access/write methods implemented
	Operator

	// Actual driver supports
	pg *pg.DB

	// Listener and callbacks
	listenerOnce    sync.Once
	listener        *pg.Listener
	topicCallbackMu sync.RWMutex
	topicCallbacks  map[string][]func(context.Context, pg.Notification)
}

// RunInTransaction runs a function in a transaction.
// If function returns an error transaction is rolled back, otherwise transaction is committed.
func (db *DB) RunInTransaction(ctx context.Context, fn func(tx Operator) (err error)) (txErr error) {
	txErr = db.pg.RunInTransaction(ctx, func(dbTx *pg.Tx) error {
		op := Operator{core: dbTx}
		return fn(op)
	})

	return
}

// NewPostgres connect to database
func NewPostgres(ctx context.Context, dsn PostgresConfig) (db *DB, err error) {
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
		pg: postgres,
		Operator: Operator{
			core: postgres,
		},
	}

	return
}

// Notify sends a message
func (op Operator) Notify(ctx context.Context, topic string, payload string) (err error) {
	_, err = op.core.ExecContext(ctx, "NOTIFY ?, ?", pg.Ident(topic), payload)
	if err != nil {
		err = fmt.Errorf("op.core.ExecContext: %w", err)
		return
	}

	return
}

// Listen to specific topic for messages
func (db *DB) Listen(ctx context.Context, topic ...string) (channel <-chan pg.Notification, closeListener func() error) {
	listener := db.pg.Listen(ctx, topic...)
	channel = listener.Channel()
	closeListener = listener.Close
	return
}

// Close closes the database client, releasing any open resources.
func (db *DB) Close() (errs []error) {
	if db.listener != nil {
		err := db.listener.Close()
		if err != nil {
			err = fmt.Errorf("db.listener.Close: %w", err)
			errs = append(errs, err)
		}
	}
	err := db.pg.Close()
	if err != nil {
		err = fmt.Errorf("db.pg.Close: %w", err)
		errs = append(errs, err)
	}

	return
}

// Watch register callback function on specified topic.
//
// Refer to https://www.postgresql.org/docs/11/sql-listen.html
func (db *DB) Watch(ctx context.Context, callback func(context.Context, pg.Notification), topic ...string) (err error) {
	db.listenerOnce.Do(func() {
		db.listener = db.pg.Listen(ctx, topic...)
		db.topicCallbacks = make(map[string][]func(context.Context, pg.Notification))
		go db.watch()
	})

	// It's ok to listen to the same topic for several times.
	// https://www.postgresql.org/docs/11/sql-listen.html
	err = db.listener.Listen(ctx, topic...)
	if err != nil {
		err = fmt.Errorf("db.listener.Listen: %w", err)
		return
	}

	db.topicCallbackMu.Lock()
	defer db.topicCallbackMu.Unlock()
	for _, t := range topic {
		db.topicCallbacks[t] = append(db.topicCallbacks[t], callback)
	}

	return
}

func (db *DB) watch() {
	channel := db.listener.Channel()
	for notify := range channel {
		cbs := db.getWatchCallbacks(notify.Channel)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		for _, cb := range cbs {
			cb(ctx, notify)
		}
		cancel()
	}
}

func (db *DB) getWatchCallbacks(topic string) (cbs []func(context.Context, pg.Notification)) {
	db.topicCallbackMu.RLock()
	defer db.topicCallbackMu.RUnlock()

	cbs = db.topicCallbacks[topic]

	return
}
