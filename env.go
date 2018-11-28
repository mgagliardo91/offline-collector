package main

import (
	"log"
	"os"
	"strconv"
)

func getEnvString(key, defaultVal string) string {
	if val, found := os.LookupEnv(key); found {
		return val
	}

	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val, found := os.LookupEnv(key); found {
		if i, err := strconv.Atoi(val); err != nil {
			log.Panicf("Env variable with key=%s and value=%s cannot be converted to an Integer. Error=%v", key, val, err)
		} else {
			return i
		}
	}

	return defaultVal
}
