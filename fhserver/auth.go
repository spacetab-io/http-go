package fhserver

import (
	"bytes"
	"encoding/base64"

	"github.com/valyala/fasthttp"
)

var colonDelimiter = []byte(":")

// BasicAuth returns the username and password provided in the request's
// Authorization header, if the request uses HTTP Basic Authentication.
// See RFC 2617, Section 2.
func BasicAuth(payload []byte) (user, pass []byte, exist bool) {
	prefix := []byte("Basic ")
	if !bytes.HasPrefix(payload, prefix) {
		return
	}

	dest := make([]byte, base64.StdEncoding.DecodedLen(len(payload)))

	c, err := base64.StdEncoding.Decode(dest, payload[len(prefix):])
	if err != nil {
		return
	}

	s := bytes.Index(dest, colonDelimiter)
	if s < 0 {
		return
	}

	return dest[:s], dest[s+1 : c], true
}

func AuthorizationHeader(ctx *fasthttp.RequestCtx) (payload []byte, exist bool) {
	authHeader := ctx.Request.Header.Peek("Authorization")
	if len(authHeader) == 0 {
		return nil, false
	}

	return authHeader, true
}
