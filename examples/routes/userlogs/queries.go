package userlogs

import "github.com/graham/tog/db"

// Queries holds pre-validated queries for the userlogs module.
type Queries struct {
	ListAll      *db.PreparedQuery[UserLog]
	ListByAuthor *db.PreparedQuery[UserLog]
	InsertLog    *db.PreparedExec
}

// RegisterQueries registers and returns all user log queries.
func RegisterQueries(database *db.DB) (*Queries, error) {
	// ListAll retrieves all logs with limit
	listAll, err := db.Register[UserLog](database,
		`SELECT id, author_id, message, created_at
		 FROM user_logs
		 ORDER BY created_at DESC
		 LIMIT $1`)
	if err != nil {
		return nil, err
	}
	listAll.Desc("Retrieves all logs with limit")

	// ListByAuthor retrieves logs for a specific author with limit
	listByAuthor, err := db.Register[UserLog](database,
		`SELECT id, author_id, message, created_at
		 FROM user_logs
		 WHERE author_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2`)
	if err != nil {
		return nil, err
	}
	listByAuthor.Desc("Retrieves logs for a specific author with limit")

	// InsertLog creates a new log entry
	insertLog, err := db.RegisterExec(database,
		`INSERT INTO user_logs (author_id, message) VALUES ($1, $2)`)
	if err != nil {
		return nil, err
	}
	insertLog.Desc("Creates a new log entry for a user")

	return &Queries{
		ListAll:      listAll,
		ListByAuthor: listByAuthor,
		InsertLog:    insertLog,
	}, nil
}
