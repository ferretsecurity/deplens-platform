package httpapi

import (
	"context"
	"net/http"
	"time"
)

const readinessTimeout = 2 * time.Second

type ReadinessChecker interface {
	CheckReady(ctx context.Context) error
}

func NewHealthRouter(readiness ReadinessChecker) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if readiness == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
		defer cancel()

		if err := readiness.CheckReady(ctx); err != nil {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}
