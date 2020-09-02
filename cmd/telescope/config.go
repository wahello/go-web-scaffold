package main

import (
	"telescope/cache"
	"telescope/controller"
	"telescope/database"
)

type Config struct {
	Log      LogConfig
	API      controller.Config
	Database database.Config
	Redis    cache.RedisConfig
}

type LogConfig struct {
	ProductionMode bool
}
