<div align="center">
  <h1>API Gateway (Go)</h1>
  <p>Production‑ready, modular, extensible API gateway: Routing · Load Balancing · Auth · Rate Limiting · Caching · Observability · Security</p>
  <p><strong>Go 1.21+</strong> · Pluggable Middleware · Graceful Shutdown · Prometheus Metrics · JWT · Multiple LB Algorithms</p>
</div>

---

## Features

| Domain | Capability | Notes |
|--------|-----------|-------|
| Routing | Path / Method / Group | Prefix based, group middleware |
| Load Balancing | round_robin / weighted_round / least_conn / ip_hash / random | Per‑route configuration |
| Auth | JWT + roles | Login / refresh / logout demo |
| Rate Limiting | Token bucket (+ window strategies scaffold) | Per route override |
| Cache | In‑Memory / Redis | Route‑level enable + TTL |
| Health | Backend + system deps | Periodic probes |
| Metrics | Prometheus | HTTP / Backend / Cache / Rate / Auth / System |
| Security | Headers / CORS / Limits | CSP / HSTS / Frame / Content-Type |
| Logging | Structured JSON | Logrus abstraction |
| Ops | Graceful shutdown | Context‑driven lifecycle |

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

## OpenAPI
See `openapi/openapi.yaml` (placeholder). Extend as stable APIs evolve.

---

## Project Layout
```
internal/
  auth/       # JWT + mock users
  cache/      # Redis & memory cache
  config/     # YAML config parsing
  gateway/    # Core orchestration
  healthcheck/# Backend & system health
  loadbalancer# Multiple algorithms
  logger/     # Logging wrapper
  metrics/    # Prometheus metrics
  middleware/ # Pluggable middlewares
  ratelimit/  # Limiter strategies
cmd/gateway   # Entry point
configs/      # YAML configs
openapi/      # OpenAPI spec
scripts/      # Demo scripts
monitoring/   # Prometheus/Grafana
examples/     # Sample backend services
```

---

## Extending
Add a middleware:
```go
type MyMw struct{}
func (m *MyMw) Name() string { return "my" }
func (m *MyMw) Handle() gin.HandlerFunc { return func(c *gin.Context){ c.Next() } }
// Register and list in route middleware array.
```

Add a load balancer: implement the `LoadBalancer` interface then register via factory.

---

## Core Metrics
| Metric | Description |
|--------|-------------|
| http_requests_total | Labeled request count |
| http_request_duration_seconds | Latency histogram |
| backend_requests_total | Upstream calls |
| backend_health_status | 0/1 health gauge |
| rate_limit_requests_total | Allowed / denied |
| cache_requests_total | Hit / miss |
| active_connections | Current active connections |

---

## Roadmap
- OpenTelemetry tracing
- Dynamic config (etcd / Consul)
- Circuit breaking / retry policy
- Canary routing
- WebSocket / gRPC proxy
- Advanced auth (OIDC / API Key / HMAC)

---

## License
MIT

---

## Contributing
PRs welcome – keep changes modular and documented.
