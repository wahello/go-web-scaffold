package main

import (
	"telescope/cache"
	"telescope/controller"
	"telescope/database"
)

type Config struct {
	Log      LogConfig
	API      controller.Config
	Postgres database.PostgresConfig
	Redis    cache.RedisConfig
}

type LogConfig struct {
	ProductionMode bool
}
