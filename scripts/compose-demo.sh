#!/usr/bin/env bash
set -euo pipefail

echo "[demo] 启动演示环境 (gateway + redis + mock backends + prometheus + grafana)"
docker compose up -d --build api-gateway redis backend1 backend2 backend3 prometheus grafana

echo "等待服务稳定..."
sleep 5

echo "[demo] 基础健康检查:"; curl -s http://localhost:8080/health | jq . 2>/dev/null || curl -s http://localhost:8080/health; echo
echo "[demo] 登录获取token:"; LOGIN=$(curl -s -X POST http://localhost:8080/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"password123"}'); echo "$LOGIN" | jq . 2>/dev/null || echo "$LOGIN"; TOKEN=$(echo "$LOGIN" | jq -r .access_token 2>/dev/null || true)
if [ -n "${TOKEN:-}" ] && [ "$TOKEN" != "null" ]; then
  echo "[demo] 访问受保护端点 /admin/status"; curl -s http://localhost:8080/admin/status -H "Authorization: Bearer $TOKEN" | jq . 2>/dev/null || curl -s http://localhost:8080/admin/status -H "Authorization: Bearer $TOKEN"; echo
fi
echo "[demo] 查看指标前10行:"; curl -s http://localhost:9090/metrics | head -10; echo
echo "Prometheus: http://localhost:9091  Grafana: http://localhost:3000 (admin/admin)"
