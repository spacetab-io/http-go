package fhclient

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spacetab-io/configuration-structs-go/v2/contracts"
	"github.com/spacetab-io/http-go/compress"
	"github.com/spacetab-io/http-go/errors"
	"github.com/spacetab-io/http-go/utils"
	log "github.com/spacetab-io/logs-go/v3"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap/zapcore"
)

const (
	AcceptJSON     = "application/json"
	AcceptXML      = "application/xml"
	ContentJSON    = "application/json; charset=utf-8"
	ContentXML     = "application/xml; charset=utf-8"
	ContentTextXML = "text/xml; charset=utf-8"

	HTTPMethodPOST  = "POST"
	HTTPMethodPUT   = "PUT"
	HTTPMethodPATCH = "PATCH"
	HTTPMethodGET   = "GET"

	DefaultTimeout = 10 * time.Second
)

// authentication, authorization, and accounting

type WebClient struct {
	logger         *log.Logger
	TargetService  string
	BaseURI        string
	JwtToken       string
	UserAgent      string
	ContentType    string
	Accept         string
	TimeOut        time.Duration
	Debug          bool
	Authentication bool
	GzipRequest    bool
}

// New creates Default setup for default fast http client.
func New(cfg contracts.SideRestServiceInterface, serviceName string) *WebClient {
	return &WebClient{
		TargetService:  serviceName,
		Authentication: false,
		UserAgent:      "fasthttpAgent",
		ContentType:    ContentJSON,
		Accept:         AcceptJSON,
		Debug:          cfg.DebugEnable(),
		TimeOut:        cfg.GetTimeout(),
		BaseURI:        strings.TrimRight(cfg.GetBaseURL(), "/"),
		GzipRequest:    cfg.GzipContent(),
	}
}

func (w *WebClient) SetLogger(logger log.Logger) *WebClient {
	w.logger = &logger

	return w
}

// FastPostByte do  POST request via fasthttp.
func (w *WebClient) FastPostByte(ctx context.Context, requestURI string, body []byte) (*fasthttp.Response, error) {
	return w.request(ctx, requestURI, HTTPMethodPOST, body)
}

// FastPutByte do  PUT request via fasthttp.
func (w *WebClient) FastPutByte(ctx context.Context, requestURI string, body []byte) (*fasthttp.Response, error) {
	return w.request(ctx, requestURI, HTTPMethodPUT, body)
}

// FastPatchByte do  PATCH request via fasthttp.
func (w *WebClient) FastPatchByte(ctx context.Context, requestURI string, body []byte) (*fasthttp.Response, error) {
	return w.request(ctx, requestURI, HTTPMethodPATCH, body)
}

// FastGet do GET request via fasthttp.
func (w *WebClient) FastGet(ctx context.Context, requestURI string) (*fasthttp.Response, error) {
	return w.request(ctx, requestURI, HTTPMethodGET, nil)
}

func (w *WebClient) request(ctx context.Context, requestURI string, method string, body []byte) (*fasthttp.Response, error) {
	t := time.Now().UTC()
	methodName := fmt.Sprintf("WebClient %s request", method)
	reqID := requestIDFromContext(ctx)
	uri := w.BaseURI + "/" + strings.TrimLeft(requestURI, "/")
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()

	defer func() {
		fasthttp.ReleaseResponse(resp)
		fasthttp.ReleaseRequest(req)
	}()

	timeOut := w.TimeOut
	if timeOut == 0 {
		timeOut = DefaultTimeout
	}

	e := w.logger.LogEvent().
		Str("method", methodName).
		Str("req.ID", reqID.String()).
		Str("req.timeOut", timeOut.String()).
		Str("req.service", w.TargetService).
		Str("req.method", method).
		Str("req.uri", uri).
		Str("req.user-agent", w.UserAgent).
		Str("req.accept", w.Accept).
		Str("req.content-type", w.ContentType)

	if w.Debug {
		e.SetLogLevel(zapcore.DebugLevel).Str("latency", time.Since(t).String()).Msg("request start")

		defer utils.EndLog(e, t, "request end")
	}

	req.SetRequestURI(uri)
	req.Header.SetContentType(w.ContentType)
	req.Header.Add("User-Agent", w.UserAgent)
	req.Header.Add("Accept", w.Accept)
	req.Header.Add(contracts.ContextKeyRequestID.String(), reqID.String())
	req.Header.SetMethod(method)

	if w.Authentication && len(w.JwtToken) > 0 {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", w.JwtToken))
	}

	switch method {
	case HTTPMethodPOST, HTTPMethodPUT, HTTPMethodPATCH:
		if w.GzipRequest {
			bb, err := compress.ZipContent(body)
			if err != nil {
				return nil, fmt.Errorf("client.request compress.ZipContent error: %w", err)
			}

			req.SetBody(bb)
			req.Header.Set("Content-Encoding", "gzip")
			req.Header.Add("Vary", "Accept-Encoding")
		} else {
			req.SetBody(body)
		}
	}

	if w.Debug {
		e.SetLogLevel(zapcore.DebugLevel).Str("latency", time.Since(t).String()).Msg("send request")
	}

	if err := fasthttp.DoTimeout(req, resp, timeOut); err != nil {
		e.SetLogLevel(zapcore.ErrorLevel).Err(err).Msg("fasthttp send request with timeout error")

		return nil, errors.WrappedError(methodName, "fasthttp.DoTimeout", err)
	}

	// list all response for debug
	if w.Debug {
		e.SetLogLevel(zapcore.DebugLevel).Int("status code", resp.StatusCode()).Str("latency", time.Since(t).String()).Msg("request done")
	}

	out := fasthttp.AcquireResponse()
	resp.CopyTo(out)

	// Do we need to decompress the response?
	contentEncoding := resp.Header.Peek("Content-Encoding")
	if bytes.EqualFold(contentEncoding, []byte("gzip")) {
		body, err := resp.BodyGunzip()
		if err != nil {
			return nil, fmt.Errorf("WebClient resp.BodyGunzip error: %w", err)
		}

		out.SetBody(body)
	}

	return out, nil
}

func requestIDFromContext(ctx context.Context) uuid.UUID {
	reqID, ok := ctx.Value(contracts.ContextKeyRequestID).(uuid.UUID)
	if !ok || reqID == uuid.Nil {
		id := uuid.New()
		reqID = id
	}

	return reqID
}
