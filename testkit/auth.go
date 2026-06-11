package testkit

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
)

// TestUser represents a user created for testing.
type TestUser struct {
	ID       int64
	Email    string
	IsAdmin  bool
	Token    string // Session token for this user
	app      *TestApp
}

// CreateUser creates a new test user and returns it with a valid session token.
// The user is automatically cleaned up when the test ends.
func (app *TestApp) CreateUser(email string, isAdmin bool) *TestUser {
	app.T.Helper()

	database := app.DB()
	if database == nil {
		app.T.Fatal("no database available")
	}

	isAdminInt := 0
	if isAdmin {
		isAdminInt = 1
	}

	// Insert user
	result, err := database.DB.Exec(
		database.Rebind("INSERT INTO users (email, is_admin, is_active) VALUES ($1, $2, 1)"),
		email, isAdminInt)
	if err != nil {
		app.T.Fatalf("failed to create user: %v", err)
	}

	userID, err := result.LastInsertId()
	if err != nil {
		app.T.Fatalf("failed to get user ID: %v", err)
	}

	// Generate session token
	token := generateTestToken()

	// Insert session
	_, err = database.DB.Exec(
		database.Rebind("INSERT INTO sessions (key_value, key_type, for_user, is_active) VALUES ($1, $2, $3, 1)"),
		token, "session", userID)
	if err != nil {
		app.T.Fatalf("failed to create session: %v", err)
	}

	return &TestUser{
		ID:      userID,
		Email:   email,
		IsAdmin: isAdmin,
		Token:   token,
		app:     app,
	}
}

// LoginAs creates a session for an existing user by email and returns it.
// The user must already exist in the database.
func (app *TestApp) LoginAs(email string) *TestUser {
	app.T.Helper()

	database := app.DB()
	if database == nil {
		app.T.Fatal("no database available")
	}

	// Find user by email
	var user struct {
		ID      int64 `db:"id"`
		Email   string `db:"email"`
		IsAdmin int    `db:"is_admin"`
	}
	err := database.DB.Get(&user, database.Rebind("SELECT id, email, is_admin FROM users WHERE email = $1"), email)
	if err != nil {
		app.T.Fatalf("failed to find user %q: %v", email, err)
	}

	// Generate session token
	token := generateTestToken()

	// Insert session
	_, err = database.DB.Exec(
		database.Rebind("INSERT INTO sessions (key_value, key_type, for_user, is_active) VALUES ($1, $2, $3, 1)"),
		token, "session", user.ID)
	if err != nil {
		app.T.Fatalf("failed to create session: %v", err)
	}

	return &TestUser{
		ID:      user.ID,
		Email:   user.Email,
		IsAdmin: user.IsAdmin == 1,
		Token:   token,
		app:     app,
	}
}

// Request makes an authenticated request as this user.
func (u *TestUser) Request(method, path string, body io.Reader) *httptest.ResponseRecorder {
	u.app.T.Helper()
	return u.app.RequestWithCookie(method, path, body, "session_key", u.Token)
}

// RequestWithHeader makes an authenticated request with additional headers.
func (u *TestUser) RequestWithHeader(method, path string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	u.app.T.Helper()
	req := httptest.NewRequest(method, path, body)
	req.AddCookie(&http.Cookie{Name: "session_key", Value: u.Token})
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	u.app.Router.ServeHTTP(rec, req)
	return rec
}

func generateTestToken() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}
