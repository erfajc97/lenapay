package httpx

import (
	"log/slog"
	"net/http"
	"time"
)

// NewServer returns an http.Server with every timeout set: a zero timeout is a slowloris
// door. Streams (SSE) lift the write deadline per request with http.ResponseController.
func NewServer(addr string, h http.Handler, logger *slog.Logger) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second, // KYC uploads up to 10 MB on phone networks
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    64 << 10,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}
}
