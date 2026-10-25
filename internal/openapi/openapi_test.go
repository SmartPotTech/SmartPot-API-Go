package openapi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDocumentMetadata(t *testing.T) {
	doc := Document("https://api.smartpot.app")
	if doc["openapi"] != "3.0.1" {
		t.Errorf("versión openapi = %v", doc["openapi"])
	}
	info := doc["info"].(gin.H)
	if info["title"] != "SmartPot API" || info["version"] != "2.0.0" {
		t.Errorf("info = %v", info)
	}
	servers := doc["servers"].([]gin.H)
	if len(servers) != 1 || servers[0]["url"] != "https://api.smartpot.app" {
		t.Errorf("servers = %v", servers)
	}
	comps := doc["components"].(gin.H)
	bearer := comps["securitySchemes"].(gin.H)["bearer"].(gin.H)
	if bearer["scheme"] != "bearer" || bearer["bearerFormat"] != "JWT" {
		t.Errorf("esquema bearer = %v", bearer)
	}
	if _, ok := comps["schemas"].(gin.H)["ErrorResponse"]; !ok {
		t.Error("falta el esquema ErrorResponse")
	}
}

// Todas las operaciones del catálogo deben aparecer exactamente una vez.
func TestDocumentRendersEveryRoute(t *testing.T) {
	paths := Document("http://x")["paths"].(gin.H)
	count := 0
	for _, item := range paths {
		count += len(item.(gin.H))
	}
	if count != len(routes) {
		t.Fatalf("operaciones renderizadas %d, se esperaban %d", count, len(routes))
	}
	for _, r := range routes {
		item, ok := paths[r.path].(gin.H)
		if !ok {
			t.Fatalf("falta el camino %s", r.path)
		}
		if _, ok := item[strings.ToLower(r.method)]; !ok {
			t.Fatalf("falta %s %s", r.method, r.path)
		}
	}
}

// Las rutas públicas anulan el token con security: []; las demás lo heredan.
func TestSecurityOverrides(t *testing.T) {
	paths := Document("http://x")["paths"].(gin.H)

	login := paths["/api/v1/auth/login"].(gin.H)["post"].(gin.H)
	sec, ok := login["security"].([]gin.H)
	if !ok || len(sec) != 0 {
		t.Errorf("login debería tener security: [] (sin token), se obtuvo %v", login["security"])
	}

	list := paths["/api/v1/crops"].(gin.H)["get"].(gin.H)
	if _, has := list["security"]; has {
		t.Errorf("una ruta autenticada no debe fijar security propio (hereda el global): %v", list["security"])
	}
}

func TestPathParameters(t *testing.T) {
	paths := Document("http://x")["paths"].(gin.H)
	op := paths["/api/v1/crops/{cropId}/channels/{type}/recipients/{recipientId}"].(gin.H)["delete"].(gin.H)
	params, ok := op["parameters"].([]gin.H)
	if !ok || len(params) != 3 {
		t.Fatalf("se esperaban 3 parámetros de ruta, se obtuvo %v", op["parameters"])
	}
	want := map[string]bool{"cropId": true, "type": true, "recipientId": true}
	for _, p := range params {
		name := p["name"].(string)
		if !want[name] {
			t.Errorf("parámetro inesperado %q", name)
		}
		if p["in"] != "path" || p["required"] != true {
			t.Errorf("parámetro %q mal definido: %v", name, p)
		}
	}
}

// El documento debe ser JSON válido (lo consume Swagger UI).
func TestDocumentIsValidJSON(t *testing.T) {
	if _, err := json.Marshal(Document("http://x")); err != nil {
		t.Fatalf("el documento no serializa a JSON: %v", err)
	}
}
