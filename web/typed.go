package web

import (
	"context"
	"net/http"
)

// HandlerFunc is the signature for typed handlers.
// Handlers receive a context and validated input, and return an output or error.
type HandlerFunc[In, Out any] func(ctx context.Context, input In) (Out, error)

// Handle wraps a typed handler into an http.HandlerFunc.
// It automatically binds and validates input, and writes the output as JSON.
//
// Example:
//
//	func (r *Routes) create(ctx context.Context, input CreateItemInput) (*CreateItemOutput, error) {
//	    // input is already validated
//	    return &CreateItemOutput{ID: 1}, nil
//	}
//
//	router.Post("/api/items", web.Handle(r.create, 201))
func Handle[In, Out any](fn HandlerFunc[In, Out], statusCode ...int) http.HandlerFunc {
	status := http.StatusOK
	if len(statusCode) > 0 {
		status = statusCode[0]
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var input In

		// Skip binding for NoInput (GET/DELETE requests with no body)
		if _, isEmpty := any(input).(NoInput); !isEmpty {
			if !Bind(r, w, &input) {
				return
			}
		}

		output, err := fn(r.Context(), input)
		if err != nil {
			if appErr, ok := err.(*AppError); ok {
				WriteAppError(w, r, appErr)
			} else {
				WriteAppError(w, r, ErrInternal("Internal error", err))
			}
			return
		}

		if status != http.StatusOK {
			w.WriteHeader(status)
		}
		WriteJSON(w, output)
	}
}

// NoInput is used for handlers that don't accept a request body.
// Use this for GET and DELETE requests.
type NoInput struct{}

// HandleGet wraps a handler with no input body (for GET requests).
//
// Example:
//
//	func (r *Routes) list(ctx context.Context) ([]Item, error) {
//	    return r.queries.List.All()
//	}
//
//	router.Get("/api/items", web.HandleGet(r.list))
func HandleGet[Out any](fn func(ctx context.Context) (Out, error)) http.HandlerFunc {
	return Handle(func(ctx context.Context, _ NoInput) (Out, error) {
		return fn(ctx)
	})
}

// HandleDelete wraps a handler with no input body (for DELETE requests).
// Alias for HandleGet with different semantics.
func HandleDelete[Out any](fn func(ctx context.Context) (Out, error)) http.HandlerFunc {
	return HandleGet(fn)
}
