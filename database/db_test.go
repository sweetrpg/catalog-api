package database

import (
	"github.com/stretchr/testify/assert"
	"github.com/sweetrpg/catalog-api/constants"
	"os"
	"testing"
)

func TestBuildURLFromURI(t *testing.T) {
	os.Setenv(constants.DB_URI, "mongo://user:pass@host:12345/db?opts=these")
	dbUrl, dbName := buildDbURL()
	assert.Equal(t, dbUrl.Scheme, "mongo")
	assert.Equal(t, dbUrl.User.Username(), "user")
	// assert.Equal(t, dbUrl.User.Password(), "pass")
	assert.Equal(t, dbUrl.Host, "host:12345")
	assert.Equal(t, dbUrl.Query().Get("opts"), "these")
	assert.Equal(t, dbName, "db")
}

func TestBuildURLFromParts(t *testing.T) {
	os.Unsetenv(constants.DB_URI)
	os.Setenv(constants.DB_NAME, "db")
	os.Setenv(constants.DB_HOST, "host")
	os.Setenv(constants.DB_SCHEME, "mongo")
	os.Setenv(constants.DB_USER, "user")
	os.Setenv(constants.DB_PW, "pass")
	os.Setenv(constants.DB_PORT, "12345")
	os.Setenv(constants.DB_OPTS, "opts=these")

	dbUrl, dbName := buildDbURL()
	assert.Equal(t, dbUrl.Scheme, "mongo")
	assert.Equal(t, dbUrl.User.Username(), "user")
	// assert.Equal(t, dbUrl.User.Password(), "pass")
	assert.Equal(t, dbUrl.Host, "host:12345")
	assert.Equal(t, dbUrl.Query().Get("opts"), "these")
	assert.Equal(t, dbName, "db")
}
