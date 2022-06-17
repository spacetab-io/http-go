package fhserver

import (
	"bytes"
	"os"
	"os/signal"
	"sync"
	"syscall"

	cors "github.com/AdhityaRamadhanus/fasthttpcors"
	"github.com/fasthttp/router"
	"github.com/spacetab-io/configuration-structs-go/v2/contracts"
	"github.com/spacetab-io/http-go/errors"
	log "github.com/spacetab-io/logs-go/v3"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/reuseport"
)

var (
	ContentTypeJSON = []byte("application/json")
	ContentTypeText = []byte("text/plain; charset=utf-8")
)

type Server struct {
	log        *log.Logger
	config     contracts.WebServerInterface
	router     *router.Router
	httpServer fasthttp.Server
}

// New creates a new WebServer Server.
func New(config contracts.WebServerInterface) *Server {
	return &Server{
		log: nil,
		httpServer: fasthttp.Server{
			Name:               "Service",
			ReadTimeout:        config.GetReadRequestTimeout(),
			WriteTimeout:       config.GetWriteResponseTimeout(),
			IdleTimeout:        config.GetIdleTimeout(),
			MaxConnsPerIP:      config.GetMaxConnsPerIP(),
			MaxRequestsPerConn: config.GetMaxRequestsPerConn(),
		},
		config: config,
	}
}

// HasAcceptEncodingBytes returns true if the header contains
// the given Accept-Encoding value.
func hasContentEncodingBytes(h *fasthttp.RequestHeader, encoding []byte) bool {
	ae := h.Peek(fasthttp.HeaderContentEncoding)
	n := bytes.Index(ae, encoding)

	if n < 0 {
		return false
	}

	b := ae[n+len(encoding):]

	if len(b) > 0 && b[0] != ',' {
		return false
	}

	if n == 0 {
		return true
	}

	return ae[n-1] == ' '
}

func DecompressRequestHandler(h fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		b := ctx.Request.Body()
		if hasContentEncodingBytes(&ctx.Request.Header, []byte("gzip")) {
			b, _ = ctx.Request.BodyGunzip()
		}

		if hasContentEncodingBytes(&ctx.Request.Header, []byte("deflate")) {
			b, _ = ctx.Request.BodyInflate()
		}

		if hasContentEncodingBytes(&ctx.Request.Header, []byte("br")) {
			b, _ = ctx.Request.BodyUnbrotli()
		}

		ctx.Request.SetBody(b)

		h(ctx)
	}
}

func (s *Server) SetLogger(logger log.Logger) *Server {
	s.log = &logger

	return s
}

func (s *Server) SetRouter(r *router.Router) {
	// compression
	h := r.Handler
	if s.config.UseCompression() {
		h = fasthttp.CompressHandler(h)
		h = DecompressRequestHandler(h)
	}

	// panic and fatal recovery
	h = recoveryMiddleware(h)

	// use custom logging
	h = loggingMiddleware(h, s.log)

	// nolint
	withCors := cors.NewCorsHandler(cors.Options{
		// if you leave allowedOrigins empty then fasthttpcors will treat it as "*"
		AllowedOrigins: []string{}, // Only allow example.com to access the resource
		// if you leave allowedHeaders empty then fasthttpcors will accept any non-simple headers
		AllowedHeaders: []string{}, // only allow x-something-client and Content-Type in actual request
		// if you leave this empty, only simple method will be accepted
		AllowedMethods:   []string{"HEAD", "GET", "POST", "PUT", "DELETE", "OPTIONS"}, // only allow get or post to resource
		AllowCredentials: true,                                                        // resource doesn't support credentials
		AllowMaxAge:      5600,                                                        // cache the preflight result
		Debug:            true,
	})

	if s.config.CORSEnabled() {
		h = withCors.CorsMiddleware(h)
	}

	s.httpServer.Handler = h

	s.router = r
}

// Run starts the HTTP server and performs a graceful shutdown.
func (s *Server) Run(wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}

	if s.log != nil {
		s.log.Debug().Msg("Server Run")
	}

	if s.router == nil {
		if s.log != nil {
			s.log.Error().Err(errors.ErrNilRouter).Send()
		}

		return
	}

	// create a fast listener ;)
	// NOTE: Package reuseport provides a TCP net.Listener with SO_REUSEPORT support.
	// SO_REUSEPORT allows linear scaling server performance on multi-CPU servers.
	ln, err := reuseport.Listen("tcp4", s.config.GetListenAddress())
	if err != nil {
		if s.log != nil {
			s.log.Error().Err(err).Msg("error in reuseport listener")
		}

		return
	}

	// create a graceful shutdown listener
	graceful := newGracefulListener(ln, s.config.GetShutdownTimeout(), s.log)

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		if s.log != nil {
			s.log.Error().Err(err).Msg("hostname unavailable")
		}

		return
	}

	// Error handling
	listenErr := make(chan error, 1)

	// Run server
	go func() {
		if s.log != nil {
			s.log.Debug().Msgf("%s - Web server starting on port %v", hostname, graceful.Addr())
			s.log.Debug().Msgf("%s - Press Ctrl+C to stop", hostname)
		}

		listenErr <- s.httpServer.Serve(graceful) // or s.httpServer.ListenAndServeTLS()
	}()

	// SIGINT/SIGTERM handling
	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, syscall.SIGINT, syscall.SIGTERM)

	// Handle channels/graceful shutdown
signalLoop:
	for {
		select {
		// If server.ListenAndServe() cannot start due to errors such
		// as "port in use" it will return an error.
		case err := <-listenErr:
			if err != nil {
				if s.log != nil {
					s.log.Error().Err(err).Msg("listener error")
				}
			}

			break signalLoop
		// handle termination signal
		case <-osSignals:
			if s.log != nil {
				s.log.Debug().Str("hostname", hostname).Msg("Shutdown signal received.")
			}

			// Servers in the process of shutting down should disable Keep-Alive
			s.httpServer.DisableKeepalive = true

			// Attempt the graceful shutdown by closing the listener
			// and completing all inflight requests.
			if err := graceful.Close(); err != nil {
				if s.log != nil {
					s.log.Error().Err(err).Msg("graceful close error")
				}

				break signalLoop
			}

			// if err := s.httpServer.Shutdown(); err != nil {
			//	log.Error().Err(err).Msg("graceful shutdown error")
			//
			//	break signalLoop
			//}

			if s.log != nil {
				s.log.Debug().Str("hostname", hostname).Msg("Server gracefully stopped.")
			}
		}
	}
}
