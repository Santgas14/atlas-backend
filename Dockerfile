# ─── Build stage ──────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /atlab-backend ./cmd/server

# ─── Runtime stage ────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata ansible openssh-client

WORKDIR /app
COPY --from=builder /atlab-backend .
COPY migrations/ ./migrations/

EXPOSE 8080

ENTRYPOINT ["./atlab-backend"]
