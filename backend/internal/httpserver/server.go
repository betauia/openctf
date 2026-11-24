// Package httpserver wires the API routes and builds the HTTP server.
package httpserver

import (
	"net/http"
	"time"
)

// NewServer returns an HTTP server for handler that logs every request and recovers from panics.
func NewServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr: addr,
		Handler: Chain(handler,
			// ordering matters people!
			Log,
			Recover,
		),
		ReadHeaderTimeout: 10 * time.Second,
	}
}
