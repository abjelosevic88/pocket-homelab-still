package server

import (
	"context"
	"net/http"

	"github.com/abjelosevic88/pocket-homelab-core/sdk/appkit"
)

// Sign-in with an OpenID Connect provider (Pocket Core on a box, or any
// provider set through OIDC_*). The relying party itself — PKCE, discovery,
// the ID token's signature, the return path — is appkit.RP in the Core SDK;
// this file only says where its settings come from and how a verified
// person becomes a Still session. Without Core and without OIDC_* nothing
// here is reachable: the status says oidc=false and the start route is 404.

func (s *Server) newRP() *appkit.RP {
	return &appkit.RP{
		Settings: func() appkit.OIDCSettings {
			return appkit.EnvOrCore(s.core, s.cfg.OIDCIssuer, s.cfg.OIDCClientID, s.cfg.OIDCClientSecret, s.cfg.OIDCName, s.cfg.OIDCAutoCreate)
		},
		CallbackURL: s.oidcCallbackURL,
		Login: func(ctx context.Context, id appkit.Identity, autoCreate bool) (string, error) {
			u, err := s.identityUser(ctx, id.Issuer, id.Subject, id.Email, id.Name, autoCreate)
			if err != nil {
				return "", err
			}
			sid, err := s.store.CreateSession(ctx, u.ID, s.cfg.SessionTTL)
			if err != nil {
				return "", &appkit.LoginError{Message: "Could not start a session", Err: err}
			}
			return sid, nil
		},
		EndSession: func(ctx context.Context, sid string) { _ = s.store.DeleteSession(ctx, sid) },
		SetSession: func(w http.ResponseWriter, sid string) { s.setSessionCookie(w, sid, s.cfg.SessionTTL) },
		Core:       s.core,
		Cookie:     "pk_oidc",
		Secure:     s.cfg.SecureCookies,
		AppName:    "Pocket Still",
		Log:        s.log,
	}
}

func (s *Server) oidcCallbackURL() string { return s.cfg.BaseURL + "/api/v1/auth/oidc/callback" }
