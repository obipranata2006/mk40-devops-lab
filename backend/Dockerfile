# ===== STAGE 1: BUILDER =====
FROM golang:1.22-alpine AS builder
WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /build/app-server main.go

# ===== STAGE 2: PRODUCTION =====
FROM alpine:3.20 AS production

RUN apk --no-cache add ca-certificates wget && \
    addgroup -S appgroup && \
    adduser -S -u 10001 -G appgroup appuser

WORKDIR /app
COPY --from=builder --chown=appuser:appgroup /build/app-server /app/app-server

USER appuser
EXPOSE 8080
ENTRYPOINT ["/app/app-server"]
