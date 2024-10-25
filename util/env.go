package util

import (
	"os"
)

func GetEnv(name string, defaultValue string) string {
	value, found := os.LookupEnv(name)
	if !found {
		return defaultValue
	}
	return value
}
