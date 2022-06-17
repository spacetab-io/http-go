package errors

import (
	"errors"
)

var (
	ErrNilRouter        = errors.New("router is nil")
	ErrFHServerShutdown = errors.New("cannot complete graceful shutdown")
	ErrNotFound         = errors.New("route not found")
	ErrNoMethod         = errors.New("method not allowed")
	ErrServerError      = errors.New("internal server error")
	ErrRecordNotFound   = errors.New("record not found")
	ErrConflict         = errors.New("conflict")
)
