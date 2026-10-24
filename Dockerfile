FROM golang:1.23-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/smartpot-api ./cmd/api

FROM alpine:3.20

LABEL org.opencontainers.image.title="SmartPot API" \
      org.opencontainers.image.description="API REST y MQTT de SmartPot (Go)" \
      org.opencontainers.image.source="https://github.com/SmartPotTech/SmartPot-API-Go" \
      org.opencontainers.image.licenses="MIT"

RUN apk add --no-cache ca-certificates tzdata wget && adduser -D -u 1000 app

WORKDIR /app
COPY --from=build /out/smartpot-api /app/smartpot-api

ENV PORT=8091 \
    TZ=UTC

USER 1000:1000

EXPOSE 8091

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD wget -q -O /dev/null "http://127.0.0.1:${PORT}/health" || exit 1

ENTRYPOINT ["/app/smartpot-api"]
