package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthRouterReturnsNoContentForLiveness(t *testing.T) {
	handler := NewHealthRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty body", rr.Body.String())
	}
}

func TestHealthRouterReturnsNoContentWhenReady(t *testing.T) {
	checker := &fakeReadinessChecker{}
	handler := NewHealthRouter(checker)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if checker.calls != 1 {
		t.Fatalf("readiness calls = %d, want 1", checker.calls)
	}
	if _, ok := checker.deadline(); !ok {
		t.Fatal("readiness context has no deadline")
	}
}

func TestHealthRouterReturnsServiceUnavailableWhenNotReady(t *testing.T) {
	handler := NewHealthRouter(&fakeReadinessChecker{
		err: errors.New("database unavailable"),
	})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

type fakeReadinessChecker struct {
	calls int
	err   error
	ctx   context.Context
}

func (f *fakeReadinessChecker) CheckReady(ctx context.Context) error {
	f.calls++
	f.ctx = ctx
	return f.err
}

func (f *fakeReadinessChecker) deadline() (time.Time, bool) {
	if f.ctx == nil {
		return time.Time{}, false
	}
	return f.ctx.Deadline()
}
