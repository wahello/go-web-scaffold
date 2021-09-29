package cache

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

var cache *Cache

func TestMain(m *testing.M) { // nolint: staticcheck
	var err error
	defer func() {
		if err != nil {
			log.Fatal(err)
		}
	}()

	redisHost := os.Getenv("TEST_REDIS_HOST")
	if redisHost == "" {
		// uses a sensible default on windows (tcp/http) and linux/osx (socket)
		var pool *dockertest.Pool
		pool, err = dockertest.NewPool("")
		if err != nil {
			err = fmt.Errorf("could not connect to docker: %w", err)
			return
		}

		// pulls an image, creates a container based on it and runs it
		var redis *dockertest.Resource
		redis, err = pool.RunWithOptions(&dockertest.RunOptions{
			Repository: "redis",
			Tag:        "6",
		}, func(config *docker.HostConfig) {
			config.AutoRemove = true
			config.RestartPolicy = docker.RestartPolicy{
				Name: "no",
			}
		})
		if err != nil {
			err = fmt.Errorf("could not start redis: %w", err)
			return
		}
		err = redis.Expire(2 * 60)
		if err != nil {
			err = fmt.Errorf("[resource leaking] failed to set container expire: %w", err)
			return
		}

		defer func() {
			if purgeErr := pool.Purge(redis); purgeErr != nil {
				log.Printf("[resource leaked] could not purge redis: %s", purgeErr)
			}
		}()

		// exponential backoff-retry, because the application in the container might not be ready to accept connections yet
		err = pool.Retry(func() (pingErr error) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			cache, pingErr = NewRedisClient(ctx, RedisConfig{
				Addr: fmt.Sprintf(":%s", redis.GetPort("6379/tcp")),
			})
			return
		})
		if err != nil {
			err = fmt.Errorf("could not connect to docker: %w", err)
			return
		}
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		cache, err = NewRedisClient(ctx, RedisConfig{
			Addr: fmt.Sprintf("%s:6379", redisHost),
		})
		if err != nil {
			err = fmt.Errorf("connecting existing Redis at %q: %w", redisHost, err)
			return
		}
	}

	// m.Run will return an exit code that may be passed to os.Exit.
	// If TestMain returns, the test wrapper will pass the result of m.Run to os.Exit itself.
	m.Run()
}
