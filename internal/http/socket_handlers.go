package httpapi

import (
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/ferretsecurity/deplens-platform/internal/socket"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func NewSocketRouter(sessions *scs.SessionManager, service *socket.Service, appURL string) http.Handler {
	mux := http.NewServeMux()
	respond := func(w http.ResponseWriter, result interface{}, err error) {
		if err != nil {
			status := http.StatusBadGateway
			message := "Socket operation could not be completed. Check the connection and try again."
			switch {
			case errors.Is(err, socket.ErrConflict):
				status = http.StatusConflict
				message = err.Error()
			case errors.Is(err, socket.ErrConfig):
				status = http.StatusBadRequest
				message = err.Error()
			case errors.Is(err, pgx.ErrNoRows):
				status = http.StatusNotFound
				message = "Not found"
			}
			http.Error(w, message, status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	}
	tenant := func(r *http.Request) string { return sessions.GetString(r.Context(), "active_tenant_id") }
	mux.HandleFunc("GET /api/v1/integrations/socket", func(w http.ResponseWriter, r *http.Request) {
		cfg, err := service.Configuration(r.Context(), tenant(r))
		respond(w, cfg, err)
	})
	mux.HandleFunc("PUT /api/v1/integrations/socket", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Organization string  `json:"organization"`
			Token        *string `json:"token"`
			Interval     int     `json:"interval_minutes"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&body); err != nil {
			http.Error(w, "Invalid configuration", 400)
			return
		}
		err := service.Save(r.Context(), tenant(r), strings.TrimSpace(body.Organization), body.Token, body.Interval)
		respond(w, map[string]bool{"saved": err == nil}, err)
	})
	mux.HandleFunc("DELETE /api/v1/integrations/socket", func(w http.ResponseWriter, r *http.Request) {
		respond(w, map[string]bool{"disconnected": true}, service.Disconnect(r.Context(), tenant(r)))
	})
	mux.HandleFunc("POST /api/v1/integrations/socket/test", func(w http.ResponseWriter, r *http.Request) {
		respond(w, map[string]bool{"connected": true}, service.Test(r.Context(), tenant(r)))
	})
	mux.HandleFunc("POST /api/v1/integrations/socket/sync", func(w http.ResponseWriter, r *http.Request) {
		id, err := service.Start(r.Context(), tenant(r))
		respond(w, map[string]string{"run_id": id}, err)
	})
	mux.HandleFunc("GET /api/v1/integrations/socket/status", func(w http.ResponseWriter, r *http.Request) {
		cfg, err := service.Configuration(r.Context(), tenant(r))
		respond(w, cfg, err)
	})
	mux.HandleFunc("PUT /api/v1/integrations/socket/repositories/{repository_id}/mapping", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			CLI *string `json:"cli_repository_id"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&body); err != nil {
			http.Error(w, "Invalid mapping", 400)
			return
		}
		respond(w, map[string]bool{"saved": true}, service.MapRepository(r.Context(), tenant(r), r.PathValue("repository_id"), body.CLI))
	})
	mux.HandleFunc("GET /api/v1/repositories/{repository_id}/inventory", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("repository_id")
		if _, err := uuid.Parse(id); err != nil {
			http.Error(w, "Invalid repository ID", 400)
			return
		}
		result, err := service.Inventory(r.Context(), tenant(r), id)
		respond(w, result, err)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, tenantID := sessions.GetString(r.Context(), "user_id"), tenant(r)
		if userID == "" || tenantID == "" {
			http.Error(w, "Authentication required", 401)
			return
		}
		var role string
		if err := service.DB.QueryRow(r.Context(), `select role from tenant_memberships where tenant_id=$1 and user_id=$2`, tenantID, userID).Scan(&role); err != nil {
			http.Error(w, "Membership required", 403)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/integrations/") && role != "owner" && role != "admin" {
			http.Error(w, "Administrator access required", 403)
			return
		}
		if r.Method != http.MethodGet {
			media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if media != "application/json" {
				http.Error(w, "JSON required", 415)
				return
			}
			origin := r.Header.Get("Origin")
			configured, _ := url.Parse(appURL)
			allowed := configured != nil && origin == configured.Scheme+"://"+configured.Host
			requestOrigin, e := url.Parse(origin)
			sameHost := e == nil && requestOrigin.Host == r.Host && (requestOrigin.Scheme == "http" || requestOrigin.Scheme == "https")
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" || (origin != "" && !allowed && !sameHost) {
				http.Error(w, "Same-origin request required", 403)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		mux.ServeHTTP(w, r)
	})
}
