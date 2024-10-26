package util

import (
	"os"
	"strconv"
)

func GetEnv(name string, defaultValue string) string {
	value, found := os.LookupEnv(name)
	if !found {
		return defaultValue
	}
	return value
}

func GetEnvInt(name string, defaultValue int) int {
	value, found := os.LookupEnv(name)
	if !found {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return intValue
}
