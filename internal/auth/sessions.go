package auth

import (
	"encoding/gob"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/ferretsecurity/deplens-platform/internal/store"
)

type SessionConfig struct {
	SecureCookie bool
	Store        scs.Store
}

func NewSessionManager(cfg SessionConfig) *scs.SessionManager {
	gob.Register([]store.MembershipRecord{})

	manager := scs.New()
	manager.Cookie.Name = "deplens_session"
	manager.Cookie.HttpOnly = true
	manager.Cookie.SameSite = http.SameSiteLaxMode
	manager.Cookie.Secure = cfg.SecureCookie
	manager.HashTokenInStore = true
	if cfg.Store != nil {
		manager.Store = cfg.Store
	}
	return manager
}
