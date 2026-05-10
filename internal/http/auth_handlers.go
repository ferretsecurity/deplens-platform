package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/ferretsecurity/deplens-platform/internal/store"
)

type AuthService interface {
	Login(r *http.Request, email string, password string) (sessionToken string, err error)
	CurrentUser(r *http.Request) (any, error)
	Logout(r *http.Request) error
}

type CurrentUserResponse struct {
	UserID         string                   `json:"user_id"`
	Memberships    []store.MembershipRecord `json:"memberships"`
	ActiveTenantID string                   `json:"active_tenant_id,omitempty"`
	Role           string                   `json:"role,omitempty"`
}

func NewAuthRouter(service AuthService) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/login", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		_, err := service.Login(r, payload.Email, payload.Password)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /auth/me", func(w http.ResponseWriter, r *http.Request) {
		user, err := service.CurrentUser(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(user)
	})
	mux.HandleFunc("POST /auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if err := service.Logout(r); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func (s ProductionAuthService) CurrentUser(r *http.Request) (any, error) {
	if s.Sessions == nil {
		return nil, errUnauthorized
	}

	userID := s.Sessions.GetString(r.Context(), "user_id")
	if userID == "" {
		return nil, errUnauthorized
	}

	memberships, _ := s.Sessions.Get(r.Context(), "memberships").([]store.MembershipRecord)
	response := CurrentUserResponse{
		UserID:      userID,
		Memberships: memberships,
		Role:        s.Sessions.GetString(r.Context(), "role"),
	}
	if activeTenantID := s.Sessions.GetString(r.Context(), "active_tenant_id"); activeTenantID != "" {
		response.ActiveTenantID = activeTenantID
	}
	return response, nil
}

func (s ProductionAuthService) Logout(r *http.Request) error {
	if s.Sessions == nil {
		return errUnauthorized
	}
	return s.Sessions.Destroy(r.Context())
}
