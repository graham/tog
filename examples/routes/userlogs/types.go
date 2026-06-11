package userlogs

// UserLog represents a log entry in the database.
type UserLog struct {
	ID        int    `db:"id" json:"id"`
	AuthorID  int    `db:"author_id" json:"author_id"`
	Message   string `db:"message" json:"message"`
	CreatedAt string `db:"created_at" json:"created_at"`
}
