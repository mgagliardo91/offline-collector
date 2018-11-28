package main

import (
	"fmt"

	"github.com/gocolly/redisstorage"
)

var (
	ADDR     = getEnvString("REDIS_ADDR", "")
	PORT     = getEnvString("REDIS_PORT", "")
	PASSWORD = getEnvString("REDIS_PASSWORD", "")
)

func createRedisStorage() *redisstorage.Storage {
	if len(ADDR) > 0 {
		storage := &redisstorage.Storage{
			Address:  fmt.Sprintf("%s:%s", ADDR, PORT),
			Password: PASSWORD,
			DB:       0,
			Prefix:   "offline",
		}

		return storage
	}

	return nil
}
