package util

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	os.Setenv("TEST", "value")
	value := GetEnv("TEST", "default")
	if value != "value" {
		t.Fail()
	}
}

func TestGetEnvDefault(t *testing.T) {
	value := GetEnv("TEST", "default")
	if value != "default" {
		t.Fail()
	}
}

func TestGetEnvInt(t *testing.T) {
	os.Setenv("TEST", "1234")
	value := GetEnvInt("TEST", 5678)
	if value != 1234 {
		t.Fail()
	}
}

func TestGetEnvIntDefault(t *testing.T) {
	value := GetEnvInt("TEST", 5678)
	if value != 5678 {
		t.Fail()
	}
}
