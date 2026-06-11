package web

import (
	"reflect"
	"sync"
)

// RouteTypeInfo holds type information for a registered route.
// Used by OpenAPI generation to document request/response schemas.
type RouteTypeInfo struct {
	Method     string
	Path       string
	InputType  reflect.Type // nil for NoInput (GET/DELETE)
	OutputType reflect.Type
}

var (
	routeRegistry   = make(map[string]RouteTypeInfo) // key: "METHOD /path"
	routeRegistryMu sync.RWMutex
)

// RegisterRouteTypes captures input/output types for a route.
// Called during route registration to populate the type registry.
func RegisterRouteTypes[In, Out any](method, path string) {
	routeRegistryMu.Lock()
	defer routeRegistryMu.Unlock()

	key := method + " " + path

	var in In
	var out Out

	info := RouteTypeInfo{
		Method: method,
		Path:   path,
	}

	// Don't register NoInput as a real type
	if _, isNoInput := any(in).(NoInput); !isNoInput {
		info.InputType = reflect.TypeOf(in)
	}

	info.OutputType = reflect.TypeOf(out)
	routeRegistry[key] = info
}

// GetRouteTypes returns all registered route type info.
// Used by OpenAPI generation to build schemas.
func GetRouteTypes() map[string]RouteTypeInfo {
	routeRegistryMu.RLock()
	defer routeRegistryMu.RUnlock()

	result := make(map[string]RouteTypeInfo, len(routeRegistry))
	for k, v := range routeRegistry {
		result[k] = v
	}
	return result
}

// GetRouteTypeInfo returns type info for a specific route.
func GetRouteTypeInfo(method, path string) (RouteTypeInfo, bool) {
	routeRegistryMu.RLock()
	defer routeRegistryMu.RUnlock()

	info, ok := routeRegistry[method+" "+path]
	return info, ok
}

// ClearRouteTypes clears the route registry.
// Useful for testing.
func ClearRouteTypes() {
	routeRegistryMu.Lock()
	defer routeRegistryMu.Unlock()
	routeRegistry = make(map[string]RouteTypeInfo)
}
