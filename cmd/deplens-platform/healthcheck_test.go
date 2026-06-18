package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunHealthcheckReturnsNilForSuccessfulProbe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/readyz" {
			t.Fatalf("path = %q, want /readyz", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	if err := runHealthcheck(context.Background(), server.URL+"/readyz"); err != nil {
		t.Fatalf("runHealthcheck() error = %v", err)
	}
}

func TestRunHealthcheckReturnsErrorForUnhealthyStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	if err := runHealthcheck(context.Background(), server.URL+"/readyz"); err == nil {
		t.Fatal("runHealthcheck() error = nil, want error")
	}
}
