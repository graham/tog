package items_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/graham/tog/examples/routes"
	"github.com/graham/tog/examples/routes/items"
	"github.com/graham/tog/testkit"
)

// TestItemsRoutes is an integration test for the items API routes.
// It tests owner-based filtering, ensuring users only see their own items.
func TestItemsRoutes(t *testing.T) {
	app := testkit.NewTestApp(t, "../../../examples/migrations").
		WithLoadRoutes(routes.LoadRoutes)

	// Login as admin (owns Widget, Gadget)
	admin := app.LoginAs("admin@example.com")

	// Login as user (owns Gizmo)
	user := app.LoginAs("user@example.com")

	t.Run("admin sees only their items", func(t *testing.T) {
		resp := admin.Request("GET", "/api/items", nil)
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}

		var result []items.Item
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(result) != 2 {
			t.Errorf("admin should see 2 items, got %d", len(result))
		}
	})

	t.Run("user sees only their items", func(t *testing.T) {
		resp := user.Request("GET", "/api/items", nil)
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}

		var result []items.Item
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(result) != 1 {
			t.Errorf("user should see 1 item, got %d", len(result))
		}
		if len(result) > 0 && result[0].Name != "Gizmo" {
			t.Errorf("user's item should be Gizmo, got %q", result[0].Name)
		}
	})

	t.Run("admin can get their own item", func(t *testing.T) {
		resp := admin.Request("GET", "/api/items/1", nil)
		if resp.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.Code)
		}

		var item items.Item
		if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if item.Name != "Widget" {
			t.Errorf("expected name 'Widget', got %q", item.Name)
		}
	})

	t.Run("user cannot get admin's item", func(t *testing.T) {
		resp := user.Request("GET", "/api/items/1", nil)
		if resp.Code != http.StatusNotFound {
			t.Errorf("expected 404 (item belongs to admin), got %d", resp.Code)
		}
	})

	t.Run("create item assigns to current user", func(t *testing.T) {
		body := `{"name":"User's New Item","description":"Created by user","price":5.99}`
		resp := user.Request("POST", "/api/items", strings.NewReader(body))
		if resp.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", resp.Code, resp.Body.String())
		}

		// Verify user now has 2 items
		listResp := user.Request("GET", "/api/items", nil)
		var result []items.Item
		json.NewDecoder(listResp.Body).Decode(&result)
		if len(result) != 2 {
			t.Errorf("user should now have 2 items, got %d", len(result))
		}
	})

	t.Run("create item missing name", func(t *testing.T) {
		body := `{"description":"A test","price":5.99}`
		resp := admin.Request("POST", "/api/items", strings.NewReader(body))
		if resp.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", resp.Code)
		}

		// Verify validation error format
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		if result["error"] != "Validation Error" {
			t.Errorf("expected 'Validation Error', got %v", result["error"])
		}
		fields, ok := result["fields"].(map[string]any)
		if !ok {
			t.Fatalf("expected 'fields' in response, got: %v", result)
		}
		if _, hasName := fields["name"]; !hasName {
			t.Errorf("expected error for 'name' field, got: %v", fields)
		}
	})

	t.Run("create item negative price rejected", func(t *testing.T) {
		body := `{"name":"Test Item","price":-10}`
		resp := admin.Request("POST", "/api/items", strings.NewReader(body))
		if resp.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", resp.Code, resp.Body.String())
		}

		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		fields := result["fields"].(map[string]any)
		if _, hasPrice := fields["price"]; !hasPrice {
			t.Errorf("expected error for 'price' field, got: %v", fields)
		}
	})

	t.Run("unauthenticated request rejected", func(t *testing.T) {
		resp := app.Request("GET", "/api/items", nil)
		if resp.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", resp.Code)
		}
	})
}
