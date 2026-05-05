package auth

import (
	"net/http"

	"github.com/alexedwards/scs/v2"
)

type SessionConfig struct {
	SecureCookie bool
}

func NewSessionManager(cfg SessionConfig) *scs.SessionManager {
	manager := scs.New()
	manager.Cookie.Name = "deplens_session"
	manager.Cookie.HttpOnly = true
	manager.Cookie.SameSite = http.SameSiteLaxMode
	manager.Cookie.Secure = cfg.SecureCookie
	return manager
}
