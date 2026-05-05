package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/ferretsecurity/deplens-platform/internal/auth"
	"github.com/ferretsecurity/deplens-platform/internal/scans"
)

func NewRouter(service UploadService) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/scans", func(w http.ResponseWriter, r *http.Request) {
		token := auth.BearerToken(r.Header.Get("Authorization"))
		if token == "" {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}

		var input scans.UploadRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		scanID, err := service.Upload(r, token, input)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"scan_id": scanID})
	})
	return mux
}
