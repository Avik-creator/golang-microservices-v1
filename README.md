# FinTech Platform — Local Microservices (Go + Chi)

A fully local microservices-based fintech platform built with Go, Chi, Kafka, PostgreSQL, and Docker Compose.

---

## Project Structure

```
fintech-platform/
├── docker-compose.yml              # Spins up everything
├── Makefile                        # Convenience commands
├── nginx/
│   └── nginx.conf                  # API Gateway routing + rate limiting
├── services/
│   ├── user-service/               # Auth + User profile (port 3001)
│   ├── account-service/            # Account management (port 3002)
│   ├── transaction-service/        # Payments + Kafka producer (port 3003)
│   ├── fraud-service/              # Kafka consumer, rule-based scoring (port 3004)
│   ├── notification-service/       # Kafka consumer, emails via Mailhog (port 3005)
│   └── audit-service/              # Kafka consumer, immutable logs to MinIO (port 3006)
│       ├── cmd/server/main.go      # Entrypoint
│       ├── config/                 # Env config loader
│       ├── internal/
│       │   ├── handler/            # HTTP handlers (Chi)
│       │   ├── middleware/         # JWT auth middleware
│       │   ├── model/              # Domain models
│       │   ├── repository/         # DB layer (pgx)
│       │   └── service/            # Business logic
│       └── Dockerfile
└── infra/
    ├── prometheus/                 # Metrics config
    └── grafana/                    # Dashboards
```

---

## Quick Start

```bash
# 1. Clone / enter the project
cd fintech-platform

# 2. Start everything
make up

# 3. Check it's running
curl http://localhost:8080/health
```

---

## API Endpoints

All requests go through Nginx at **http://localhost:8080**

### Auth (rate-limited: 10 req/s per IP, burst 20)
| Method | Path | Body | Description |
|--------|------|------|-------------|
| POST | `/api/v1/auth/register` | `{email, password, full_name}` | Register a new user |
| POST | `/api/v1/auth/login` | `{email, password}` | Login, receive JWT |

### Users (Protected — add `Authorization: Bearer <token>`)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/users/health` | Service health check (public) |
| GET | `/api/v1/users/me` | Get your profile |

### Accounts (Protected)
| Method | Path | Body | Description |
|--------|------|------|-------------|
| POST | `/api/v1/accounts` | `{account_type, currency}` | Create account (`savings` or `current`) |
| GET | `/api/v1/accounts` | — | List your accounts |
| GET | `/api/v1/accounts/{id}` | — | Get account by ID |

### Transactions (Protected)
| Method | Path | Body | Description |
|--------|------|------|-------------|
| POST | `/api/v1/transactions` | `{from_account_id, to_account_id, amount, currency, description, idempotency_key}` | Create a transfer |
| GET | `/api/v1/transactions?account_id={id}` | — | List transactions for an account |
| GET | `/api/v1/transactions/{id}` | — | Get transaction by ID |

> The `idempotency_key` field is required on every transaction. Submitting the same key twice returns the original result without double-charging.

---

## Test the API

```bash
# Register
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"password123","full_name":"Alice Smith"}'

# Login
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"password123"}'

# Create a savings account (replace TOKEN)
curl -X POST http://localhost:8080/api/v1/accounts \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{"account_type":"savings","currency":"USD"}'

# Transfer funds (replace TOKEN, FROM_ID, TO_ID)
curl -X POST http://localhost:8080/api/v1/transactions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "from_account_id": "FROM_ID",
    "to_account_id":   "TO_ID",
    "amount":          100.00,
    "currency":        "USD",
    "description":     "Rent",
    "idempotency_key": "unique-key-001"
  }'
```

---

## Architecture

```
Client
  │
  ▼
Nginx (API Gateway :8080)
  │  rate-limits /api/v1/auth/*
  ├──► user-service:3001          (JWT auth, user profiles)
  ├──► account-service:3002       (balances, account management)
  └──► transaction-service:3003   (payments, saga pattern)
             │
             │ publishes to Kafka
             ▼
     topic: transactions.events
             │
      ┌──────┼──────────────┐
      ▼      ▼              ▼
  fraud   notification   audit
  :3004    :3005          :3006
  (scores) (emails)    (MinIO logs)
      │
      │ publishes fraud.alerts
      ▼
  notification + audit
  also consume fraud.alerts
```

Each service has its own PostgreSQL database (database-per-service pattern).

### Transaction Saga

The transaction-service implements a simple saga for every payment:

1. Create transaction record (`pending`)
2. Debit sender via account-service internal API
3. Credit receiver via account-service internal API
4. Mark transaction `completed`
5. Publish `transaction.completed` event to Kafka

If step 3 fails, a compensating debit refunds the sender before marking the transaction `failed`.

### Kafka Topics

| Topic | Producer | Consumers |
|-------|----------|-----------|
| `transactions.events` | transaction-service | fraud-service, notification-service, audit-service |
| `fraud.alerts` | fraud-service | notification-service, audit-service |
| `fraud.dead-letter` | fraud-service | — (manual inspection) |
| `audit.dead-letter` | audit-service | — (manual inspection) |

---

## Local UIs

| UI | URL | Credentials |
|----|-----|-------------|
| Kafdrop (Kafka) | http://localhost:9000 | — |
| Grafana | http://localhost:3000 | admin / admin |
| Mailhog | http://localhost:8025 | — |
| Jaeger Tracing | http://localhost:16686 | — |
| MinIO | http://localhost:9001 | minioadmin / minioadmin |
| Prometheus | http://localhost:9090 | — |

---

## Environment Variables

Secrets are injected via docker-compose and can be overridden with a `.env` file at the project root:

| Variable | Default | Used by |
|----------|---------|---------|
| `JWT_SECRET` | `fintech-jwt-secret` | user-service, account-service, transaction-service |
| `INTERNAL_SECRET` | `fintech-internal-secret` | account-service (validates calls from transaction-service) |

---

## Makefile Commands

```bash
make up              # Start all services
make down            # Stop all services
make logs            # Follow all logs
make logs-svc svc=user-service  # Follow one service
make rebuild svc=user-service   # Rebuild one service
make reset           # Wipe everything (fresh start)
make db-user         # psql into user DB
make kafka-ui        # Open Kafdrop in browser
```

---

## Services Roadmap

- [x] **API Gateway** (Nginx + rate limiting)
- [x] **User Service** (Go + Chi + JWT + PostgreSQL)
- [x] **Account Service** (balances, account management)
- [x] **Transaction Service** (payments, saga pattern, Kafka producer)
- [x] **Fraud Detection Service** (Kafka consumer, rule-based scoring, dead-letter topic)
- [x] **Notification Service** (Kafka consumer, emails via Mailhog)
- [x] **Audit Service** (Kafka consumer, immutable logs to MinIO, dead-letter topic)
- [ ] **Analytics Service** — spending summaries
- [ ] **FX Rate Service** — mock currency conversion
