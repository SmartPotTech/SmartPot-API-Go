# Etapa de compilación: aquí se arma el binario estático.
FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

# Certificados para copiarlos a la imagen final (scratch no trae ninguno).
RUN apk add --no-cache ca-certificates

COPY . .
# CGO desactivado + tzdata incrustada en el binario: no depende de libc ni de
# /usr/share/zoneinfo, así la imagen de ejecución puede ser "scratch".
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -tags timetzdata -ldflags="-s -w" -o /out/smartpot-api ./cmd/api

# Etapa de ejecución: scratch, sin shell, sin gestor de paquetes y sin
# herramientas de red. Un contenedor comprometido no encuentra utilidades que
# aprovechar ni forma de escalar.
FROM scratch

LABEL org.opencontainers.image.title="SmartPot API" \
      org.opencontainers.image.description="API REST y MQTT de SmartPot (Go)" \
      org.opencontainers.image.source="https://github.com/SmartPotTech/SmartPot-API-Go" \
      org.opencontainers.image.licenses="MIT"

# Solo el bundle de certificados CA (para las conexiones TLS salientes) y el binario.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/smartpot-api /smartpot-api

ENV PORT=8091 \
    TZ=UTC

# Usuario no root (numérico: scratch no tiene /etc/passwd).
USER 1000:1000

EXPOSE 8091

# El propio binario hace el chequeo; no hace falta wget ni curl en la imagen.
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD ["/smartpot-api", "healthcheck"]

ENTRYPOINT ["/smartpot-api"]
