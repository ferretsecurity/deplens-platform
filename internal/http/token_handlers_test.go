package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/ferretsecurity/deplens-platform/internal/store"
)

func TestListTokensReturnsMetadataOnlyForOwnerSession(t *testing.T) {
	sessions := scs.New()
	handler := NewTokenRouter(sessions, fakeTokenService{
		items: []store.APITokenMetadata{
			{ID: "token-1", Label: "scanner", Scopes: []string{"scan:write"}, CreatedAt: time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tokens", nil)
	req = req.WithContext(sessionContext(t, sessions, map[string]string{
		"active_tenant_id": "tenant-123",
		"role":             "owner",
	}))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if strings.Contains(body, "plain-token") || strings.Contains(body, "token_hash") {
		t.Fatalf("response leaked secret material: %s", body)
	}
	if !strings.Contains(body, "scanner") || !strings.Contains(body, "scan:write") {
		t.Fatalf("response missing token metadata: %s", body)
	}
}

func TestCreateTokenReturnsPlaintextOnceForOwnerSession(t *testing.T) {
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
		ID     string   `json:"id"`
		Token  string   `json:"token"`
		Label  string   `json:"label"`
		Scopes []string `json:"scopes"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if response.Token != "plain-token" {
		t.Fatalf("token = %q, want plain-token", response.Token)
	}
	if response.ID != "token-1" || response.Label != "scanner" {
		t.Fatalf("metadata = %#v, want created token metadata", response)
	}
}

func TestUpdateTokenChangesLabelAndScopes(t *testing.T) {
	sessions := scs.New()
	handler := NewTokenRouter(sessions, fakeTokenService{})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tokens/token-1", bytes.NewBufferString(`{"label":"ci scanner","scopes":["scan:read"]}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(sessionContext(t, sessions, map[string]string{
		"active_tenant_id": "tenant-123",
		"role":             "admin",
	}))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	var response store.APITokenMetadata
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if response.ID != "token-1" || response.Label != "ci scanner" {
		t.Fatalf("response = %#v, want updated metadata", response)
	}
	if strings.Contains(rr.Body.String(), "plain-token") {
		t.Fatalf("update response leaked token secret: %s", rr.Body.String())
	}
}

func TestDeleteTokenReturnsNoContent(t *testing.T) {
	sessions := scs.New()
	handler := NewTokenRouter(sessions, fakeTokenService{})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/tokens/token-1", nil)
	req = req.WithContext(sessionContext(t, sessions, map[string]string{
		"active_tenant_id": "tenant-123",
		"role":             "owner",
	}))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", rr.Body.String())
	}
}

func TestTokenRouterRejectsViewerSession(t *testing.T) {
	sessions := scs.New()
	handler := NewTokenRouter(sessions, fakeTokenService{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tokens", nil)
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

func TestTokenRouterRejectsInvalidScopes(t *testing.T) {
	sessions := scs.New()
	handler := NewTokenRouter(sessions, fakeTokenService{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tokens", bytes.NewBufferString(`{"label":"scanner","scopes":["admin:*"]}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(sessionContext(t, sessions, map[string]string{
		"active_tenant_id": "tenant-123",
		"role":             "owner",
	}))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestTokenRouterReturnsNotFoundForMissingTenantToken(t *testing.T) {
	sessions := scs.New()
	handler := NewTokenRouter(sessions, fakeTokenService{})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tokens/missing", bytes.NewBufferString(`{"label":"scanner","scopes":["scan:write"]}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(sessionContext(t, sessions, map[string]string{
		"active_tenant_id": "tenant-123",
		"role":             "owner",
	}))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

type fakeTokenService struct {
	items []store.APITokenMetadata
}

func (f fakeTokenService) ListTokens(_ context.Context, tenantID string, role string) ([]store.APITokenMetadata, error) {
	if tenantID != "tenant-123" || role == "viewer" {
		return nil, errUnauthorized
	}
	return f.items, nil
}

func (fakeTokenService) CreateToken(_ context.Context, tenantID string, role string, label string, scopes []string) (string, store.APITokenMetadata, error) {
	if tenantID != "tenant-123" || role == "viewer" {
		return "", store.APITokenMetadata{}, errUnauthorized
	}
	if label != "scanner" {
		return "", store.APITokenMetadata{}, errUnauthorized
	}
	return "plain-token", store.APITokenMetadata{ID: "token-1", Label: label, Scopes: scopes}, nil
}

func (fakeTokenService) UpdateToken(_ context.Context, tenantID string, role string, tokenID string, label string, scopes []string) (store.APITokenMetadata, error) {
	if tenantID != "tenant-123" || role == "viewer" {
		return store.APITokenMetadata{}, errUnauthorized
	}
	if tokenID == "missing" {
		return store.APITokenMetadata{}, errNotFound
	}
	return store.APITokenMetadata{ID: tokenID, Label: label, Scopes: scopes}, nil
}

func (fakeTokenService) DeleteToken(_ context.Context, tenantID string, role string, tokenID string) error {
	if tenantID != "tenant-123" || role == "viewer" {
		return errUnauthorized
	}
	if tokenID == "missing" {
		return errNotFound
	}
	return nil
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
