# ── Build stage ──────────────────────────────────────────────────────
FROM golang:1.26.1-alpine AS builder

WORKDIR /app

# Download deps first (cached layer)
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o user-service ./cmd/server

# ── Runtime stage ─────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/user-service .

EXPOSE 3001

CMD ["./user-service"]
