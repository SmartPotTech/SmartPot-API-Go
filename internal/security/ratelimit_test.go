package security

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/cache"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/httpserver"
	"github.com/gin-gonic/gin"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter(cache.NewStore(nil), 3, 2)
	fixed := time.Now()
	rl.now = func() time.Time { return fixed }

	e := httpserver.New(discardLogger())
	e.Use(rl.Middleware())
	e.GET("/api/v1/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	e.POST("/api/v1/auth/login", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	send := func(method, path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, nil)
		req.RemoteAddr = "203.0.113.5:4444"
		e.ServeHTTP(rec, req)
		return rec
	}

	// Cupo general = 3: la cuarta petición se rechaza.
	for i := 1; i <= 3; i++ {
		if rec := send(http.MethodGet, "/api/v1/ping"); rec.Code != http.StatusOK {
			t.Fatalf("petición %d: código %d, se esperaba 200", i, rec.Code)
		}
	}
	rec := send(http.MethodGet, "/api/v1/ping")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("la cuarta petición: código %d, se esperaba 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("falta la cabecera Retry-After")
	}
	if !strings.Contains(rec.Body.String(), "demasiadas solicitudes") {
		t.Errorf("cuerpo inesperado: %s", rec.Body.String())
	}

	// Cupo de autenticación = 2: la tercera se rechaza (llave independiente).
	for i := 1; i <= 2; i++ {
		if rec := send(http.MethodPost, "/api/v1/auth/login"); rec.Code != http.StatusOK {
			t.Fatalf("auth %d: código %d, se esperaba 200", i, rec.Code)
		}
	}
	if rec := send(http.MethodPost, "/api/v1/auth/login"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("tercera auth: código %d, se esperaba 429", rec.Code)
	}
}
