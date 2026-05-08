package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ferretsecurity/deplens-platform/internal/auth"
	"github.com/ferretsecurity/deplens-platform/internal/store"
)

type QueryService interface {
	ListRepositories(r *http.Request, token string) (any, error)
	ListScans(r *http.Request, token string) (any, error)
	GetScan(r *http.Request, token string) (any, error)
	ListScanManifests(r *http.Request, token string) (any, error)
	UpdateScanMetadata(r *http.Request, token string) error
}

type QueryStore interface {
	ListRepositories(ctx context.Context, tenantID string) ([]store.RepositoryListItem, error)
	ListScans(ctx context.Context, filter store.ScanFilter) ([]store.ScanListItem, error)
	GetScan(ctx context.Context, tenantID string, scanID string) (store.ScanListItem, error)
	ListScanManifests(ctx context.Context, tenantID string, scanID string) ([]store.ScanManifestItem, error)
	UpdateScanMetadata(ctx context.Context, tenantID string, scanID string, labels map[string]string, annotation string) error
}

type ProductionQueryService struct {
	Lookup TokenLookup
	Reads  QueryStore
}

func NewProductionQueryService(lookup TokenLookup, reads QueryStore) ProductionQueryService {
	return ProductionQueryService{
		Lookup: lookup,
		Reads:  reads,
	}
}

func (s ProductionQueryService) ListRepositories(r *http.Request, token string) (any, error) {
	tenantID, scopes, err := s.Lookup.FindToken(r.Context(), auth.HashToken(token))
	if err != nil || !hasScope(scopes, "scan:read") {
		return nil, errUnauthorized
	}
	return s.Reads.ListRepositories(r.Context(), tenantID)
}

func (s ProductionQueryService) ListScans(r *http.Request, token string) (any, error) {
	tenantID, scopes, err := s.Lookup.FindToken(r.Context(), auth.HashToken(token))
	if err != nil || !hasScope(scopes, "scan:read") {
		return nil, errUnauthorized
	}

	from, err := parseTime(r.URL.Query().Get("from"))
	if err != nil {
		return nil, err
	}
	to, err := parseTime(r.URL.Query().Get("to"))
	if err != nil {
		return nil, err
	}

	return s.Reads.ListScans(r.Context(), store.ScanFilter{
		TenantID:       tenantID,
		RepositorySlug: r.URL.Query().Get("repository_slug"),
		From:           from,
		To:             to,
	})
}

func (s ProductionQueryService) GetScan(r *http.Request, token string) (any, error) {
	tenantID, scopes, err := s.Lookup.FindToken(r.Context(), auth.HashToken(token))
	if err != nil || !hasScope(scopes, "scan:read") {
		return nil, errUnauthorized
	}
	return s.Reads.GetScan(r.Context(), tenantID, r.PathValue("scan_id"))
}

func (s ProductionQueryService) ListScanManifests(r *http.Request, token string) (any, error) {
	tenantID, scopes, err := s.Lookup.FindToken(r.Context(), auth.HashToken(token))
	if err != nil || !hasScope(scopes, "scan:read") {
		return nil, errUnauthorized
	}

	items, err := s.Reads.ListScanManifests(r.Context(), tenantID, r.PathValue("scan_id"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"items": items}, nil
}

func (s ProductionQueryService) UpdateScanMetadata(r *http.Request, token string) error {
	tenantID, scopes, err := s.Lookup.FindToken(r.Context(), auth.HashToken(token))
	if err != nil || !hasScope(scopes, "scan:metadata:write") {
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
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func parseTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339, value)
}
