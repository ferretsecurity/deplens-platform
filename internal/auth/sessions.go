package auth

import (
	"encoding/gob"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/ferretsecurity/deplens-platform/internal/store"
)

type SessionConfig struct {
	SecureCookie bool
}

func NewSessionManager(cfg SessionConfig) *scs.SessionManager {
	gob.Register([]store.MembershipRecord{})

	manager := scs.New()
	manager.Cookie.Name = "deplens_session"
	manager.Cookie.HttpOnly = true
	manager.Cookie.SameSite = http.SameSiteLaxMode
	manager.Cookie.Secure = cfg.SecureCookie
	return manager
}
