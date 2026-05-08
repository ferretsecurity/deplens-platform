package httpapi

import (
	"encoding/json"
	"net/http"
)

type AuthService interface {
	Login(r *http.Request, email string, password string) (sessionToken string, err error)
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
	return mux
}
