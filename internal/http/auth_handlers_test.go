package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoginSetsSessionCookieForValidUser(t *testing.T) {
	handler := NewAuthRouter(fakeAuthService{})

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"email":"admin@example.com","password":"change-me-now"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if len(rr.Result().Cookies()) == 0 {
		t.Fatal("expected session cookie to be set")
	}
}

type fakeAuthService struct{}

func (fakeAuthService) Login(_ *http.Request, email string, password string) (string, error) {
	if email != "admin@example.com" || password != "change-me-now" {
		return "", errUnauthorized
	}
	return "session-token", nil
}
