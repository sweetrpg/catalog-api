package util

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	os.Setenv("TGE", "value")
	value := GetEnv("TGE", "default")
	if value != "value" {
		t.Fail()
	}
}

func TestGetEnvDefault(t *testing.T) {
	value := GetEnv("TGED", "default")
	if value != "default" {
		t.Fail()
	}
}

func TestGetEnvInt(t *testing.T) {
	os.Setenv("TGEI", "1234")
	value := GetEnvInt("TGEI", 5678)
	if value != 1234 {
		t.Fail()
	}
}

func TestGetEnvIntDefault(t *testing.T) {
	value := GetEnvInt("TGEID", 5678)
	if value != 5678 {
		t.Fail()
	}
}
