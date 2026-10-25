package main

import (
	"net/http"
	"os"
	"time"
)

// healthcheck consulta /health en el propio proceso y devuelve el código de
// salida para el HEALTHCHECK de Docker. Vive dentro del binario a propósito: así
// la imagen de ejecución no necesita wget, curl ni ninguna herramienta de red,
// y un contenedor comprometido no encuentra utilidades que aprovechar.
func healthcheck() int {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8091"
	}
	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/health")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return 0
	}
	return 1
}
