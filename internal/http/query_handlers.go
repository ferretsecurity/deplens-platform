package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/alexedwards/scs/v2"
	"github.com/ferretsecurity/deplens-platform/internal/assessment"
	"github.com/ferretsecurity/deplens-platform/internal/auth"
	"github.com/ferretsecurity/deplens-platform/internal/blob"
	"github.com/ferretsecurity/deplens-platform/internal/store"
	"github.com/google/uuid"
)

type QueryService interface {
	ListRepositories(r *http.Request, token string) (any, error)
	ListRepositoryManifests(r *http.Request, token string) (any, error)
	ListDependencies(r *http.Request, token string) (any, error)
	ListPackages(r *http.Request, token string) (any, error)
	ListScans(r *http.Request, token string) (any, error)
	GetScan(r *http.Request, token string) (any, error)
	GetScanReport(r *http.Request, token string) (any, error)
	ListScanManifests(r *http.Request, token string) (any, error)
	UpdateScanMetadata(r *http.Request, token string) error
}

type QueryStore interface {
	ListRepositories(ctx context.Context, filter store.RepositoryListFilter) (store.RepositoryListPage, error)
	ListRepositoryManifests(ctx context.Context, tenantID string, repositoryID string) ([]store.RepositoryManifestItem, error)
	ListDependencies(ctx context.Context, tenantID string) ([]store.DependencyListItem, error)
	ListPackages(ctx context.Context, filter store.PackageListFilter) (store.PackageListPage, error)
	ListScans(ctx context.Context, filter store.ScanFilter) ([]store.ScanListItem, error)
	GetScan(ctx context.Context, tenantID string, scanID string) (store.ScanListItem, error)
	GetScanArtifactKey(ctx context.Context, tenantID string, scanID string) (string, error)
	ListScanManifests(ctx context.Context, tenantID string, scanID string) ([]store.ScanManifestItem, error)
	UpdateScanMetadata(ctx context.Context, tenantID string, scanID string, labels map[string]string, annotation string) error
}

type QueryIdentityStore interface {
	TokenLookup
	ListMemberships(context.Context, string) ([]store.MembershipRecord, error)
}

type ProductionQueryService struct {
	Lookup    QueryIdentityStore
	Reads     QueryStore
	Sessions  *scs.SessionManager
	Artifacts blob.Reader
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

func NewProductionQueryService(lookup QueryIdentityStore, reads QueryStore, sessions *scs.SessionManager, artifacts blob.Reader) ProductionQueryService {
	return ProductionQueryService{
		Lookup:    lookup,
		Reads:     reads,
		Sessions:  sessions,
		Artifacts: artifacts,
	}
}

func (s ProductionQueryService) ListRepositories(r *http.Request, token string) (any, error) {
	tenantID, _, err := s.authorizeRead(r, token)
	if err != nil {
		return nil, errUnauthorized
	}

	filter, err := repositoryListFilterFromRequest(r, tenantID)
	if err != nil {
		return nil, newBadRequestError(err)
	}
	return s.Reads.ListRepositories(r.Context(), filter)
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

func (s ProductionQueryService) ListPackages(r *http.Request, token string) (any, error) {
	tenantID, _, err := s.authorizeRead(r, token)
	if err != nil {
		return nil, errUnauthorized
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if utf8.RuneCountInString(query) > 200 {
		return nil, newBadRequestError(fmt.Errorf("q must be at most 200 characters"))
	}
	version := strings.TrimSpace(r.URL.Query().Get("version"))
	if err := store.ValidatePackageVersion(version); err != nil {
		return nil, newBadRequestError(err)
	}
	ecosystem := strings.TrimSpace(r.URL.Query().Get("ecosystem"))
	affectedRange := strings.TrimSpace(r.URL.Query().Get("affected_range"))
	if ecosystem != "" || affectedRange != "" {
		if version != "" {
			return nil, newBadRequestError(fmt.Errorf("use either version or affected_range"))
		}
		if _, err := assessment.NewQuery(ecosystem, query, affectedRange); err != nil {
			return nil, newBadRequestError(err)
		}
	}
	page, err := parsePositiveIntQuery(r, "page", 1)
	if err != nil {
		return nil, newBadRequestError(err)
	}
	pageSize, err := parsePositiveIntQuery(r, "page_size", store.DefaultPackagePageSize)
	if err != nil {
		return nil, newBadRequestError(err)
	}
	if pageSize > 100 {
		return nil, newBadRequestError(fmt.Errorf("page_size must be between 1 and 100"))
	}
	if int64(page-1) > math.MaxInt64/int64(pageSize) {
		return nil, newBadRequestError(fmt.Errorf("page is out of range"))
	}
	return s.Reads.ListPackages(r.Context(), store.PackageListFilter{
		TenantID: tenantID, Query: query, Version: version, Ecosystem: ecosystem, AffectedRange: affectedRange, Page: page, PageSize: pageSize,
	})
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

func (s ProductionQueryService) GetScanReport(r *http.Request, token string) (any, error) {
	tenantID, _, err := s.authorizeRead(r, token)
	if err != nil {
		return nil, errUnauthorized
	}
	scanID := r.PathValue("scan_id")
	if _, err := uuid.Parse(scanID); err != nil {
		return nil, newBadRequestError(err)
	}
	key, err := s.Reads.GetScanArtifactKey(r.Context(), tenantID, scanID)
	if err != nil {
		return nil, err
	}
	payload, err := s.Artifacts.Get(r.Context(), key)
	if err != nil {
		return nil, err
	}
	var artifact struct {
		Report json.RawMessage `json:"report"`
	}
	if err := json.Unmarshal(payload, &artifact); err != nil {
		return nil, err
	}
	return artifact, nil
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
	var scopes []string
	tenantID, role, scopes, err = s.authorizeWrite(r, token)
	if err == nil && token != "" && !hasScope(scopes, "scan:read") {
		return "", "", errUnauthorized
	}
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
	memberships, lookupErr := s.Lookup.ListMemberships(r.Context(), s.Sessions.GetString(r.Context(), "user_id"))
	if lookupErr == nil {
		for _, membership := range memberships {
			if membership.TenantID == tenantID {
				return tenantID, membership.Role, nil, nil
			}
		}
	}
	return "", "", nil, errUnauthorized
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
	mux.HandleFunc("GET /api/v1/packages", func(w http.ResponseWriter, r *http.Request) {
		writeJSONResult(w, r, service.ListPackages)
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
	mux.HandleFunc("GET /api/v1/scans/{scan_id}/report", func(w http.ResponseWriter, r *http.Request) {
		writeJSONResult(w, r, service.GetScanReport)
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

func repositoryListFilterFromRequest(r *http.Request, tenantID string) (store.RepositoryListFilter, error) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) > 200 {
		return store.RepositoryListFilter{}, fmt.Errorf("q must be at most 200 characters")
	}

	page, err := parsePositiveIntQuery(r, "page", 1)
	if err != nil {
		return store.RepositoryListFilter{}, err
	}

	pageSize, err := parsePositiveIntQuery(r, "page_size", store.DefaultRepositoryPageSize)
	if err != nil {
		return store.RepositoryListFilter{}, err
	}
	if pageSize > 100 {
		return store.RepositoryListFilter{}, fmt.Errorf("page_size must be between 1 and 100")
	}

	return store.RepositoryListFilter{
		TenantID: tenantID,
		Query:    query,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func parsePositiveIntQuery(r *http.Request, name string, defaultValue int) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return defaultValue, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return value, nil
}
