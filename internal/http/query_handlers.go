package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/ferretsecurity/deplens-platform/internal/auth"
	"github.com/ferretsecurity/deplens-platform/internal/store"
	"github.com/google/uuid"
)

type QueryService interface {
	ListRepositories(r *http.Request, token string) (any, error)
	ListRepositoryManifests(r *http.Request, token string) (any, error)
	ListDependencies(r *http.Request, token string) (any, error)
	ListScans(r *http.Request, token string) (any, error)
	GetScan(r *http.Request, token string) (any, error)
	ListScanManifests(r *http.Request, token string) (any, error)
	UpdateScanMetadata(r *http.Request, token string) error
}

type QueryStore interface {
	ListRepositories(ctx context.Context, tenantID string) ([]store.RepositoryListItem, error)
	ListRepositoryManifests(ctx context.Context, tenantID string, repositoryID string) ([]store.RepositoryManifestItem, error)
	ListDependencies(ctx context.Context, tenantID string) ([]store.DependencyListItem, error)
	ListScans(ctx context.Context, filter store.ScanFilter) ([]store.ScanListItem, error)
	GetScan(ctx context.Context, tenantID string, scanID string) (store.ScanListItem, error)
	ListScanManifests(ctx context.Context, tenantID string, scanID string) ([]store.ScanManifestItem, error)
	UpdateScanMetadata(ctx context.Context, tenantID string, scanID string, labels map[string]string, annotation string) error
}

type ProductionQueryService struct {
	Lookup   TokenLookup
	Reads    QueryStore
	Sessions *scs.SessionManager
}

type badRequestError struct {
	err error
}

func (e badRequestError) Error() string {
	return e.err.Error()
}

func (e badRequestError) Unwrap() error {
	return e.err
}

func newBadRequestError(err error) error {
	return badRequestError{err: err}
}

func NewProductionQueryService(lookup TokenLookup, reads QueryStore, sessions *scs.SessionManager) ProductionQueryService {
	return ProductionQueryService{
		Lookup:   lookup,
		Reads:    reads,
		Sessions: sessions,
	}
}

func (s ProductionQueryService) ListRepositories(r *http.Request, token string) (any, error) {
	tenantID, _, err := s.authorizeRead(r, token)
	if err != nil {
		return nil, errUnauthorized
	}
	return s.Reads.ListRepositories(r.Context(), tenantID)
}

func (s ProductionQueryService) ListRepositoryManifests(r *http.Request, token string) (any, error) {
	tenantID, _, err := s.authorizeRead(r, token)
	if err != nil {
		return nil, errUnauthorized
	}

	repositoryID := r.PathValue("repository_id")
	if _, err := uuid.Parse(repositoryID); err != nil {
		return nil, newBadRequestError(err)
	}

	items, err := s.Reads.ListRepositoryManifests(r.Context(), tenantID, repositoryID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"items": items}, nil
}

func (s ProductionQueryService) ListDependencies(r *http.Request, token string) (any, error) {
	tenantID, _, err := s.authorizeRead(r, token)
	if err != nil {
		return nil, errUnauthorized
	}

	items, err := s.Reads.ListDependencies(r.Context(), tenantID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"items": items}, nil
}

func (s ProductionQueryService) ListScans(r *http.Request, token string) (any, error) {
	tenantID, _, err := s.authorizeRead(r, token)
	if err != nil {
		return nil, errUnauthorized
	}

	repositoryID := r.URL.Query().Get("repository_id")
	if _, err := uuid.Parse(repositoryID); err != nil {
		return nil, newBadRequestError(err)
	}

	from, err := parseTime(r.URL.Query().Get("from"))
	if err != nil {
		return nil, newBadRequestError(err)
	}
	to, err := parseTime(r.URL.Query().Get("to"))
	if err != nil {
		return nil, newBadRequestError(err)
	}

	return s.Reads.ListScans(r.Context(), store.ScanFilter{
		TenantID:     tenantID,
		RepositoryID: repositoryID,
		From:         from,
		To:           to,
	})
}

func (s ProductionQueryService) GetScan(r *http.Request, token string) (any, error) {
	tenantID, _, err := s.authorizeRead(r, token)
	if err != nil {
		return nil, errUnauthorized
	}
	return s.Reads.GetScan(r.Context(), tenantID, r.PathValue("scan_id"))
}

