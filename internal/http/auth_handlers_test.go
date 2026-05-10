package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/ferretsecurity/deplens-platform/internal/auth"
	"github.com/ferretsecurity/deplens-platform/internal/store"
)

func TestLoginSetsSessionCookieForValidUser(t *testing.T) {
	sessions := auth.NewSessionManager(auth.SessionConfig{})
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

func TestMeReturnsCurrentUserFromSession(t *testing.T) {
	sessions := auth.NewSessionManager(auth.SessionConfig{})
	handler := sessions.LoadAndSave(NewAuthRouter(ProductionAuthService{Sessions: sessions}))
	req := requestWithSession(t, sessions, http.MethodGet, "/auth/me")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var got CurrentUserResponse
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if got.UserID != "user-123" {
		t.Fatalf("UserID = %q, want %q", got.UserID, "user-123")
	}
	if len(got.Memberships) != 1 || got.Memberships[0].TenantID != "tenant-123" {
		t.Fatalf("Memberships = %+v, want tenant-123", got.Memberships)
	}
	if got.ActiveTenantID != "tenant-123" {
		t.Fatalf("ActiveTenantID = %q, want %q", got.ActiveTenantID, "tenant-123")
	}
}

func TestMeReturnsUnauthorizedWithoutSession(t *testing.T) {
	sessions := auth.NewSessionManager(auth.SessionConfig{})
	handler := sessions.LoadAndSave(NewAuthRouter(ProductionAuthService{Sessions: sessions}))

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestLogoutClearsSessionCookie(t *testing.T) {
	sessions := auth.NewSessionManager(auth.SessionConfig{})
	handler := sessions.LoadAndSave(NewAuthRouter(ProductionAuthService{Sessions: sessions}))
	req := requestWithSession(t, sessions, http.MethodPost, "/auth/logout")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if len(rr.Result().Cookies()) == 0 {
		t.Fatal("expected logout to send a clearing cookie")
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
		f.sessions.Put(r.Context(), "memberships", []store.MembershipRecord{{
			TenantID:   "tenant-123",
			TenantSlug: "default",
			Role:       "owner",
		}})
		f.sessions.Put(r.Context(), "active_tenant_id", "tenant-123")
		f.sessions.Put(r.Context(), "role", "owner")
	}
	return "", nil
}

func (f fakeAuthService) CurrentUser(_ *http.Request) (any, error) {
	return nil, errUnauthorized
}

func (f fakeAuthService) Logout(_ *http.Request) error {
	return errUnauthorized
}

func requestWithSession(t *testing.T, sessions *scs.SessionManager, method, target string) *http.Request {
	t.Helper()

	ctx, err := sessions.Load(context.Background(), "")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	sessions.Put(ctx, "user_id", "user-123")
	sessions.Put(ctx, "memberships", []store.MembershipRecord{{
		TenantID:   "tenant-123",
		TenantSlug: "default",
		Role:       "owner",
	}})
	sessions.Put(ctx, "active_tenant_id", "tenant-123")
	sessions.Put(ctx, "role", "owner")

	token, expiry, err := sessions.Commit(ctx)
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	req := httptest.NewRequest(method, target, nil)
	req.AddCookie(&http.Cookie{
		Name:    "deplens_session",
		Value:   token,
		Expires: expiry,
		Path:    "/",
	})
	return req
}
