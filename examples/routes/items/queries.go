package items

import "github.com/graham/tog/db"

// Queries holds pre-validated queries for the items module.
type Queries struct {
	ListAll          *db.PreparedQuery[Item]
	ListByOwner      *db.PreparedQuery[Item]
	GetByIDAndOwner  *db.PreparedQuery[Item]
	InsertItem       *db.PreparedExec
	UpdateItem       *db.PreparedExec
	DeleteItem       *db.PreparedExec
}

// RegisterQueries registers and returns all item queries.
func RegisterQueries(database *db.DB) (*Queries, error) {
	// ListAll retrieves all active items (no auth required)
	listAll, err := db.Register[Item](database,
		`SELECT id, name, description, price, is_active, created_at, owner_id
		 FROM items
		 WHERE is_active = 1
		 ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	listAll.Desc("Retrieves all active items (public)")

	// ListByOwner retrieves active items for a specific owner
	listByOwner, err := db.Register[Item](database,
		`SELECT id, name, description, price, is_active, created_at, owner_id
		 FROM items
		 WHERE owner_id = $1 AND is_active = 1
		 ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	listByOwner.Desc("Retrieves active items for a specific owner")

	// GetByIDAndOwner retrieves a single item by ID, only if owned by the user
	getByIDAndOwner, err := db.Register[Item](database,
		`SELECT id, name, description, price, is_active, created_at, owner_id
		 FROM items
		 WHERE id = $1 AND owner_id = $2`)
	if err != nil {
		return nil, err
	}
	getByIDAndOwner.Desc("Retrieves an item by ID if owned by the user")

	// InsertItem creates a new item with owner
	insertItem, err := db.RegisterExec(database,
		`INSERT INTO items (name, description, price, owner_id) VALUES ($1, $2, $3, $4)`)
	if err != nil {
		return nil, err
	}
	insertItem.Desc("Creates a new item with name, description, price, and owner")

	// UpdateItem updates an item only if owned by the user
	updateItem, err := db.RegisterExec(database,
		`UPDATE items SET name = $1, description = $2, price = $3 WHERE id = $4 AND owner_id = $5`)
	if err != nil {
		return nil, err
	}
	updateItem.Desc("Updates an item by ID if owned by the user")

	// DeleteItem removes an item only if owned by the user
	deleteItem, err := db.RegisterExec(database,
		`DELETE FROM items WHERE id = $1 AND owner_id = $2`)
	if err != nil {
		return nil, err
	}
	deleteItem.Desc("Deletes an item by ID if owned by the user")

	return &Queries{
		ListAll:         listAll,
		ListByOwner:     listByOwner,
		GetByIDAndOwner: getByIDAndOwner,
		InsertItem:      insertItem,
		UpdateItem:      updateItem,
		DeleteItem:      deleteItem,
	}, nil
}
