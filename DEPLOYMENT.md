# API Gateway 部署指南

本文档详细介绍了如何部署和运行API网关系统。

## 前置要求

### 系统要求
- Go 1.21 或更高版本
- Docker 和 Docker Compose（可选，用于容器化部署）
- Redis（可选，用于缓存和速率限制）

### 依赖工具
```bash
# 安装开发工具
make install-tools

# 检查环境
make check-env
```

## 快速开始

### 1. 克隆项目
```bash
git clone <repository-url>
cd API-Gateway
```

### 2. 安装依赖
```bash
make deps
```

### 3. 配置文件
复制并修改配置文件：
```bash
cp configs/config.yaml configs/config.local.yaml
```

### 4. 构建并运行
```bash
# 开发模式运行
make dev

# 或者构建后运行
make run
```

### 5. 验证部署
```bash
# 运行演示脚本
make demo
```

## 部署方式

### 方式一：本地部署

#### 1. 准备环境
```bash
# 安装Redis（如果需要）
# Ubuntu/Debian
sudo apt-get install redis-server

# macOS
brew install redis

# 启动Redis
redis-server
```

#### 2. 配置应用
编辑 `configs/config.yaml`：
```yaml
server:
  host: "0.0.0.0"
  port: 8080

redis:
  addr: "localhost:6379"
  password: ""
  db: 0

auth:
  jwt_secret: "your-production-secret-key"
```

#### 3. 启动应用
```bash
# 构建
make build

# 运行
./build/gateway -config configs/config.yaml
```

### 方式二：Docker部署

#### 1. 构建镜像
```bash
make docker-build
```

#### 2. 运行容器
```bash
# 单独运行网关
make docker-run

# 或使用Docker Compose运行完整环境
make compose-up
```

#### 3. 查看状态
```bash
# 查看日志
make compose-logs

# 查看容器状态
docker-compose ps
```

### 方式三：生产环境部署

#### 1. 准备生产配置
```yaml
# configs/production.yaml
server:
  host: "0.0.0.0"
  port: 8080
  tls:
    enabled: true
    cert_file: "/etc/ssl/certs/gateway.crt"
    key_file: "/etc/ssl/private/gateway.key"

redis:
  addr: "redis-cluster:6379"
  password: "production-password"
  pool_size: 50

auth:
  jwt_secret: "your-very-secure-production-key"
  token_expiry: 1h
  refresh_expiry: 24h

logging:
  level: "warn"
  output: "/var/log/gateway/gateway.log"

routes:
  - path: "/api/v1/users"
    backends:
      - url: "https://users-service.internal:443"
        weight: 1
    auth_required: true
    rate_limit: 1000
    load_balancer: "least_conn"
```

#### 2. 使用systemd管理服务
创建服务文件 `/etc/systemd/system/api-gateway.service`：
```ini
[Unit]
Description=API Gateway
After=network.target

[Service]
Type=simple
User=gateway
Group=gateway
WorkingDirectory=/opt/api-gateway
ExecStart=/opt/api-gateway/gateway -config /opt/api-gateway/configs/production.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=api-gateway

[Install]
WantedBy=multi-user.target
```

启动服务：
```bash
sudo systemctl daemon-reload
sudo systemctl enable api-gateway
sudo systemctl start api-gateway
```

#### 3. 配置反向代理（Nginx）
```nginx
upstream api_gateway {
    server 127.0.0.1:8080;
    # 可以添加多个实例实现高可用
    # server 127.0.0.1:8081;
}

server {
    listen 80;
    listen 443 ssl http2;
    server_name api.example.com;

    ssl_certificate /etc/ssl/certs/api.example.com.crt;
    ssl_certificate_key /etc/ssl/private/api.example.com.key;

    location / {
        proxy_pass http://api_gateway;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # 超时设置
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
        
        # 缓冲设置
        proxy_buffering on;
        proxy_buffer_size 4k;
        proxy_buffers 8 4k;
    }

    # 健康检查端点
    location /health {
        proxy_pass http://api_gateway/health;
        access_log off;
    }
}
```

## 监控配置

### Prometheus监控
1. 确保指标端点启用：
```yaml
metrics:
  enabled: true
  path: "/metrics"
  port: 9090
```

