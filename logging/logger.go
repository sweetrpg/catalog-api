package logging

import (
	"github.com/zerodha/logf"
	"time"
)

var Logger logf.Logger

func Init() {
	Logger = logf.New(logf.Opts{
		EnableColor:          true,
		Level:                logf.DebugLevel,
		CallerSkipFrameCount: 3,
		EnableCaller:         true,
		TimestampFormat:      time.RFC3339,
		DefaultFields:        []any{},
	})
}
