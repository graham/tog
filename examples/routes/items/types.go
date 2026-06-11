package items

// Item represents a product in the database.
type Item struct {
	ID          int     `db:"id" json:"id"`
	Name        string  `db:"name" json:"name"`
	Description string  `db:"description" json:"description"`
	Price       float64 `db:"price" json:"price"`
	IsActive    int     `db:"is_active" json:"is_active"`
	CreatedAt   string  `db:"created_at" json:"created_at"`
	OwnerID     int     `db:"owner_id" json:"owner_id"`
}
