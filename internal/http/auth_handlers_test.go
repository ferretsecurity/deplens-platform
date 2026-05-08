package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexedwards/scs/v2"
)

func TestLoginSetsSessionCookieForValidUser(t *testing.T) {
	sessions := scs.New()
	handler := sessions.LoadAndSave(NewAuthRouter(fakeAuthService{sessions: sessions}))

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

type fakeAuthService struct {
	sessions *scs.SessionManager
}

func (f fakeAuthService) Login(r *http.Request, email string, password string) (string, error) {
	if email != "admin@example.com" || password != "change-me-now" {
		return "", errUnauthorized
	}
	if f.sessions != nil {
		f.sessions.Put(r.Context(), "user_id", "user-123")
	}
	return "", nil
}
