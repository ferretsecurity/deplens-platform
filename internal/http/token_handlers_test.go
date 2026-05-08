package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexedwards/scs/v2"
)

func TestCreateTokenReturnsCreatedForOwnerSession(t *testing.T) {
	sessions := scs.New()
	handler := NewTokenRouter(sessions, fakeTokenService{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tokens", bytes.NewBufferString(`{"label":"scanner","scopes":["scan:write","scan:read"]}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(sessionContext(t, sessions, map[string]string{
		"active_tenant_id": "tenant-123",
		"role":             "owner",
	}))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}

	var response struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if response.Token == "" {
		t.Fatal("token = empty, want non-empty")
	}
}

func TestCreateTokenRejectsViewerSession(t *testing.T) {
	sessions := scs.New()
	handler := NewTokenRouter(sessions, fakeTokenService{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tokens", bytes.NewBufferString(`{"label":"scanner","scopes":["scan:write"]}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(sessionContext(t, sessions, map[string]string{
		"active_tenant_id": "tenant-123",
		"role":             "viewer",
	}))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

type fakeTokenService struct{}

func (fakeTokenService) CreateToken(_ context.Context, tenantID string, role string, label string, scopes []string) (string, error) {
	if tenantID != "tenant-123" {
		return "", errUnauthorized
	}
	if role != "owner" && role != "admin" {
		return "", errUnauthorized
	}
	if label == "" || len(scopes) == 0 {
		return "", errUnauthorized
	}
	return "plain-token", nil
}

func sessionContext(t *testing.T, sessions *scs.SessionManager, values map[string]string) context.Context {
	t.Helper()

	ctx, err := sessions.Load(context.Background(), "")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	for key, value := range values {
		sessions.Put(ctx, key, value)
	}
	return ctx
}
