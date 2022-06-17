package fhclient

import (
	"context"

	log "github.com/spacetab-io/logs-go/v3"
	"github.com/valyala/fasthttp"
)

type WebClientInterface interface {
	SetLogger(logger log.Logger)
	FastPostByte(ctx context.Context, requestURI string, body []byte) (*fasthttp.Response, error)
	FastPutByte(ctx context.Context, requestURI string, body []byte) (*fasthttp.Response, error)
	FastPatchByte(ctx context.Context, requestURI string, body []byte) (*fasthttp.Response, error)
	FastGet(ctx context.Context, requestURI string) (*fasthttp.Response, error)
}
