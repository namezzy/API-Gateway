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
```yaml
server:
  host: 0.0.0.0
  port: 8080
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 60s
redis:
  addr: localhost:6379
auth:
  jwt_secret: "change-me-in-prod"
  token_expiry: 24h
  refresh_expiry: 168h
routes:
  - path: /api/v1/users
    method: GET
    backends:
      - url: http://localhost:3001
        weight: 2
      - url: http://localhost:3002
        weight: 1
    auth_required: true
    rate_limit: 100
    cache_enabled: true
    cache_ttl: 5m
    load_balancer: weighted_round
```

### 3. 启动
```bash
go run ./cmd/gateway
# 或使用配置文件
go run ./cmd/gateway -config configs/config.yaml
```

### 4. 关键端点
| 端点 | 描述 |
|------|------|
| GET /health | 基础健康检查 |
| GET /health/detailed | 系统与后端依赖详细状态 |
| POST /auth/login | 登录，返回 access / refresh token |
| POST /auth/refresh | 刷新访问令牌 |
| POST /auth/logout | 登出 (演示版) |
| GET /admin/status | 网关状态（需认证） |
| GET /admin/backends | 后端健康及连接情况（需认证） |
| GET /metrics | Prometheus 指标 |
| (Prometheus) /api/v1/query | PromQL 查询（前端直接调用 9091） |

### 5. 简单测试
```bash
curl -s localhost:8080/health | jq
curl -s -X POST localhost:8080/auth/login -d '{"username":"admin","password":"password123"}' -H 'Content-Type: application/json'
```

---

## 🧩 项目结构
```
├── cmd/
│   └── gateway/              # 主入口 (main.go)
├── internal/
│   ├── auth/                 # JWT & 模拟用户服务
│   ├── cache/                # Redis & 内存缓存
│   ├── config/               # YAML 配置解析 + 默认值
│   ├── gateway/              # 核心网关组合逻辑
│   ├── healthcheck/          # 后端与系统依赖健康检查
│   ├── loadbalancer/         # 多种负载均衡算法
│   ├── logger/               # 日志抽象
│   ├── metrics/              # 指标封装与记录
│   ├── middleware/           # 可注册中间件集合
│   └── ratelimit/            # 多策略速率限制实现
├── configs/                  # 配置文件
├── monitoring/               # Prometheus / Grafana 配置
├── scripts/                  # 演示脚本
├── mock-backends/            # 模拟后端资源 (可扩展)
├── Dockerfile                # 容器构建
├── docker-compose.yml        # 本地编排 (可含 Redis / 后端 / Grafana / Prometheus)
├── frontend/                 # React + Vite 前端面板
└── Makefile                  # 常用任务 (build / run / lint / test)
```

---

## 🔐 认证与授权 & 自动刷新
1. 登录: `POST /auth/login` 返回 `access_token` 与 `refresh_token`
2. 访问受保护 API:
   ```http
   Authorization: Bearer <access_token>
   ```
3. 刷新令牌: `POST /auth/refresh`
4. 角色策略: Claims 中 `roles` 可用于网关扩展 RBAC
5. 自动刷新: 前端解析 access token 的 `exp`，在到期前 60s 调用 `/auth/refresh` 获取新 access，失败则清除登录状态（详见 `frontend/src/context/AuthContext.tsx`）

刷新流程：
```
login -> 保存 { access, refresh, expiresAt }
      ↓ 定时器 (exp - 60s)
    refresh (保持 refresh 不变) -> 更新 access + expiresAt
```

---

## ⚖️ 负载均衡策略
| 策略 | 适用场景 | 说明 |
|------|----------|------|
| round_robin | 后端性能均衡 | 顺序轮询 |
| weighted_round | 不同权重 | 动态权重调节流量比例 |
| least_conn | 连接差异大 | 优先空闲后端 |
| ip_hash | 会话粘性 | 同 IP 固定后端 |
| random | 轻量随机 | 均匀概率 |

---

## 🚦 速率限制
支持：
1. 全局中间件令牌桶 (默认)
2. 路由级速率覆盖 (配置 `rate_limit`)
3. 预留实现：滑动窗口 / 固定窗口 (接口已定义)

Key 维度：`clientIP + userID + path`

---

## 🧠 缓存策略
| 级别 | 存储 | 说明 |
|------|------|------|
| 内存 | 进程内 map | 低延迟，重启丢失 |
| Redis | 外部 | 跨实例共享，可 TTL 控制 |

缓存键: `prefix:METHOD:/path:query` 可按路由开启 `cache_enabled` 并设定 `cache_ttl`。

---

