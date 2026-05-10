package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/alexedwards/scs/pgxstore"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ferretsecurity/deplens-platform/internal/auth"
	"github.com/ferretsecurity/deplens-platform/internal/blob"
	"github.com/ferretsecurity/deplens-platform/internal/config"
	httpapi "github.com/ferretsecurity/deplens-platform/internal/http"
	"github.com/ferretsecurity/deplens-platform/internal/scans"
	"github.com/ferretsecurity/deplens-platform/internal/store"
)

type App struct {
	Config    config.Config
	DB        *pgxpool.Pool
	BlobStore blob.Store
	Handler   http.Handler
}

func New(cfg config.Config) (*App, error) {
	ctx := context.Background()
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	var blobStore blob.Store
	switch cfg.BlobBackend {
	case "filesystem":
		blobStore = blob.NewFileSystemStore(cfg.BlobFilesystemRoot)
	default:
		db.Close()
		return nil, fmt.Errorf("unsupported blob backend %q", cfg.BlobBackend)
	}

	persistence := store.Store{DB: db}
	uploadService := httpapi.NewProductionUploadService(persistence, scans.Service{
		Blob:  blobStore,
		Store: persistence,
	})
	sessionManager := auth.NewSessionManager(auth.SessionConfig{
		SecureCookie: cfg.SessionCookieSecure,
		Store: pgxstore.NewWithConfig(db, pgxstore.Config{
			TableName:       "http_sessions",
			CleanUpInterval: 5 * time.Minute,
		}),
	})
	authRouter := httpapi.NewAuthRouter(httpapi.ProductionAuthService{
		Sessions: sessionManager,
		Store:    persistence,
	})
	tokenRouter := httpapi.NewTokenRouter(sessionManager, httpapi.NewProductionTokenService(persistence))
	apiRouter := httpapi.NewServer(
		uploadService,
		httpapi.NewProductionQueryService(persistence, store.ScanStore{DB: db}, sessionManager),
	)
	handler := sessionManager.LoadAndSave(routeAuthAndAPI(authRouter, tokenRouter, apiRouter))

	return &App{
		Config:    cfg,
		DB:        db,
		BlobStore: blobStore,
		Handler:   handler,
	}, nil
}

func routeAuthAndAPI(authRouter http.Handler, tokenRouter http.Handler, apiRouter http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/login" || r.URL.Path == "/auth/me" || r.URL.Path == "/auth/logout" {
			authRouter.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/v1/tokens" {
			tokenRouter.ServeHTTP(w, r)
			return
		}
		apiRouter.ServeHTTP(w, r)
	})
}
