package app

import (
	"context"
	"fmt"
	"net/http"

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
		SecureCookie: cfg.Mode != "self-hosted",
	})
	authRouter := sessionManager.LoadAndSave(httpapi.NewAuthRouter(httpapi.ProductionAuthService{
		Sessions: sessionManager,
		Store:    persistence,
	}))
	apiRouter := httpapi.NewServer(
		uploadService,
		httpapi.NewProductionQueryService(persistence, store.ScanStore{DB: db}),
	)
	handler := routeAuthAndAPI(authRouter, apiRouter)

	return &App{
		Config:    cfg,
		DB:        db,
		BlobStore: blobStore,
		Handler:   handler,
	}, nil
}

func routeAuthAndAPI(authRouter http.Handler, apiRouter http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/login" {
			authRouter.ServeHTTP(w, r)
			return
		}
		apiRouter.ServeHTTP(w, r)
	})
}
