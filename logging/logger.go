package logging

import (
	"github.com/sweetrpg/catalog-api/constants"
	"github.com/sweetrpg/catalog-api/util"
	"github.com/zerodha/logf"
	"time"
)

var Logger logf.Logger

func Init() {
	level := logf.InfoLevel
	switch util.GetEnv(constants.LOG_LEVEL, constants.INFO) {
	case constants.DEBUG:
		level = logf.DebugLevel
	case constants.WARN:
		level = logf.WarnLevel
	case constants.ERROR:
		level = logf.ErrorLevel
	}

	Logger = logf.New(logf.Opts{
		EnableColor:          true,
		Level:                level,
		CallerSkipFrameCount: 3,
		EnableCaller:         true,
		TimestampFormat:      time.RFC3339,
		DefaultFields:        []any{},
	})
}
