package fhserver

import (
	"net/http"
	"time"

	log "github.com/spacetab-io/logs-go/v3"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap/zapcore"
)

// loggingMiddleware is same as Combined but colored.
func loggingMiddleware(req fasthttp.RequestHandler, logger *log.Logger) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		begin := time.Now()

		req(ctx)

		defer func() {
			end := time.Now()
			statusCode := ctx.Response.Header.StatusCode()

			event := logger.LogEvent().
				Int("status", statusCode).
				Bytes("method", ctx.Method()).
				Bytes("path", ctx.RequestURI()).
				Bytes("ip", ctx.RemoteIP()).
				Dur("latency", end.Sub(begin)).
				Bytes("user-agent", ctx.UserAgent())

			switch {
			case statusCode >= http.StatusBadRequest && statusCode < http.StatusInternalServerError:
				event.SetLogLevel(zapcore.WarnLevel).Send()
			case statusCode >= http.StatusInternalServerError:
				event.SetLogLevel(zapcore.ErrorLevel).Send()
			default:
				event.SetLogLevel(zapcore.DebugLevel).Send()
			}
		}()
	}
}
