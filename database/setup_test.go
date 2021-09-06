package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"

	"github.com/go-pg/pg/extra/pgdebug"
)

var db *DB

func TestMain(m *testing.M) { // nolint: staticcheck
	var err error
	defer func() {
		if err != nil {
			log.Fatal(err)
		}
	}()

	postgresHost := os.Getenv("TEST_DB_HOST")
	if postgresHost == "" {
		// uses a sensible default on windows (tcp/http) and linux/osx (socket)
		var pool *dockertest.Pool
		pool, err = dockertest.NewPool("")
		if err != nil {
			err = fmt.Errorf("could not connect to docker: %w", err)
			return
		}

		// pulls an image, creates a container based on it and runs it
		var postgres *dockertest.Resource
		postgres, err = pool.RunWithOptions(&dockertest.RunOptions{
			Repository: "postgres",
			Tag:        "11",
			Env: []string{
				"POSTGRES_USER=telescope",
				"POSTGRES_PASSWORD=telescope",
				"listen_addresses = '*'",
			},
		}, func(config *docker.HostConfig) {
			config.AutoRemove = true
			config.RestartPolicy = docker.RestartPolicy{
				Name: "no",
			}
		})
		if err != nil {
			err = fmt.Errorf("could not start postgres: %w", err)
			return
		}
		err = postgres.Expire(2 * 60)
		if err != nil {
			err = fmt.Errorf("[resource leaking] failed to set container expire: %w", err)
			return
		}

		defer func() {
			if purgeErr := pool.Purge(postgres); purgeErr != nil {
				log.Printf("[resource leaked] could not purge postgres: %s", purgeErr)
			}
		}()

		// exponential backoff-retry, because the application in the container might not be ready to accept connections yet
		err = pool.Retry(func() error {
			var pingErr error
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			db, pingErr = NewPostgres(ctx, PostgresConfig{
				Host:         fmt.Sprintf(":%s", postgres.GetPort("5432/tcp")),
				User:         "telescope",
				Password:     "telescope",
				DatabaseName: "telescope",
			})
			return pingErr
		})
		if err != nil {
			err = fmt.Errorf("could not connect to docker: %w", err)
			return
		}
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		db, err = NewPostgres(ctx, PostgresConfig{
			Host:         fmt.Sprintf("%s:5432", postgresHost),
			User:         "telescope",
			Password:     "telescope",
			DatabaseName: "telescope",
		})
		if err != nil {
			err = fmt.Errorf("connecting existing DB at %q: %w", postgresHost, err)
			return
		}
	}

	db.pg.AddQueryHook(pgdebug.DebugHook{
		// Print all queries.
		Verbose: true,
	})

	// init & reset database here

	// m.Run will return an exit code that may be passed to os.Exit.
	// If TestMain returns, the test wrapper will pass the result of m.Run to os.Exit itself.
	m.Run()
}

func interfaces(input interface{}) (output []interface{}) {
	v := reflect.ValueOf(input)
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	length := v.Len()
	output = make([]interface{}, 0, length)

	for i := 0; i < length; i++ {
		output = append(output, v.Index(i).Interface())
	}

	return
}
