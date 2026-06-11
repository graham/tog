package app

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/graham/tog/tools/routes"
	"github.com/graham/tog/web"
)

func cmdOpenAPI(cfg Config, args []string) {
	fs := newFlagSet(cfg.Name, "openapi", "Generate OpenAPI 3.0 specification from routes.")
	title := fs.String("title", cfg.Name, "API title")
	version := fs.String("version", "1.0.0", "API version")
	fs.Parse(args)

	router, dbm, err := createRouter(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create router: %v\n", err)
		os.Exit(1)
	}
	defer dbm.Close()

	// Collect routes
	routesCfg := routes.Config{ShowAll: false}
	routeInfos := routes.CollectRoutes(router.Mux, routesCfg)

	// Get type registry
	typeInfo := web.GetRouteTypes()

	spec := generateOpenAPISpec(*title, *version, routeInfos, typeInfo)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(spec); err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode JSON: %v\n", err)
		os.Exit(1)
	}
}

func generateOpenAPISpec(title, version string, routeInfos []routes.RouteInfo, typeInfo map[string]web.RouteTypeInfo) map[string]any {
	spec := map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":   title,
			"version": version,
		},
		"paths": make(map[string]any),
		"components": map[string]any{
			"schemas": make(map[string]any),
		},
	}

	paths := spec["paths"].(map[string]any)
	schemas := spec["components"].(map[string]any)["schemas"].(map[string]any)

	for _, route := range routeInfos {
		// Convert chi path params to OpenAPI format
		openAPIPath := convertPathParams(route.Path)

		key := route.Method + " " + route.Path
		info, hasTypes := typeInfo[key]

		pathItem, ok := paths[openAPIPath].(map[string]any)
		if !ok {
			pathItem = make(map[string]any)
			paths[openAPIPath] = pathItem
		}

		operation := map[string]any{
			"responses": map[string]any{
				"200": map[string]any{
					"description": "Success",
				},
			},
		}

		// Add security requirement if route requires auth
		if route.HasAuth {
			operation["security"] = []map[string]any{
				{"cookieAuth": []string{}},
				{"bearerAuth": []string{}},
			}
		}

		// Add path parameters
		pathParams := extractPathParams(route.Path)
		if len(pathParams) > 0 {
			params := make([]map[string]any, len(pathParams))
			for i, param := range pathParams {
				params[i] = map[string]any{
					"name":     param,
					"in":       "path",
					"required": true,
					"schema":   map[string]any{"type": "string"},
				}
			}
			operation["parameters"] = params
		}

		// Add description from route
		if route.Description != "" {
			operation["summary"] = route.Description
		}

		if hasTypes {
			// Add request body schema
			if info.InputType != nil {
				schemaName := getTypeName(info.InputType)
				schemas[schemaName] = typeToSchema(info.InputType)
				operation["requestBody"] = map[string]any{
					"required": true,
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
								"$ref": "#/components/schemas/" + schemaName,
							},
						},
					},
				}
			}

			// Add response schema
			if info.OutputType != nil {
				schemaName := getTypeName(info.OutputType)
				schemas[schemaName] = typeToSchema(info.OutputType)
				operation["responses"].(map[string]any)["200"] = map[string]any{
					"description": "Success",
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
								"$ref": "#/components/schemas/" + schemaName,
							},
						},
					},
				}
			}
		}

		method := strings.ToLower(route.Method)
		pathItem[method] = operation
	}

	// Add security schemes
	spec["components"].(map[string]any)["securitySchemes"] = map[string]any{
		"cookieAuth": map[string]any{
			"type": "apiKey",
			"in":   "cookie",
			"name": "session",
		},
		"bearerAuth": map[string]any{
			"type":   "http",
			"scheme": "bearer",
		},
	}

	return spec
}

// convertPathParams converts chi path params ({id}) to OpenAPI format ({id})
// Chi uses {param} which is already OpenAPI compatible
func convertPathParams(path string) string {
	return path
}

// extractPathParams extracts parameter names from a path like /api/items/{id}
func extractPathParams(path string) []string {
	re := regexp.MustCompile(`\{([^}]+)\}`)
	matches := re.FindAllStringSubmatch(path, -1)
	params := make([]string, len(matches))
	for i, match := range matches {
		params[i] = match[1]
	}
	return params
}

// getTypeName returns a schema-friendly name for a type
func getTypeName(t reflect.Type) string {
	if t == nil {
		return "Object"
	}

	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Handle slices
	if t.Kind() == reflect.Slice {
		return getTypeName(t.Elem()) + "Array"
	}

	name := t.Name()
	if name == "" {
		return "Object"
	}
	return name
}

// typeToSchema converts a Go type to JSON Schema
func typeToSchema(t reflect.Type) map[string]any {
	if t == nil {
		return map[string]any{"type": "object"}
	}

	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.Struct:
		return structToSchema(t)
	case reflect.Slice:
		return map[string]any{
			"type":  "array",
			"items": typeToSchema(t.Elem()),
		}
	case reflect.Map:
		return map[string]any{
			"type":                 "object",
			"additionalProperties": typeToSchema(t.Elem()),
		}
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return map[string]any{"type": "integer"}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer", "minimum": 0}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Interface:
		return map[string]any{} // any type
	default:
		return map[string]any{"type": "object"}
	}
}

func structToSchema(t reflect.Type) map[string]any {
	properties := make(map[string]any)
	required := []string{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get JSON field name
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}

		jsonName := strings.Split(jsonTag, ",")[0]
		if jsonName == "" {
			jsonName = field.Name
		}

		// Build property schema
		prop := typeToSchema(field.Type)

		// Check validation tags for constraints
		validateTag := field.Tag.Get("validate")
		if validateTag != "" {
			applyValidationConstraints(prop, validateTag)
			if strings.Contains(validateTag, "required") {
				required = append(required, jsonName)
			}
		}

		properties[jsonName] = prop
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// applyValidationConstraints adds JSON Schema constraints based on validation tags
func applyValidationConstraints(prop map[string]any, validateTag string) {
	parts := strings.Split(validateTag, ",")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, value := kv[0], kv[1]

		switch key {
		case "min":
			if prop["type"] == "string" {
				prop["minLength"] = parseInt(value)
			} else {
				prop["minimum"] = parseInt(value)
			}
		case "max":
			if prop["type"] == "string" {
				prop["maxLength"] = parseInt(value)
			} else {
				prop["maximum"] = parseInt(value)
			}
		case "gte":
			prop["minimum"] = parseInt(value)
		case "lte":
			prop["maximum"] = parseInt(value)
		case "gt":
			prop["exclusiveMinimum"] = parseInt(value)
		case "lt":
			prop["exclusiveMaximum"] = parseInt(value)
		case "len":
			prop["minLength"] = parseInt(value)
			prop["maxLength"] = parseInt(value)
		case "oneof":
			prop["enum"] = strings.Split(value, " ")
		}
	}

	// Handle email and url formats
	if strings.Contains(validateTag, "email") {
		prop["format"] = "email"
	}
	if strings.Contains(validateTag, "url") {
		prop["format"] = "uri"
	}
	if strings.Contains(validateTag, "uuid") {
		prop["format"] = "uuid"
	}
}

func parseInt(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}
