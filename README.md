# 🏦 FinTech Platform — Local Microservices (Go + Chi)

A fully local microservices-based fintech platform built with Go, Chi, Kafka, PostgreSQL, and Docker Compose.

---

## 🗂️ Project Structure

```
fintech-platform/
├── docker-compose.yml          # Spins up everything
├── Makefile                    # Convenience commands
├── nginx/
│   └── nginx.conf              # API Gateway routing
├── services/
│   └── user-service/           # ✅ Built — Auth + User profile
│       ├── cmd/server/main.go  # Entrypoint
│       ├── config/             # Env config loader
│       ├── internal/
│       │   ├── handler/        # HTTP handlers (Chi)
│       │   ├── middleware/     # JWT auth middleware
│       │   ├── model/          # Domain models
│       │   ├── repository/     # DB layer (pgx)
│       │   └── service/        # Business logic
│       ├── Dockerfile
│       └── .env
└── infra/
    ├── prometheus/             # Metrics config
    └── grafana/                # Dashboards
```

---

## 🚀 Quick Start

```bash
# 1. Clone / enter the project
cd fintech-platform

# 2. Start everything
make up

# 3. Check it's running
curl http://localhost:8080/health
```

---

## 📡 API Endpoints

All requests go through Nginx at **http://localhost:8080**

### Auth
| Method | Path | Body | Description |
|--------|------|------|-------------|
| POST | `/api/v1/auth/register` | `{email, password, full_name}` | Register a new user |
| POST | `/api/v1/auth/login` | `{email, password}` | Login, receive JWT |

### Users (🔒 Protected — add `Authorization: Bearer <token>`)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/users/me` | Get your profile |
| GET | `/api/v1/users/health` | Service health check |

---

## 🧪 Test the API

```bash
# Register
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"password123","full_name":"Alice Smith"}'

# Login
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"password123"}'

# Get profile (replace TOKEN with JWT from login)
curl http://localhost:8080/api/v1/users/me \
  -H "Authorization: Bearer TOKEN"
```

---

## 🖥️ Local UIs

| UI | URL | Credentials |
|----|-----|-------------|
| Kafdrop (Kafka) | http://localhost:9000 | — |
| Grafana | http://localhost:3000 | admin / admin |
| Mailhog | http://localhost:8025 | — |
| Jaeger Tracing | http://localhost:16686 | — |
| MinIO | http://localhost:9001 | minioadmin / minioadmin |
| Prometheus | http://localhost:9090 | — |

---

## 🛠️ Makefile Commands

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

## 🗺️ Services Roadmap

- [x] **API Gateway** (Nginx)
- [x] **User Service** (Go + Chi + JWT + PostgreSQL)
- [ ] **Account Service** — balances, account management
- [ ] **Transaction Service** — payments, Kafka producer
- [ ] **Fraud Detection Service** — consumes Kafka, rule-based scoring
- [ ] **Notification Service** — emails via Mailhog
- [ ] **Analytics Service** — spending summaries
- [ ] **Audit Service** — immutable logs to MinIO
- [ ] **FX Rate Service** — mock currency conversion

---

## 🏗️ Architecture

```
Client
  │
  ▼
Nginx (API Gateway :8080)
  │
  ├──► user-service:3001
  ├──► account-service:3002  (coming soon)
  └──► transaction-service:3003  (coming soon)
             │
             ▼
           Kafka
             │
      ┌──────┴──────┐
      ▼             ▼
  fraud-service  notification-service
```

Each service has its own PostgreSQL database (database-per-service pattern).
