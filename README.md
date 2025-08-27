<div align="center">
  <h1>API Gateway (Go)</h1>
  <p>Production‑ready, modular, extensible API gateway: Routing · Load Balancing · Auth · Rate Limiting · Caching · Observability · Security</p>
  <p><strong>Go 1.21+</strong> · Pluggable Middleware · Graceful Shutdown · Prometheus Metrics · JWT · Multiple LB Algorithms</p>
  <p>English | <a href="README.zh.md">中文</a></p>
</div>

---

## Features

| Domain | Capability | Notes |
|--------|-----------|-------|
| Routing | Path / Method / Group | Prefix based, group middleware |
| Load Balancing | round_robin / weighted_round / least_conn / ip_hash / random | Per‑route configuration |
| Auth | JWT + roles | Login / refresh / logout demo |
| Rate Limiting | Token bucket (+ future window strategies) | Per route override |
| Cache | In‑Memory / Redis | Route‑level enable + TTL |
| Health | Backend + system deps | Periodic probes |
| Metrics | Prometheus | HTTP / Backend / Cache / Rate / Auth / System |
| Security | Headers / CORS / Limits | CSP / HSTS / Frame / Content-Type |
| Logging | Structured JSON | Logrus abstraction |
| Ops | Graceful shutdown | Context lifecycle |
| Frontend | React Dashboard | Auth / Backends / Metrics / PromQL / Theme |

> Added: automatic token refresh, PromQL querying, theme brand colors, multi-stage docker build bundling SPA, CI frontend job.

---

## Quick Start

```bash
git clone https://github.com/your-org/api-gateway.git
cd api-gateway
go mod tidy
go run ./cmd/gateway -config configs/config.yaml
```

Health check:
```bash
curl -s localhost:8080/health
```

Login:
```bash
curl -s -X POST localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"password123"}'
```

Metrics: `GET /metrics`

---

## Auth & Token Refresh

1. Login: `POST /auth/login` => access + refresh
2. Protected routes: `Authorization: Bearer <access>`
3. Refresh: `POST /auth/refresh`
4. Roles: claims.roles for RBAC extensions
5. Frontend decodes `exp` and schedules refresh 60s before expiry.

Flow:
```
login -> store { access, refresh, expiresAt }
      ↓ timer (exp - 60s)
   /auth/refresh -> update access (refresh unchanged)
```

---

## Project Layout
```
internal/
  auth/       # JWT + mock users
  cache/      # Redis & memory cache
  config/     # YAML config parsing
  gateway/    # Core orchestration
  healthcheck/# Backend & system health
  loadbalancer# Algorithms
  logger/     # Logging
  metrics/    # Prometheus metrics
  middleware/ # Middlewares
  ratelimit/  # Limiter strategies
cmd/gateway   # Entry point
frontend/     # React + Vite dashboard
```

---

## Load Balancing

| Strategy | Use Case | Note |
|----------|----------|------|
| round_robin | Even backends | Sequential rotation |
| weighted_round | Uneven capacity | Proportional weights |
| least_conn | Varied concurrency | Choose least active |
| ip_hash | Session stickiness | Deterministic by IP |
| random | Simple distribution | Uniform random |

---

## Rate Limiting

Token bucket global + per route override (future: sliding / fixed windows). Key: `clientIP + userID + path`.

---

## Caching

| Level | Store | Note |
|-------|-------|------|
| Memory | in‑process | Fast, volatile |
| Redis | external | Shared, TTL |

Key pattern: `prefix:METHOD:/path:query`.

---

## Metrics (Prometheus)

Examples: `http_requests_total`, `http_request_duration_seconds`, `backend_requests_total`, `backend_health_status`, `rate_limit_requests_total`, `cache_requests_total`, `active_connections`, `auth_requests_total`.

---

## Security

HSTS, CSP, X-Frame-Options, X-Content-Type-Options, JWT expiry, rate limiting; extensible for mTLS / ACL / WAF.

---

## Extending

Middleware skeleton:
```go
type MyMw struct{}
func (m *MyMw) Name() string { return "my" }
func (m *MyMw) Handle() gin.HandlerFunc { return func(c *gin.Context){ c.Next() } }
```
Implement `LoadBalancer` for new algorithms.

---

## Docker / Compose & Multi‑stage Frontend

Dockerfile stages:
1. Go build (go-builder)
2. Frontend build (fe-builder, Vite)
3. Final Alpine with binary + `/public` SPA (served with fallback).

Remove stage 2 if API‑only.

```bash
docker build -t api-gateway:latest .
docker run -p 8080:8080 -p 9090:9090 api-gateway:latest
# or
docker compose up -d
```

---

## Frontend Dashboard

| Feature | Description |
|---------|-------------|
| Auth / Refresh | JWT + auto renew (‑60s) |
| Backends | Weight / health / connections |
| Metrics Trend | Parse /metrics -> charts |
| PromQL Query | Direct Prometheus HTTP API |
| Theme | Dark/Light + brand colors picker |
| Notifications | Axios + Snackbar queue |

---

## CI (GitHub Actions)

Jobs: build-test (Go), lint, security (govulncheck), frontend (ESLint + build + dist artifact).

---

## Roadmap

- OpenTelemetry tracing
- Dynamic config (etcd / Consul)
- Circuit breaking / retry
- Canary routing
- WebSocket / gRPC proxy
- Advanced auth (OIDC / API Key / HMAC)

---

## License

MIT

---

## Contributing

PRs welcome – keep changes modular, documented, tested.

---

## Support

For enterprise add‑ons (dynamic routing center, service discovery, distributed rate limiting, plugin system) extend this baseline.
