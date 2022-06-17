package fhserver

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"

	pkgErr "github.com/spacetab-io/http-go/errors"
	log "github.com/spacetab-io/logs-go/v3"
)

// gracefulListener defines a listener that we can gracefully stop.
type gracefulListener struct {
	log *log.Logger

	// inner listener
	ln net.Listener

	// maximum wait time for graceful shutdown
	maxWaitTime time.Duration

	// this channel is closed during graceful shutdown on zero open connections.
	done chan struct{}

	// the number of open connections
	connsCount uint64
	// becomes non-zero when graceful shutdown starts
	shutdown uint64
}

// newGracefulListener wraps the given listener into 'graceful shutdown' listener.
func newGracefulListener(ln net.Listener, maxWaitTime time.Duration, log *log.Logger) net.Listener {
	return &gracefulListener{
		log:         log,
		ln:          ln,
		maxWaitTime: maxWaitTime,
		done:        make(chan struct{}),
	}
}

// Accept creates a conn.
func (ln *gracefulListener) Accept() (net.Conn, error) {
	c, err := ln.ln.Accept()
	if err != nil {
		return nil, fmt.Errorf("gracefulListener accept error: %w", err)
	}

	atomic.AddUint64(&ln.connsCount, 1)

	return &gracefulConn{
		Conn: c,
		ln:   ln,
	}, nil
}

// Addr returns the listen address.
func (ln *gracefulListener) Addr() net.Addr {
	return ln.ln.Addr()
}

// Close closes the inner listener and waits until all the pending
// open connections are closed before returning.
func (ln *gracefulListener) Close() error {
	if err := ln.ln.Close(); err != nil {
		return fmt.Errorf("gracefulListener close error: %w", err)
	}

	return ln.waitForZeroConns()
}

func (ln *gracefulListener) waitForZeroConns() error {
	atomic.AddUint64(&ln.shutdown, 1)

	if atomic.LoadUint64(&ln.connsCount) == 0 {
		close(ln.done)

		return nil
	}

	select {
	case <-ln.done:
		return nil
	case <-time.After(ln.maxWaitTime):
		if ln.log != nil {
			ln.log.Error().Err(pkgErr.ErrFHServerShutdown).Dur("maxWaitTime", ln.maxWaitTime).Send()
		}

		return pkgErr.ErrFHServerShutdown
	}
}

func (ln *gracefulListener) closeConn() {
	connsCount := atomic.AddUint64(&ln.connsCount, ^uint64(0))

	if atomic.LoadUint64(&ln.shutdown) != 0 && connsCount == 0 {
		close(ln.done)
	}
}

type gracefulConn struct {
	net.Conn
	ln *gracefulListener
}

func (c *gracefulConn) Close() error {
	if err := c.Conn.Close(); err != nil {
		return fmt.Errorf("gracefulConn close error: %w", err)
	}

	c.ln.closeConn()

	return nil
}
