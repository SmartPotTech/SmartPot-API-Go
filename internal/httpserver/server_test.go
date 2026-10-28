package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/apperr"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/validation"
	"github.com/labstack/echo/v4"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type testPayload struct {
	Name string `json:"name"`
}

func (p *testPayload) Validate() validation.Fields {
	return validation.New().NotBlank("name", p.Name, "el nombre no puede estar vacío").Result()
}

func TestHTTPServer_NotFound(t *testing.T) {
	e := New(testLogger())
	req := httptest.NewRequest(http.MethodGet, "/ruta-inexistente", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("se esperaba status 404, se obtuvo %d", rec.Code)
	}
	var resp apperr.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("error deserializando respuesta JSON: %v", err)
	}
	if resp.Status != http.StatusNotFound {
		t.Errorf("status esperado 404, obtenido %d", resp.Status)
	}
	if resp.Message != "La ruta solicitada no existe" {
		t.Errorf("mensaje inesperado: %s", resp.Message)
	}
}

func TestHTTPServer_MethodNotAllowed(t *testing.T) {
	e := New(testLogger())
	e.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "pong")
	})

	req := httptest.NewRequest(http.MethodPost, "/ping", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("se esperaba status 405, se obtuvo %d", rec.Code)
	}
	var resp apperr.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("error deserializando respuesta JSON: %v", err)
	}
	if resp.Status != http.StatusMethodNotAllowed {
		t.Errorf("status esperado 405, obtenido %d", resp.Status)
	}
	if resp.Message != "Método no permitido para esta ruta" {
		t.Errorf("mensaje inesperado: %s", resp.Message)
	}
}

func TestHTTPServer_SecurityHeaders(t *testing.T) {
	e := New(testLogger())
	e.GET("/secure", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Errorf("Referrer-Policy incorrecta: %s", rec.Header().Get("Referrer-Policy"))
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("X-Frame-Options incorrecto: %s", rec.Header().Get("X-Frame-Options"))
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("X-Content-Type-Options incorrecto: %s", rec.Header().Get("X-Content-Type-Options"))
	}
}

func TestHTTPServer_Recovery(t *testing.T) {
	e := New(testLogger())
	e.GET("/panic", func(c echo.Context) error {
		panic("error inesperado")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("se esperaba status 500, se obtuvo %d", rec.Code)
	}
	var resp apperr.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("error deserializando respuesta JSON: %v", err)
	}
	if !strings.Contains(resp.Message, "error inesperado") {
		t.Errorf("mensaje inesperado: %s", resp.Message)
	}
}

func TestHTTPServer_Bind(t *testing.T) {
	e := New(testLogger())
	e.POST("/bind", func(c echo.Context) error {
		var payload testPayload
		if !Bind(c, &payload) {
			return nil
		}
		return c.JSON(http.StatusOK, payload)
	})

	// JSON inválido
	reqBadJSON := httptest.NewRequest(http.MethodPost, "/bind", bytes.NewBufferString("{invalid"))
	recBadJSON := httptest.NewRecorder()
	e.ServeHTTP(recBadJSON, reqBadJSON)
	if recBadJSON.Code != http.StatusBadRequest {
		t.Fatalf("se esperaba status 400 para JSON inválido, se obtuvo %d", recBadJSON.Code)
	}

	// Validación fallida
	reqInvalid := httptest.NewRequest(http.MethodPost, "/bind", bytes.NewBufferString(`{"name":""}`))
	recInvalid := httptest.NewRecorder()
	e.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("se esperaba status 400 para validación fallida, se obtuvo %d", recInvalid.Code)
	}
	var valResp apperr.Response
	_ = json.Unmarshal(recInvalid.Body.Bytes(), &valResp)
	if valResp.Fields["name"] == "" {
		t.Errorf("se esperaba campo 'name' en errores de validación")
	}

	// Éxito
	reqValid := httptest.NewRequest(http.MethodPost, "/bind", bytes.NewBufferString(`{"name":"test"}`))
	recValid := httptest.NewRecorder()
	e.ServeHTTP(recValid, reqValid)
	if recValid.Code != http.StatusOK {
		t.Fatalf("se esperaba status 200, se obtuvo %d", recValid.Code)
	}
}