2. 配置Prometheus抓取：
```yaml
scrape_configs:
  - job_name: 'api-gateway'
    static_configs:
      - targets: ['localhost:9090']
    scrape_interval: 15s
```

### Grafana仪表板
1. 导入预配置的仪表板
2. 配置数据源指向Prometheus
3. 设置告警规则

### 日志管理
```yaml
logging:
  level: "info"
  format: "json"
  output: "/var/log/gateway/gateway.log"
```

配置日志轮转：
```bash
# /etc/logrotate.d/api-gateway
/var/log/gateway/*.log {
    daily
    missingok
    rotate 52
    compress
    delaycompress
    notifempty
    create 644 gateway gateway
    postrotate
        systemctl reload api-gateway
    endscript
}
```

## 性能优化

### 1. 系统级优化
```bash
# 增加文件描述符限制
echo "* soft nofile 65536" >> /etc/security/limits.conf
echo "* hard nofile 65536" >> /etc/security/limits.conf

# 优化网络参数
echo "net.core.somaxconn = 65535" >> /etc/sysctl.conf
echo "net.ipv4.tcp_max_syn_backlog = 65535" >> /etc/sysctl.conf
sysctl -p
```

### 2. 应用级优化
```yaml
server:
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 60s

redis:
  pool_size: 100
  min_idle_conns: 10
```

### 3. 容器优化
```dockerfile
# 优化后的Dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o gateway cmd/gateway/main.go

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/gateway /gateway
COPY --from=builder /app/configs /configs
EXPOSE 8080 9090
CMD ["/gateway"]
```

## 安全配置

### 1. TLS配置
```yaml
server:
  tls:
    enabled: true
    cert_file: "/etc/ssl/certs/server.crt"
    key_file: "/etc/ssl/private/server.key"
```

### 2. 安全头部
系统自动添加以下安全头部：
- Content-Security-Policy
- X-Frame-Options
- X-Content-Type-Options
- Strict-Transport-Security

### 3. 速率限制
```yaml
routes:
  - path: "/api/v1/public"
    rate_limit: 100  # 每分钟100次请求
```

## 故障排查

### 常见问题

1. **连接Redis失败**
```bash
# 检查Redis连接
redis-cli ping

# 检查网络连接
telnet redis-host 6379
```

2. **后端服务不可用**
```bash
# 检查后端健康状态
curl http://localhost:8080/admin/backends

# 手动更新后端状态
curl -X POST http://localhost:8080/admin/backends/health \
  -H "Content-Type: application/json" \
  -d '{"backend":"http://service:8080","healthy":true}'
```

3. **性能问题**
```bash
# 查看指标
curl http://localhost:9090/metrics

# 查看系统状态
curl http://localhost:8080/admin/status
```

### 日志分析
```bash
# 查看错误日志
grep "ERROR" /var/log/gateway/gateway.log

# 查看慢请求
grep "duration" /var/log/gateway/gateway.log | grep -E "duration.*[5-9]\d{3}ms"

# 实时监控
tail -f /var/log/gateway/gateway.log | grep ERROR
```

## 扩展和定制

### 添加自定义中间件
1. 实现 `Middleware` 接口
2. 在网关初始化时注册
3. 在路由配置中启用

### 添加新的负载均衡算法
1. 实现 `LoadBalancer` 接口
2. 在 `CreateLoadBalancer` 函数中添加
3. 在配置中使用新算法

### 集成新的缓存后端
1. 实现 `Cache` 接口
2. 在网关初始化时选择缓存实现

## 备份和恢复

### 配置备份
```bash
# 备份配置文件
tar -czf gateway-config-$(date +%Y%m%d).tar.gz configs/

# 备份日志
tar -czf gateway-logs-$(date +%Y%m%d).tar.gz logs/
```

### 数据恢复
```bash
# 恢复配置
tar -xzf gateway-config-20240101.tar.gz

# 重启服务
systemctl restart api-gateway
```

## 升级指南

### 滚动升级
1. 准备新版本
2. 更新配置文件
3. 逐个替换实例
4. 验证功能正常

### 回滚流程
1. 停止新版本服务
2. 恢复旧版本配置
3. 启动旧版本服务
4. 验证服务正常

通过以上配置和部署指南，您可以在各种环境中成功部署和运行API网关系统。
