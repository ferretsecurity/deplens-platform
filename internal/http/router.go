package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/ferretsecurity/deplens-platform/internal/auth"
	"github.com/ferretsecurity/deplens-platform/internal/scans"
	"github.com/ferretsecurity/deplens-platform/internal/store"
)

var errUnauthorized = errors.New("unauthorized")

type UploadService interface {
	Upload(r *http.Request, token string, input scans.UploadRequest) (string, error)
}

type TokenLookup interface {
	FindToken(ctx context.Context, tokenHash string) (tenantID string, scopes []string, err error)
}

type UploadProcessor interface {
	Upload(ctx context.Context, tenantID string, input scans.UploadRequest) (string, error)
}

type ProductionUploadService struct {
	Lookup  TokenLookup
	Uploads UploadProcessor
}

func NewProductionUploadService(lookup TokenLookup, uploads UploadProcessor) ProductionUploadService {
	return ProductionUploadService{
		Lookup:  lookup,
		Uploads: uploads,
	}
}

func (s ProductionUploadService) Upload(r *http.Request, token string, input scans.UploadRequest) (string, error) {
	tenantID, scopes, err := s.Lookup.FindToken(r.Context(), auth.HashToken(token))
	if err != nil {
		return "", errUnauthorized
	}
	if !hasScope(scopes, "scan:write") {
		return "", errUnauthorized
	}
	return s.Uploads.Upload(r.Context(), tenantID, input)
}

type LocalIdentityStore interface {
	FindLocalIdentityByEmail(ctx context.Context, email string) (userID string, passwordHash string, err error)
	ListMemberships(ctx context.Context, userID string) ([]store.MembershipRecord, error)
}

type ProductionAuthService struct {
	Sessions *scs.SessionManager
	Store    LocalIdentityStore
}

func (s ProductionAuthService) Login(r *http.Request, email string, password string) (string, error) {
	userID, passwordHash, err := s.Store.FindLocalIdentityByEmail(r.Context(), email)
	if err != nil {
		return "", errUnauthorized
	}
	ok, err := auth.VerifyPassword(passwordHash, password)
	if err != nil || !ok {
		return "", errUnauthorized
	}
	memberships, err := s.Store.ListMemberships(r.Context(), userID)
	if err != nil {
		return "", err
	}

	if s.Sessions != nil {
		if err := s.Sessions.RenewToken(r.Context()); err != nil {
			return "", err
		}
		s.Sessions.Put(r.Context(), "user_id", userID)
		s.Sessions.Put(r.Context(), "memberships", memberships)
		if len(memberships) == 1 {
			s.Sessions.Put(r.Context(), "active_tenant_id", memberships[0].TenantID)
			s.Sessions.Put(r.Context(), "role", memberships[0].Role)
		}
	}

	return "session", nil
}

func hasScope(scopes []string, want string) bool {
	for _, scope := range scopes {
		if scope == want {
			return true
		}
	}
	return false
}
