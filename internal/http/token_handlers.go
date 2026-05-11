package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/jackc/pgx/v5"

	"github.com/ferretsecurity/deplens-platform/internal/store"
)

type TokenService interface {
	ListTokens(ctx context.Context, tenantID string, role string) ([]store.APITokenMetadata, error)
	CreateToken(ctx context.Context, tenantID string, role string, label string, scopes []string) (string, store.APITokenMetadata, error)
	UpdateToken(ctx context.Context, tenantID string, role string, tokenID string, label string, scopes []string) (store.APITokenMetadata, error)
	DeleteToken(ctx context.Context, tenantID string, role string, tokenID string) error
}

type TokenStore interface {
	CreateAPIToken(ctx context.Context, tenantID string, label string, scopes []string) (string, store.APITokenMetadata, error)
	ListAPITokens(ctx context.Context, tenantID string) ([]store.APITokenMetadata, error)
	UpdateAPIToken(ctx context.Context, tenantID string, tokenID string, label string, scopes []string) (store.APITokenMetadata, error)
	DeleteAPIToken(ctx context.Context, tenantID string, tokenID string) error
}

type ProductionTokenService struct {
	Store TokenStore
}

func NewProductionTokenService(store TokenStore) ProductionTokenService {
	return ProductionTokenService{Store: store}
}

var errBadRequest = errors.New("bad request")
var errNotFound = errors.New("not found")

var allowedTokenScopes = map[string]struct{}{
	"scan:read":           {},
	"scan:write":          {},
	"scan:metadata:write": {},
}

func authorizeTokenManagement(tenantID string, role string) error {
	if tenantID == "" {
		return errUnauthorized
	}
	if role != "owner" && role != "admin" {
		return errUnauthorized
	}
	return nil
}

func validateTokenPayload(label string, scopes []string) (string, []string, error) {
	label = strings.TrimSpace(label)
	if label == "" || len(scopes) == 0 {
		return "", nil, errBadRequest
	}
	for _, scope := range scopes {
		if _, ok := allowedTokenScopes[scope]; !ok {
			return "", nil, errBadRequest
		}
	}
	return label, scopes, nil
}

func (s ProductionTokenService) ListTokens(ctx context.Context, tenantID string, role string) ([]store.APITokenMetadata, error) {
	if err := authorizeTokenManagement(tenantID, role); err != nil {
		return nil, err
	}
	return s.Store.ListAPITokens(ctx, tenantID)
}

func (s ProductionTokenService) CreateToken(ctx context.Context, tenantID string, role string, label string, scopes []string) (string, store.APITokenMetadata, error) {
	if err := authorizeTokenManagement(tenantID, role); err != nil {
		return "", store.APITokenMetadata{}, err
	}
	label, scopes, err := validateTokenPayload(label, scopes)
	if err != nil {
		return "", store.APITokenMetadata{}, err
	}
	token, item, err := s.Store.CreateAPIToken(ctx, tenantID, label, scopes)
	if err != nil {
		return "", store.APITokenMetadata{}, err
	}
	return token, item, nil
}

func (s ProductionTokenService) UpdateToken(ctx context.Context, tenantID string, role string, tokenID string, label string, scopes []string) (store.APITokenMetadata, error) {
	if err := authorizeTokenManagement(tenantID, role); err != nil {
		return store.APITokenMetadata{}, err
	}
	label, scopes, err := validateTokenPayload(label, scopes)
	if err != nil {
		return store.APITokenMetadata{}, err
	}
	item, err := s.Store.UpdateAPIToken(ctx, tenantID, tokenID, label, scopes)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.APITokenMetadata{}, errNotFound
	}
	return item, err
}

func (s ProductionTokenService) DeleteToken(ctx context.Context, tenantID string, role string, tokenID string) error {
	if err := authorizeTokenManagement(tenantID, role); err != nil {
		return err
	}
	err := s.Store.DeleteAPIToken(ctx, tenantID, tokenID)
	if errors.Is(err, pgx.ErrNoRows) {
		return errNotFound
	}
	return err
}

func NewTokenRouter(sessions *scs.SessionManager, service TokenService) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/tokens", func(w http.ResponseWriter, r *http.Request) {
		items, err := service.ListTokens(r.Context(), sessions.GetString(r.Context(), "active_tenant_id"), sessions.GetString(r.Context(), "role"))
		if writeTokenError(w, err) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(items)
	})

	mux.HandleFunc("POST /api/v1/tokens", func(w http.ResponseWriter, r *http.Request) {
		var payload tokenPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}
		label, scopes, err := validateTokenPayload(payload.Label, payload.Scopes)
		if writeTokenError(w, err) {
			return
		}
		token, item, err := service.CreateToken(r.Context(), sessions.GetString(r.Context(), "active_tenant_id"), sessions.GetString(r.Context(), "role"), label, scopes)
		if writeTokenError(w, err) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(createdTokenResponse{
			APITokenMetadata: item,
			Token:            token,
		})
	})

	mux.HandleFunc("PATCH /api/v1/tokens/{tokenID}", func(w http.ResponseWriter, r *http.Request) {
		var payload tokenPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}
		label, scopes, err := validateTokenPayload(payload.Label, payload.Scopes)
		if writeTokenError(w, err) {
			return
		}
		item, err := service.UpdateToken(r.Context(), sessions.GetString(r.Context(), "active_tenant_id"), sessions.GetString(r.Context(), "role"), r.PathValue("tokenID"), label, scopes)
		if writeTokenError(w, err) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(item)
	})

	mux.HandleFunc("DELETE /api/v1/tokens/{tokenID}", func(w http.ResponseWriter, r *http.Request) {
		err := service.DeleteToken(r.Context(), sessions.GetString(r.Context(), "active_tenant_id"), sessions.GetString(r.Context(), "role"), r.PathValue("tokenID"))
		if writeTokenError(w, err) {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

type tokenPayload struct {
	Label  string   `json:"label"`
	Scopes []string `json:"scopes"`
}

type createdTokenResponse struct {
	store.APITokenMetadata
	Token string `json:"token"`
}

func writeTokenError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, errBadRequest):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, errNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, errUnauthorized):
		http.Error(w, err.Error(), http.StatusUnauthorized)
	default:
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
	return true
}
