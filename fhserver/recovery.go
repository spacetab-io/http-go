package fhserver

import (
	"net/http"

	"github.com/valyala/fasthttp"
)

func recoveryMiddleware(next func(ctx *fasthttp.RequestCtx)) func(ctx *fasthttp.RequestCtx) {
	return func(ctx *fasthttp.RequestCtx) {
		defer func() {
			if rvr := recover(); rvr != nil {
				ctx.Error("recover", http.StatusInternalServerError)
			}
		}()

		// do next
		next(ctx)
	}
}
