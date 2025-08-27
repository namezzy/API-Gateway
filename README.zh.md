<div align="center">
  <h1>API Gateway （Go 高性能网关）</h1>
  <p>生产级、模块化、可扩展的 API 网关：路由 | 负载均衡 | 认证 | 速率限制 | 缓存 | 监控 | 安全</p>
  <p><strong>Go 1.21+</strong> · 可插拔中间件 · 优雅关停 · Prometheus 指标 · JWT 授权 · 多算法负载均衡</p>
  <p><a href="README.md">English Version</a> | 中文版</p>
</div>

---

## ✨ 核心能力概览

| 领域 | 能力 | 说明 |
|------|------|------|
| 路由 | Path / Method / Group | 基于前缀与通配处理，支持分组中间件 |
| 负载均衡 | round_robin / weighted_round / least_conn / ip_hash / random | 可按路由独立配置 |
| 认证授权 | JWT + 角色 | 登录/刷新/登出，角色信息写入 Claims |
| 速率限制 | 令牌桶 + 预留滑动/固定窗口 | 支持路由级覆盖，全局中间件默认限制 |
| 缓存 | 内存 / Redis | 路由级可选缓存，Cache-Control 友好 |
| 健康检查 | 后端 + 系统依赖 | 定期探测 /admin/backends 查看状态 |
| 监控 | Prometheus 指标 | HTTP/Backend/Cache/Rate/Auth/System 统一指标体系 |
| 安全 | 安全头部 / CORS / 限制 | HSTS / CSP / X-Frame / X-Content-Type |
| 观测 | 结构化日志 | Logrus JSON，可扩展收集链路ID |
| 可运维性 | 优雅关停 / 配置解耦 | 支持 context 关闭、YAML 配置化 |
| 前端控制台 | React Dashboard | 登录 / 后端状态 / 指标曲线 / PromQL / 主题自定义 |

> 新增：令牌自动刷新、前端多阶段打包、PromQL 查询、主题品牌色动态设置、CI 前端构建。

---

## 🚀 快速上手

```bash
git clone https://github.com/your-org/api-gateway.git
cd api-gateway
go mod tidy   # 如网络或 TLS 受限，可使用自建 GOPROXY
go run ./cmd/gateway -config configs/config.yaml
```

### 关键端点
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

### 简单测试
```bash
curl -s localhost:8080/health
curl -s -X POST localhost:8080/auth/login -d '{"username":"admin","password":"password123"}' -H 'Content-Type: application/json'
```

---

## 🧩 项目结构
```
├── cmd/gateway/              # 主入口 (main.go)
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
├── frontend/                 # React + Vite 前端面板
├── monitoring/               # Prometheus / Grafana 配置
├── scripts/                  # 演示脚本
├── mock-backends/            # 模拟后端资源
├── Dockerfile                # 容器构建
├── docker-compose.yml        # 本地编排
└── Makefile                  # 常用任务
```

---

## 🔐 认证与授权 & 自动刷新

1. 登录: `POST /auth/login` 返回 `access_token` 与 `refresh_token`
2. 访问受保护 API: `Authorization: Bearer <access_token>`
3. 刷新令牌: `POST /auth/refresh`
4. 角色策略: Claims 中 `roles` 可用于网关扩展 RBAC
5. 自动刷新: 前端解析 access token 的 `exp`，在到期前 60s 调用 `/auth/refresh` 获取新 access，失败则清除登录状态

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

## 🐳 Docker / Compose & 多阶段前端集成

`Dockerfile` 包含：
1. Go 编译阶段 (go-builder)
2. 前端构建阶段 (fe-builder) -> 生成 `frontend/dist`
3. 最终 alpine 镜像复制二进制与静态资源至 `/public`

网关启动后：访问 `http://<host>:8080/` 即加载前端 SPA（若存在）。

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

---

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

---

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