func (s ProductionQueryService) ListScanManifests(r *http.Request, token string) (any, error) {
	tenantID, _, err := s.authorizeRead(r, token)
	if err != nil {
		return nil, errUnauthorized
	}

	items, err := s.Reads.ListScanManifests(r.Context(), tenantID, r.PathValue("scan_id"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"items": items}, nil
}

func (s ProductionQueryService) UpdateScanMetadata(r *http.Request, token string) error {
	tenantID, role, scopes, err := s.authorizeWrite(r, token)
	if err != nil {
		return errUnauthorized
	}
	if role == "" && !hasScope(scopes, "scan:metadata:write") {
		return errUnauthorized
	}

	var payload struct {
		Labels     map[string]string `json:"labels"`
		Annotation string            `json:"annotation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return err
	}

	return s.Reads.UpdateScanMetadata(r.Context(), tenantID, r.PathValue("scan_id"), payload.Labels, payload.Annotation)
}

func (s ProductionQueryService) authorizeRead(r *http.Request, token string) (tenantID string, role string, err error) {
	tenantID, role, _, err = s.authorizeWrite(r, token)
	return tenantID, role, err
}

func (s ProductionQueryService) authorizeWrite(r *http.Request, token string) (tenantID string, role string, scopes []string, err error) {
	if token != "" {
		tenantID, scopes, err = s.Lookup.FindToken(r.Context(), auth.HashToken(token))
		if err != nil {
			return "", "", nil, errUnauthorized
		}
		return tenantID, "", scopes, nil
	}
	if s.Sessions == nil {
		return "", "", nil, errUnauthorized
	}
	if s.Sessions.GetString(r.Context(), "user_id") == "" {
		return "", "", nil, errUnauthorized
	}
	tenantID = s.Sessions.GetString(r.Context(), "active_tenant_id")
	if tenantID == "" {
		return "", "", nil, errUnauthorized
	}
	role = s.Sessions.GetString(r.Context(), "role")
	return tenantID, role, nil, nil
}

func NewServer(upload UploadService, query QueryService) http.Handler {
	uploadMux := NewRouter(upload)
	queryMux := NewQueryRouter(query)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/scans" {
			uploadMux.ServeHTTP(w, r)
			return
		}
		queryMux.ServeHTTP(w, r)
	})
}

func NewQueryRouter(service QueryService) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/repositories", func(w http.ResponseWriter, r *http.Request) {
		writeJSONResult(w, r, service.ListRepositories)
	})
	mux.HandleFunc("GET /api/v1/repositories/{repository_id}/manifests", func(w http.ResponseWriter, r *http.Request) {
		writeJSONResult(w, r, service.ListRepositoryManifests)
	})
	mux.HandleFunc("GET /api/v1/dependencies", func(w http.ResponseWriter, r *http.Request) {
		writeJSONResult(w, r, service.ListDependencies)
	})
	mux.HandleFunc("GET /api/v1/scans", func(w http.ResponseWriter, r *http.Request) {
		writeJSONResult(w, r, service.ListScans)
	})
	mux.HandleFunc("GET /api/v1/scans/{scan_id}", func(w http.ResponseWriter, r *http.Request) {
		writeJSONResult(w, r, service.GetScan)
	})
	mux.HandleFunc("GET /api/v1/scans/{scan_id}/manifests", func(w http.ResponseWriter, r *http.Request) {
		writeJSONResult(w, r, service.ListScanManifests)
	})
	mux.HandleFunc("PATCH /api/v1/scans/{scan_id}/metadata", func(w http.ResponseWriter, r *http.Request) {
		token := auth.BearerToken(r.Header.Get("Authorization"))
		if err := service.UpdateScanMetadata(r, token); err != nil {
			var badRequest badRequestError
			if errors.As(err, &badRequest) {
				http.Error(w, badRequest.Error(), http.StatusBadRequest)
				return
			}
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	return mux
}

func writeJSONResult(w http.ResponseWriter, r *http.Request, fn func(*http.Request, string) (any, error)) {
	token := auth.BearerToken(r.Header.Get("Authorization"))
	result, err := fn(r, token)
	if err != nil {
		var badRequest badRequestError
		if errors.As(err, &badRequest) {
			http.Error(w, badRequest.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func parseTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339, value)
}
