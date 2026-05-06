package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/alexedwards/scs/v2"
)

type TokenService interface {
	CreateToken(ctx context.Context, tenantID string, role string, label string, scopes []string) (string, error)
}

type TokenCreator interface {
	CreateAPIToken(ctx context.Context, tenantID string, label string, scopes []string) (string, error)
}

type ProductionTokenService struct {
	Store TokenCreator
}

func NewProductionTokenService(store TokenCreator) ProductionTokenService {
	return ProductionTokenService{Store: store}
}

func (s ProductionTokenService) CreateToken(ctx context.Context, tenantID string, role string, label string, scopes []string) (string, error) {
	if tenantID == "" {
		return "", errUnauthorized
	}
	if role != "owner" && role != "admin" {
		return "", errUnauthorized
	}
	if label == "" || len(scopes) == 0 {
		return "", errUnauthorized
	}
	return s.Store.CreateAPIToken(ctx, tenantID, label, scopes)
}

func NewTokenRouter(sessions *scs.SessionManager, service TokenService) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/tokens", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Label  string   `json:"label"`
			Scopes []string `json:"scopes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		token, err := service.CreateToken(
			r.Context(),
			sessions.GetString(r.Context(), "active_tenant_id"),
			sessions.GetString(r.Context(), "role"),
			payload.Label,
			payload.Scopes,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":  token,
			"label":  payload.Label,
			"scopes": payload.Scopes,
		})
	})
	return mux
}
