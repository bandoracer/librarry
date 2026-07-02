package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/bandoracer/librarry/backend/internal/auth"
	compatdata "github.com/bandoracer/librarry/backend/internal/compat"
)

// Auth middleware (M6.2, arr parity): API keys always work for compat
// clients; otherwise the active method decides. /healthz, /ping, /feed/*
// (apikey-checked), POST /api/v1/login, and GET /api/v1/auth/status stay
// reachable without a session. Only the API enforces auth — the nginx-served
// static UI stays public and gates itself on auth/status.
func withAuth(apiKey string, authService *auth.Service, next http.Handler) http.Handler {
	apiKey = strings.TrimSpace(apiKey)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authExemptPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/feed/") {
			if feedRequestAllowed(r, apiKey, authService) {
				next.ServeHTTP(w, r)
				return
			}
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": "apikey query parameter is required for feeds",
			})
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		if openAPIAuthPath(r) {
			next.ServeHTTP(w, r)
			return
		}
		// Compat clients: a valid API key always wins, regardless of method.
		if apiKey != "" && validAPIKey(r, apiKey) {
			next.ServeHTTP(w, r)
			return
		}
		switch effectiveAuthMethod(authService) {
		case auth.MethodBasic:
			if username, password, ok := r.BasicAuth(); ok {
				if _, valid := authService.VerifyPassword(r.Context(), username, password); valid {
					next.ServeHTTP(w, r)
					return
				}
			}
			w.Header().Set("WWW-Authenticate", `Basic realm="librarry"`)
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "authentication required"})
		case auth.MethodForms:
			if cookie, err := r.Cookie(auth.SessionCookieName); err == nil {
				if _, valid := authService.ValidateSession(r.Context(), cookie.Value); valid {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "authentication required"})
		default:
			// Method none preserves the pre-M6 contract: a configured API key
			// still protects /api/*.
			if apiKey == "" {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("WWW-Authenticate", `X-Api-Key realm="librarry"`)
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "API key is missing or invalid"})
		}
	})
}

// effectiveAuthMethod degrades to none when the users table is unavailable
// (no database) so a broken install cannot lock the operator out.
func effectiveAuthMethod(authService *auth.Service) string {
	if authService == nil || !authService.Available() {
		return auth.MethodNone
	}
	return authService.Method()
}

// feedRequestAllowed gates /feed/*: open when no API key and no auth method
// are configured; otherwise the apikey query parameter (or header) must match.
func feedRequestAllowed(r *http.Request, apiKey string, authService *auth.Service) bool {
	if apiKey != "" {
		return validAPIKey(r, apiKey)
	}
	return effectiveAuthMethod(authService) == auth.MethodNone
}

func openAPIAuthPath(r *http.Request) bool {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/login":
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/auth/status":
		return true
	default:
		return false
	}
}

/* -------------------------------- handlers -------------------------------- */

// authStatus reports the active method plus whether this request already
// carries valid credentials (session cookie, Basic, API key, or open).
func (h *handler) authStatus(w http.ResponseWriter, r *http.Request) {
	method := effectiveAuthMethod(h.deps.Auth)
	status := map[string]any{
		"method":        method,
		"authenticated": false,
	}
	apiKey := strings.TrimSpace(h.deps.Config.APIKey)
	switch {
	case method == auth.MethodNone:
		status["authenticated"] = apiKey == "" || validAPIKey(r, apiKey)
	case apiKey != "" && validAPIKey(r, apiKey):
		status["authenticated"] = true
	case method == auth.MethodBasic:
		if username, password, ok := r.BasicAuth(); ok {
			if user, valid := h.deps.Auth.VerifyPassword(r.Context(), username, password); valid {
				status["authenticated"] = true
				status["username"] = user.Username
			}
		}
	case method == auth.MethodForms:
		if cookie, err := r.Cookie(auth.SessionCookieName); err == nil {
			if user, valid := h.deps.Auth.ValidateSession(r.Context(), cookie.Value); valid {
				status["authenticated"] = true
				status["username"] = user.Username
			}
		}
	}
	writeJSON(w, http.StatusOK, status)
}

type loginPayload struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	RememberMe bool   `json:"rememberMe"`
}

func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	if h.deps.Auth == nil || !h.deps.Auth.Available() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "authentication requires database persistence"})
		return
	}
	defer r.Body.Close()
	var payload loginPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid login payload"})
		return
	}
	result, err := h.deps.Auth.Login(r.Context(), payload.Username, payload.Password, payload.RememberMe)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid username or password"})
			return
		}
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	cookie := &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    result.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	if result.Remember {
		cookie.MaxAge = int(auth.RememberMeTTL.Seconds())
	}
	http.SetCookie(w, cookie)
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "username": result.Username})
}

func (h *handler) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(auth.SessionCookieName); err == nil && h.deps.Auth != nil {
		h.deps.Auth.Logout(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
}

type authConfigPayload struct {
	Method   string `json:"method"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// updateAuthConfig switches the auth method and (optionally) updates the
// single user's credentials. A blank password keeps the stored one. Reaching
// this handler already required an authenticated/apikey caller.
func (h *handler) updateAuthConfig(w http.ResponseWriter, r *http.Request) {
	if h.deps.Auth == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "authentication service is unavailable"})
		return
	}
	defer r.Body.Close()
	var payload authConfigPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid auth config payload"})
		return
	}
	method := auth.NormalizeMethod(strings.ToLower(strings.TrimSpace(payload.Method)))
	if method == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "method must be none, basic, or forms"})
		return
	}
	if method != auth.MethodNone && !h.deps.Auth.Available() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "authentication requires database persistence"})
		return
	}
	if strings.TrimSpace(payload.Username) != "" {
		if err := h.deps.Auth.EnsureUser(r.Context(), payload.Username, payload.Password); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
	}
	if method != auth.MethodNone && !h.deps.Auth.HasUser(r.Context()) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "username and password are required to enable authentication"})
		return
	}
	h.deps.Auth.SetMethod(method)
	h.persistAuthMethod(r.Context(), method)
	writeJSON(w, http.StatusOK, map[string]any{"method": method})
}

// persistAuthMethod stores the method in a compat resource so restarts keep
// the UI-chosen mode (an explicit LIBRARRY_AUTH_METHOD env still wins at boot).
func (h *handler) persistAuthMethod(ctx context.Context, method string) {
	if h.deps.Compat == nil {
		return
	}
	_, err := h.deps.Compat.UpsertResource(ctx, compatdata.Resource{
		ResourceType: authConfigResourceType,
		CompatID:     1,
		Name:         "auth-config",
		Payload:      map[string]any{"method": method},
	})
	if err != nil && h.deps.Logger != nil {
		h.deps.Logger.Warn("auth method persistence failed", "error", err)
	}
}

// authConfigResourceType is the compat resource that persists the auth method.
const authConfigResourceType = "auth-config"
