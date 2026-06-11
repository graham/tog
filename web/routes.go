package web

import (
	"context"

	"github.com/go-chi/chi/v5"
)

// Post registers a POST route with typed input/output.
// The handler receives validated input and returns output as JSON.
//
// Example:
//
//	web.Post(r, "/api/items", routes.create, 201)
func Post[In, Out any](r chi.Router, path string, fn HandlerFunc[In, Out], status ...int) {
	RegisterRouteTypes[In, Out]("POST", path)
	r.Post(path, Handle(fn, status...))
}

// Get registers a GET route with typed output.
// GET handlers don't receive input bodies.
//
// Example:
//
//	web.Get(r, "/api/items", routes.list)
func Get[Out any](r chi.Router, path string, fn func(context.Context) (Out, error)) {
	RegisterRouteTypes[NoInput, Out]("GET", path)
	r.Get(path, HandleGet(fn))
}

// Put registers a PUT route with typed input/output.
//
// Example:
//
//	web.Put(r, "/api/items/{id}", routes.update)
func Put[In, Out any](r chi.Router, path string, fn HandlerFunc[In, Out]) {
	RegisterRouteTypes[In, Out]("PUT", path)
	r.Put(path, Handle(fn))
}

// Patch registers a PATCH route with typed input/output.
//
// Example:
//
//	web.Patch(r, "/api/items/{id}", routes.patch)
func Patch[In, Out any](r chi.Router, path string, fn HandlerFunc[In, Out]) {
	RegisterRouteTypes[In, Out]("PATCH", path)
	r.Patch(path, Handle(fn))
}

// Delete registers a DELETE route with typed output.
// DELETE handlers typically don't receive input bodies.
//
// Example:
//
//	web.Delete(r, "/api/items/{id}", routes.delete)
func Delete[Out any](r chi.Router, path string, fn func(context.Context) (Out, error)) {
	RegisterRouteTypes[NoInput, Out]("DELETE", path)
	r.Delete(path, HandleDelete(fn))
}
