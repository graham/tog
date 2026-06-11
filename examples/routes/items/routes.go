package items

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/graham/tog/tools/routes"
	"github.com/graham/tog/web"
	"github.com/graham/tog/web/auth"
)

// Routes handles HTTP requests for items.
type Routes struct {
	queries *Queries
}

// NewRoutes creates a new items Routes handler.
func NewRoutes(q *Queries) *Routes {
	return &Routes{queries: q}
}

// Mount returns a function that mounts all item routes.
// The prefix parameter is used to register accurate source locations for documentation.
func (r *Routes) Mount(prefix string) func(chi.Router) {
	return func(router chi.Router) {
		wrapped := routes.Wrap(router, prefix)
		wrapped.Get("/", r.list)
		wrapped.Get("/{id}", r.getByID)
		wrapped.Post("/", r.create)
		wrapped.Put("/{id}", r.update)
		wrapped.Delete("/{id}", r.delete)
		wrapped.Get("/test", r.testing)
	}
}

// MountPublic mounts public (no auth) item routes for testing.
func (r *Routes) MountPublic(prefix string) func(chi.Router) {
	return func(router chi.Router) {
		wrapped := routes.Wrap(router, prefix)
		wrapped.Get("/", r.listPublic)
	}
}

func (r *Routes) listPublic(w http.ResponseWriter, req *http.Request) {
	webCtx := web.GetContext(req.Context())

	// Example: time the entire handler
	webCtx.Start("list_public_handler")
	defer webCtx.Stop("list_public_handler")

	// Use ExecCtx to log query timing
	ctx := webCtx.WithQueryLogging(req.Context())
	items, err := r.queries.ListAll.ExecCtx(ctx).All()
	if err != nil {
		web.WriteAppError(w, req, web.ErrInternal("Failed to fetch items", err))
		return
	}
	web.WriteJSON(w, items)
}

func (r *Routes) list(w http.ResponseWriter, req *http.Request) {
	webCtx := web.GetContext(req.Context())

	// Example: time the entire handler
	webCtx.Start("list_handler")
	defer webCtx.Stop("list_handler")

	user := auth.MustUserFromContext(req.Context())

	// Use ExecCtx to log query timing
	ctx := webCtx.WithQueryLogging(req.Context())
	items, err := r.queries.ListByOwner.ExecCtx(ctx, user.ID).All()
	if err != nil {
		web.WriteAppError(w, req, web.ErrInternal("Failed to fetch items", err))
		return
	}
	web.WriteJSON(w, items)
}

func (r *Routes) getByID(w http.ResponseWriter, req *http.Request) {
	user := auth.MustUserFromContext(req.Context())
	id := chi.URLParam(req, "id")
	item, err := r.queries.GetByIDAndOwner.Exec(id, user.ID).FirstE()
	if err != nil {
		web.WriteAppError(w, req, web.ErrNotFound("Item not found", err))
		return
	}
	web.WriteJSON(w, item)
}

type createItemRequest struct {
	Name        string  `json:"name" validate:"required,min=1,max=100"`
	Description string  `json:"description" validate:"max=500"`
	Price       float64 `json:"price" validate:"gte=0"`
}

func (r *Routes) create(w http.ResponseWriter, req *http.Request) {
	user := auth.MustUserFromContext(req.Context())

	var input createItemRequest
	if !web.Bind(req, w, &input) {
		return // Error already written
	}

	result := r.queries.InsertItem.Exec(input.Name, input.Description, input.Price, user.ID)
	if err := result.Err(); err != nil {
		web.WriteAppError(w, req, web.ErrInternal("Failed to create item", err))
		return
	}

	id, _ := result.LastInsertID()
	w.WriteHeader(http.StatusCreated)
	web.WriteJSON(w, map[string]any{
		"id":      id,
		"message": "item created",
	})
}

type updateItemRequest struct {
	Name        string  `json:"name" validate:"required,min=1,max=100"`
	Description string  `json:"description" validate:"max=500"`
	Price       float64 `json:"price" validate:"gte=0"`
}

func (r *Routes) update(w http.ResponseWriter, req *http.Request) {
	user := auth.MustUserFromContext(req.Context())
	id := chi.URLParam(req, "id")

	var input updateItemRequest
	if !web.Bind(req, w, &input) {
		return
	}

	result := r.queries.UpdateItem.Exec(input.Name, input.Description, input.Price, id, user.ID)
	if err := result.Err(); err != nil {
		web.WriteAppError(w, req, web.ErrInternal("Failed to update item", err))
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		web.WriteAppError(w, req, web.ErrNotFound("Item not found", nil))
		return
	}

	web.WriteJSON(w, map[string]string{"message": "item updated"})
}

func (r *Routes) delete(w http.ResponseWriter, req *http.Request) {
	user := auth.MustUserFromContext(req.Context())
	id := chi.URLParam(req, "id")

	result := r.queries.DeleteItem.Exec(id, user.ID)
	if err := result.Err(); err != nil {
		web.WriteAppError(w, req, web.ErrInternal("Failed to delete item", err))
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		web.WriteAppError(w, req, web.ErrNotFound("Item not found", nil))
		return
	}

	web.WriteJSON(w, map[string]string{"message": "item deleted"})
}

func (r *Routes) testing(w http.ResponseWriter, req *http.Request) {
	web.WriteJSON(w, map[string]string{"message": "foobar!!!"})
}
