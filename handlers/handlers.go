package handlers

import (
	"net/http"

	slog "github.com/sorintlab/sircles/log"
)

var log = slog.S()

type maxBytesHandler struct {
	h http.Handler
	n int64
}

func NewMaxBytesHandler(h http.Handler, n int64) *maxBytesHandler {
	return &maxBytesHandler{
		h: h,
		n: n,
	}
}

func (h *maxBytesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.n)
	h.h.ServeHTTP(w, r)
}