## 📊 指标 (Prometheus)
部分指标：
| 名称 | 含义 |
|------|------|
| http_requests_total | HTTP 请求计数 (method/path/status) |
| http_request_duration_seconds | 延迟直方图 |
| backend_requests_total | 后端调用计数 |
| backend_health_status | 后端健康 (0/1) |
| rate_limit_requests_total | 速率限制允许/拒绝 |
| cache_requests_total | 缓存命中/未命中 |
| active_connections | 当前活跃连接 |
| auth_requests_total | 登录成功/失败 |
| backend_health_status | 后端健康状态 (0/1) |

---

## 🛡️ 安全
- CSP / HSTS / X-Frame-Options / X-Content-Type-Options 等头部
- JWT 签名与过期控制
- 速率限制抵御暴力请求
- 预留：IP 白名单 / ACL / mTLS / Web Application Firewall 接入

---

## 🛠️ 开发与扩展
### 添加中间件
```go
type MyMiddleware struct{}
func (m *MyMiddleware) Name() string { return "my" }
func (m *MyMiddleware) Handle() gin.HandlerFunc { return func(c *gin.Context){ /* ... */ } }
// 注册: gateway.middlewareManager.Register(&MyMiddleware{})
// 路由配置里 middleware: ["my"]
```

### 新增负载均衡算法
实现接口:
```go
type LoadBalancer interface {
  NextBackend(clientIP string) (*Backend, error)
  AddBackend(*Backend)
  RemoveBackend(url string)
  GetBackends() []*Backend
  UpdateBackendHealth(url string, healthy bool)
}
```

---

## 🧪 测试 & 运行
```bash
make build        # 构建
make run          # 运行主网关
make demo         # 可添加脚本演示
make test         # 运行测试(如后续补充)
```

---

## 🐳 Docker / Compose & 多阶段前端集成
`Dockerfile` 包含：
1. Go 编译阶段 (go-builder)
2. 前端构建阶段 (fe-builder) -> 生成 `frontend/dist`
3. 最终 alpine 镜像复制二进制与静态资源至 `/public`

网关启动后：访问 `http://<host>:8080/` 即加载前端 SPA（若存在）。

如仅需后端，可删除前端阶段。
```bash
docker build -t api-gateway:latest .
docker run -p 8080:8080 -p 9090:9090 api-gateway:latest
# 或
docker compose up -d
```

---

## 🌐 前端 Dashboard 功能摘要
路径：`frontend/` (详细见其 README)

| 功能 | 描述 |
|------|------|
| 登录 / 退出 | JWT + refresh 自动续期 |
| 后端服务列表 | 权重 / 健康 / 连接数展示 |
| 网关概览 | 核心状态、缓存/速率统计（可扩展） |
| 指标趋势 | 基于 /metrics 文本简易解析 + Recharts 绘图 |
| PromQL 查询 | 直接调用 Prometheus HTTP API 执行即时查询 |
| 主题切换 | Light/Dark + 品牌主色/次色自定义弹窗 |
| 全局通知 | Axios 拦截 + Snackbar 统一提示 |
| Token 刷新 | 提前 60s 自动刷新 access token |

## ⚙️ CI (GitHub Actions)
Workflow: `.github/workflows/ci.yml`

Jobs：
| Job | 内容 |
|-----|------|
| build-test | Go 依赖、测试、覆盖率 artifact |
| lint | golangci-lint 静态检查 |
| security | go vet + govulncheck |
| frontend | Node 20 安装依赖、ESLint、Vite build、上传 dist |

可扩展：SAST、镜像扫描、依赖缓存、版本发布。

## 🔍 Roadmap (可演进)
- [ ] OpenAPI / 自动文档
- [ ] 分布式追踪 (OpenTelemetry)
- [ ] 动态配置热更新 (etcd / Consul)
- [ ] 高级认证(OAuth2 / OIDC / API Key / HMAC)
- [ ] 请求重试 + 断路器 + 熔断 (Resilience)
- [ ] Canary / 灰度发布策略
- [ ] WebSocket / GRPC 代理支持
- [ ] 租户隔离 / 限额配额

---

## 🤝 贡献
欢迎 PR / Issue：
1. Fork & 创建分支
2. 遵循模块化与单一职责
3. 添加/更新必要注释与文档
4. 通过现有构建与测试

---

## 📄 许可证
MIT License — 自由用于商业与个人项目，保留版权声明。

---

## 💬 支持
如需企业级增强（动态路由中心 / 服务注册发现 / 限流分布式令牌 / 插件体系）可进一步拓展，本仓库为基础骨架示例。

> 打造属于你的可观测、高扩展、可维护 API 流量入口 ✨
