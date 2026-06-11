package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	// oauthStateCookieName is the name of the cookie used to store OAuth state.
	oauthStateCookieName = "oauth_state"
	// oauthStateCookieMaxAge is how long the state cookie is valid (5 minutes).
	oauthStateCookieMaxAge = 5 * 60
)

// GoogleUserInfo represents the user info returned by Google's userinfo endpoint.
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// getGoogleOAuthConfig creates an OAuth2 config for Google.
func (r *Routes) getGoogleOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     r.authConfig.OAuth.Google.ClientID,
		ClientSecret: r.authConfig.OAuth.Google.ClientSecret,
		RedirectURL:  r.authConfig.OAuth.Google.RedirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
}

// oauthGoogleStart initiates the Google OAuth flow.
// GET /auth/oauth/google
func (r *Routes) oauthGoogleStart(w http.ResponseWriter, req *http.Request) {
	// Generate state token for CSRF protection
	state, err := generateOAuthState()
	if err != nil {
		http.Error(w, "Failed to generate state", http.StatusInternalServerError)
		return
	}

	// Store state in cookie
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    state,
		Path:     "/",
		MaxAge:   oauthStateCookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.cookieConfig.Secure,
	})

	// Redirect to Google
	config := r.getGoogleOAuthConfig()
	url := config.AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, req, url, http.StatusTemporaryRedirect)
}

// oauthGoogleCallback handles the callback from Google OAuth.
// GET /auth/oauth/google/callback
func (r *Routes) oauthGoogleCallback(w http.ResponseWriter, req *http.Request) {
	// Get state from cookie
	stateCookie, err := req.Cookie(oauthStateCookieName)
	if err != nil {
		r.oauthError(w, req, "Missing state cookie")
		return
	}

	// Verify state matches
	state := req.URL.Query().Get("state")
	if state != stateCookie.Value {
		r.oauthError(w, req, "Invalid state parameter")
		return
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	// Check for error from Google
	if errParam := req.URL.Query().Get("error"); errParam != "" {
		r.oauthError(w, req, "OAuth error: "+errParam)
		return
	}

	// Get authorization code
	code := req.URL.Query().Get("code")
	if code == "" {
		r.oauthError(w, req, "Missing authorization code")
		return
	}

	// Exchange code for token
	config := r.getGoogleOAuthConfig()
	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		r.oauthError(w, req, "Failed to exchange code")
		return
	}

	// Get user info from Google
	userInfo, err := getGoogleUserInfo(token.AccessToken)
	if err != nil {
		r.oauthError(w, req, "Failed to get user info")
		return
	}

	// Verify email is verified
	if !userInfo.VerifiedEmail {
		r.oauthError(w, req, "Email not verified with Google")
		return
	}

	// Look up user by email (must exist)
	user, err := r.queries.GetUserByEmail.Exec(userInfo.Email).FirstE()
	if err != nil {
		r.oauthError(w, req, "No account found for this email")
		return
	}

	// Check user is active
	if !user.IsActiveUser() {
		r.oauthError(w, req, "Account is inactive")
		return
	}

	// Create session (30 day expiration)
	_, err = r.CreateSession(w, user.ID, "", 30*24*time.Hour)
	if err != nil {
		r.oauthError(w, req, "Failed to create session")
		return
	}

	// Redirect to success URL
	successURL := r.authConfig.OAuth.SuccessURL
	if successURL == "" {
		successURL = "/"
	}
	http.Redirect(w, req, successURL, http.StatusTemporaryRedirect)
}

// oauthError redirects to the failure URL with an error message.
func (r *Routes) oauthError(w http.ResponseWriter, req *http.Request, message string) {
	failureURL := r.authConfig.OAuth.FailureURL
	if failureURL == "" {
		failureURL = "/login?error=oauth_failed"
	}
	// Append error message as query param if not already present
	if req.URL.Query().Get("error") == "" {
		if len(failureURL) > 0 && failureURL[len(failureURL)-1] != '?' {
			if !contains(failureURL, "?") {
				failureURL += "?"
			} else {
				failureURL += "&"
			}
		}
		failureURL += "error=" + message
	}
	http.Redirect(w, req, failureURL, http.StatusTemporaryRedirect)
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// getGoogleUserInfo fetches user info from Google's userinfo endpoint.
func getGoogleUserInfo(accessToken string) (*GoogleUserInfo, error) {
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + accessToken)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var userInfo GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

// generateOAuthState generates a random state token for CSRF protection.
func generateOAuthState() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
