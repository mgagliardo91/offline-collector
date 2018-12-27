package main

import (
	"fmt"

	"github.com/gocolly/redisstorage"
	"github.com/mgagliardo91/go-utils"
)

var (
	ADDR     = utils.GetEnvString("REDIS_ADDR", "")
	PORT     = utils.GetEnvString("REDIS_PORT", "")
	PASSWORD = utils.GetEnvString("REDIS_PASSWORD", "")
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
