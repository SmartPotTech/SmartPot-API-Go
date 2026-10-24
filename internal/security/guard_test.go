package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/httpserver"
	"github.com/gin-gonic/gin"
)

func TestGuard(t *testing.T) {
	svc := NewTokenService(testSecret, time.Hour)
	issued, _ := svc.Issue("user-123", "a@b.com", "USER")

	e := httpserver.New(discardLogger())
	grp := e.Group("/api/v1")
	grp.Use(svc.Guard())
	grp.GET("/me", func(c *gin.Context) { c.String(http.StatusOK, CurrentUserID(c)) })

	send := func(authHeader string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		e.ServeHTTP(rec, req)
		return rec
	}

	if rec := send(""); rec.Code != http.StatusUnauthorized {
		t.Errorf("sin token: código %d, se esperaba 401", rec.Code)
	}
	if rec := send("Bearer no-es-un-token"); rec.Code != http.StatusUnauthorized {
		t.Errorf("token inválido: código %d, se esperaba 401", rec.Code)
	}
	rec := send("Bearer " + issued.Value)
	if rec.Code != http.StatusOK {
		t.Fatalf("token válido: código %d, se esperaba 200", rec.Code)
	}
	if rec.Body.String() != "user-123" {
		t.Errorf("CurrentUserID = %q, se esperaba user-123", rec.Body.String())
	}
}
