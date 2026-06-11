package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/graham/tog/web"
)

// magicLinkRequest is the expected JSON body for requesting a magic link.
type magicLinkRequest struct {
	Email string `json:"email"`
}

// requestMagicLink creates a magic link token for the given email.
// If emailSender is configured, sends email and returns generic message.
// Otherwise returns the token in response (for testing/development).
// POST /auth/magic-link
func (r *Routes) requestMagicLink(w http.ResponseWriter, req *http.Request) {
	var body magicLinkRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		web.WriteAppError(w, req, web.ErrBadRequest("Invalid request body", err))
		return
	}

	if body.Email == "" {
		web.WriteAppError(w, req, web.ErrBadRequest("Email is required", nil))
		return
	}

	// Generic success message to prevent email enumeration
	genericMessage := "If an account exists with this email, a magic link has been sent"
	if r.emailSender == nil {
		genericMessage = "If an account exists with this email, a magic link has been generated"
	}

	// Look up user by email (must exist)
	user, err := r.queries.GetUserByEmail.Exec(body.Email).FirstE()
	if err != nil {
		// Don't reveal whether the user exists - return success either way
		// This prevents email enumeration attacks
		web.WriteJSON(w, map[string]any{
			"message": genericMessage,
		})
		return
	}

	// Check user is active
	if !user.IsActiveUser() {
		// Same message to prevent enumeration
		web.WriteJSON(w, map[string]any{
			"message": genericMessage,
		})
		return
	}

	// Check rate limit - don't create new token if one was created recently
	rateLimitSecs := r.authConfig.MagicLink.RateLimitSecs
	if rateLimitSecs == 0 {
		rateLimitSecs = 30
	}
	cutoff := time.Now().Unix() - int64(rateLimitSecs)
	_, err = r.queries.GetRecentMagicLinkByEmail.Exec(body.Email, cutoff).FirstE()
	if err == nil {
		// Found a recent link - rate limited
		// Return same success message to prevent enumeration
		web.WriteJSON(w, map[string]any{
			"message": genericMessage,
		})
		return
	}

	// Generate magic link token
	token, err := generateMagicLinkToken()
	if err != nil {
		web.WriteAppError(w, req, web.ErrInternal("Failed to generate token", err))
		return
	}

	// Calculate expiration (default 15 minutes)
	lifetime := r.authConfig.MagicLink.TokenLifetime
	if lifetime == 0 {
		lifetime = 15
	}
	expiresAt := time.Now().Add(time.Duration(lifetime) * time.Minute).Unix()

	// Insert magic link
	result := r.queries.InsertMagicLink.Exec(token, body.Email, expiresAt, user.ID)
	if result.Err() != nil {
		web.WriteAppError(w, req, web.ErrInternal("Failed to create magic link", result.Err()))
		return
	}

	// Build verification URL
	verifyPath := "/auth/magic-link/verify?token=" + token
	verifyURL := verifyPath
	if r.authConfig.MagicLink.BaseURL != "" {
		verifyURL = r.authConfig.MagicLink.BaseURL + verifyPath
	}

	// If email sender is configured, send email and return generic message
	if r.emailSender != nil {
		err := r.emailSender.SendMagicLink(req.Context(), body.Email, verifyURL, lifetime)
		if err != nil {
			// Log error but don't expose to user - still return success
			// to prevent enumeration attacks
		}
		web.WriteJSON(w, map[string]any{
			"message": genericMessage,
		})
		return
	}

	// No email sender - return token in response (dev/testing mode)
	web.WriteJSON(w, map[string]any{
		"token":      token,
		"expires_in": lifetime * 60, // seconds
		"verify_url": verifyPath,
	})
}

// verifyMagicLink validates a magic link token and creates a session.
// GET /auth/magic-link/verify?token=ml_xxx
func (r *Routes) verifyMagicLink(w http.ResponseWriter, req *http.Request) {
	token := req.URL.Query().Get("token")
	if token == "" {
		web.WriteAppError(w, req, web.ErrBadRequest("Token is required", nil))
		return
	}

	// Look up magic link
	magicLink, err := r.queries.GetMagicLinkByToken.Exec(token).FirstE()
	if err != nil {
		web.WriteAppError(w, req, web.ErrBadRequest("Invalid or expired token", nil))
		return
	}

	// Check if already used
	if magicLink.IsUsed() {
		web.WriteAppError(w, req, web.ErrBadRequest("Token has already been used", nil))
		return
	}

	// Check expiration
	if magicLink.IsExpired() {
		web.WriteAppError(w, req, web.ErrBadRequest("Token has expired", nil))
		return
	}

	// Mark as used
	usedAt := time.Now().Unix()
	result := r.queries.MarkMagicLinkUsed.Exec(usedAt, token)
	if result.Err() != nil {
		web.WriteAppError(w, req, web.ErrInternal("Failed to process token", result.Err()))
		return
	}

	// Look up user
	user, err := r.queries.GetUserByID.Exec(magicLink.ForUser).FirstE()
	if err != nil {
		web.WriteAppError(w, req, web.ErrInternal("User not found", err))
		return
	}

	// Check user is still active
	if !user.IsActiveUser() {
		web.WriteAppError(w, req, web.ErrForbidden("Account is inactive", nil))
		return
	}

	// Create session (30 day expiration)
	sessionKey, err := r.CreateSession(w, user.ID, "", 30*24*time.Hour)
	if err != nil {
		web.WriteAppError(w, req, web.ErrInternal("Failed to create session", err))
		return
	}

	web.WriteJSON(w, map[string]any{
		"message":     "logged in",
		"session_key": sessionKey,
		"user_id":     user.ID,
		"email":       user.Email,
	})
}

// generateMagicLinkToken generates a random token with "ml_" prefix.
func generateMagicLinkToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return TokenPrefixMagicLink + hex.EncodeToString(bytes), nil
}
