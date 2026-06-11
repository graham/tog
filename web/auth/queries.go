package auth

import "github.com/graham/tog/db"

// Queries holds pre-validated queries for authentication.
type Queries struct {
	GetSessionByKey       *db.PreparedQuery[Session]
	GetSessionWithUser    *db.PreparedQuery[SessionWithUser]
	GetUserByID           *db.PreparedQuery[User]
	GetUserByEmail        *db.PreparedQuery[User]
	InsertSession         *db.PreparedExec
	InvalidateSession     *db.PreparedExec
	InvalidateAllSessions *db.PreparedExec
	// Magic link queries
	InsertMagicLink           *db.PreparedExec
	GetMagicLinkByToken       *db.PreparedQuery[MagicLink]
	MarkMagicLinkUsed         *db.PreparedExec
	GetRecentMagicLinkByEmail *db.PreparedQuery[MagicLink]
}

// RegisterQueries registers and returns all auth queries.
func RegisterQueries(database *db.DB) (*Queries, error) {
	// GetSessionByKey retrieves an active session by its key value
	getSessionByKey, err := db.Register[Session](database,
		`SELECT id, key_value, key_type, scopes, created_at, expires_at, is_active, for_user
		 FROM sessions
		 WHERE key_value = $1 AND is_active = 1`)
	if err != nil {
		return nil, err
	}
	getSessionByKey.Desc("Retrieves an active session by its key value")

	// GetSessionWithUser retrieves session and user in a single query
	getSessionWithUser, err := db.Register[SessionWithUser](database,
		`SELECT
			s.id as session_id, s.key_value as session_key_value, s.key_type as session_key_type,
			s.scopes as session_scopes, s.created_at as session_created_at, s.expires_at as session_expires_at,
			s.is_active as session_is_active, s.for_user as session_for_user,
			u.id as user_id, u.email as user_email, u.password_hash as user_password_hash,
			u.is_admin as user_is_admin, u.is_active as user_is_active
		 FROM sessions s
		 JOIN users u ON s.for_user = u.id
		 WHERE s.key_value = $1 AND s.is_active = 1`)
	if err != nil {
		return nil, err
	}
	getSessionWithUser.Desc("Retrieves session and user data in a single query for efficient auth")

	// GetUserByID retrieves a user by their ID
	getUserByID, err := db.Register[User](database,
		`SELECT id, email, password_hash, is_admin, is_active
		 FROM users
		 WHERE id = $1`)
	if err != nil {
		return nil, err
	}
	getUserByID.Desc("Retrieves a user by their primary key ID")

	// GetUserByEmail retrieves a user by their email address
	getUserByEmail, err := db.Register[User](database,
		`SELECT id, email, password_hash, is_admin, is_active
		 FROM users
		 WHERE email = $1`)
	if err != nil {
		return nil, err
	}
	getUserByEmail.Desc("Retrieves a user by their email address")

	// InsertSession creates a new session for a user
	insertSession, err := db.RegisterExec(database,
		`INSERT INTO sessions (key_value, key_type, scopes, for_user, is_active, expires_at)
		 VALUES ($1, $2, $3, $4, 1, $5)`)
	if err != nil {
		return nil, err
	}
	insertSession.Desc("Creates a new session for a user with scopes and expiration")

	// InvalidateSession marks a session as inactive (logout)
	invalidateSession, err := db.RegisterExec(database,
		`UPDATE sessions SET is_active = 0 WHERE key_value = $1`)
	if err != nil {
		return nil, err
	}
	invalidateSession.Desc("Invalidates a session by marking it inactive")

	// InvalidateAllSessions marks all sessions for a user as inactive
	invalidateAllSessions, err := db.RegisterExec(database,
		`UPDATE sessions SET is_active = 0 WHERE for_user = $1`)
	if err != nil {
		return nil, err
	}
	invalidateAllSessions.Desc("Invalidates all sessions for a user")

	// InsertMagicLink creates a new magic link token
	insertMagicLink, err := db.RegisterExec(database,
		`INSERT INTO magic_links (token, email, expires_at, for_user)
		 VALUES ($1, $2, $3, $4)`)
	if err != nil {
		return nil, err
	}
	insertMagicLink.Desc("Creates a new magic link token for passwordless login")

	// GetMagicLinkByToken retrieves a magic link by its token
	getMagicLinkByToken, err := db.Register[MagicLink](database,
		`SELECT id, token, email, created_at, expires_at, used_at, for_user
		 FROM magic_links
		 WHERE token = $1`)
	if err != nil {
		return nil, err
	}
	getMagicLinkByToken.Desc("Retrieves a magic link by its token")

	// MarkMagicLinkUsed marks a magic link as used
	markMagicLinkUsed, err := db.RegisterExec(database,
		`UPDATE magic_links SET used_at = $1 WHERE token = $2`)
	if err != nil {
		return nil, err
	}
	markMagicLinkUsed.Desc("Marks a magic link as used with timestamp")

	// GetRecentMagicLinkByEmail checks for recent magic links (rate limiting)
	getRecentMagicLinkByEmail, err := db.Register[MagicLink](database,
		`SELECT id, token, email, created_at, expires_at, used_at, for_user
		 FROM magic_links
		 WHERE email = $1 AND created_at > $2
		 ORDER BY created_at DESC
		 LIMIT 1`)
	if err != nil {
		return nil, err
	}
	getRecentMagicLinkByEmail.Desc("Retrieves most recent magic link for an email after a given time (for rate limiting)")

	return &Queries{
		GetSessionByKey:       getSessionByKey,
		GetSessionWithUser:    getSessionWithUser,
		GetUserByID:           getUserByID,
		GetUserByEmail:        getUserByEmail,
		InsertSession:         insertSession,
		InvalidateSession:     invalidateSession,
		InvalidateAllSessions: invalidateAllSessions,
		InsertMagicLink:           insertMagicLink,
		GetMagicLinkByToken:       getMagicLinkByToken,
		MarkMagicLinkUsed:         markMagicLinkUsed,
		GetRecentMagicLinkByEmail: getRecentMagicLinkByEmail,
	}, nil
}
