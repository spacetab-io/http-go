package utils

import (
	"time"

	log "github.com/spacetab-io/logs-go/v3"
	"go.uber.org/zap/zapcore"
)

func EndLog(l *log.Event, t time.Time, msg string) {
	if msg == "" {
		msg = "end"
	}

	l.SetLogLevel(zapcore.DebugLevel).Str("latency", time.Since(t).String()).Msg(msg)
}
