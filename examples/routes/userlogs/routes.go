package userlogs

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/graham/tog/tools/routes"
	"github.com/graham/tog/web"
	"github.com/graham/tog/web/auth"
)

// Routes handles HTTP requests for user logs.
type Routes struct {
	queries *Queries
}

// NewRoutes creates a new userlogs Routes handler.
func NewRoutes(q *Queries) *Routes {
	return &Routes{queries: q}
}

// Mount returns a function that mounts all user log routes.
func (r *Routes) Mount(prefix string) func(chi.Router) {
	return func(router chi.Router) {
		wrapped := routes.Wrap(router, prefix)
		wrapped.Get("/", r.list)
		wrapped.Post("/", r.create)
	}
}

func (r *Routes) list(w http.ResponseWriter, req *http.Request) {
	webCtx := web.GetContext(req.Context())
	webCtx.Start("list_logs_handler")
	defer webCtx.Stop("list_logs_handler")

	user := auth.MustUserFromContext(req.Context())

	// Parse limit from query param, default to 100
	limit := 100
	if limitStr := req.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	ctx := webCtx.WithQueryLogging(req.Context())

	// Check for ?all=true to list all logs
	if req.URL.Query().Get("all") == "true" {
		logs, err := r.queries.ListAll.ExecCtx(ctx, limit).All()
		if err != nil {
			web.WriteAppError(w, req, web.ErrInternal("Failed to fetch logs", err))
			return
		}
		web.WriteJSON(w, logs)
		return
	}

	// Parse user_id from query param, default to current user
	userID := user.ID
	if userIDStr := req.URL.Query().Get("user_id"); userIDStr != "" {
		if parsed, err := strconv.Atoi(userIDStr); err == nil {
			userID = parsed
		}
	}

	logs, err := r.queries.ListByAuthor.ExecCtx(ctx, userID, limit).All()
	if err != nil {
		web.WriteAppError(w, req, web.ErrInternal("Failed to fetch logs", err))
		return
	}
	web.WriteJSON(w, logs)
}

type createLogRequest struct {
	Message string `json:"message" validate:"required,min=1,max=1000"`
}

func (r *Routes) create(w http.ResponseWriter, req *http.Request) {
	user := auth.MustUserFromContext(req.Context())

	var input createLogRequest
	if !web.Bind(req, w, &input) {
		return
	}

	result := r.queries.InsertLog.Exec(user.ID, input.Message)
	if err := result.Err(); err != nil {
		web.WriteAppError(w, req, web.ErrInternal("Failed to create log", err))
		return
	}

	id, _ := result.LastInsertID()
	w.WriteHeader(http.StatusCreated)
	web.WriteJSON(w, map[string]any{
		"id":      id,
		"message": "log created",
	})
}
