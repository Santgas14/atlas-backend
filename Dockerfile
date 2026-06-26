# ─── Build stage ──────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app
COPY go.mod ./
RUN go mod download || true
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /atlab-backend ./cmd/server

# ─── Runtime stage ────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata ansible openssh-client

WORKDIR /app
COPY --from=builder /atlab-backend .
COPY migrations/ ./migrations/

EXPOSE 8080

ENTRYPOINT ["./atlab-backend"]
