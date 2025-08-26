#!/bin/bash

# API Gateway 演示脚本

echo "=== API Gateway 演示 ==="

# 检查是否安装了必要的工具
command -v curl >/dev/null 2>&1 || { echo "curl 未安装，请先安装 curl"; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "jq 未安装，建议安装 jq 以获得更好的JSON格式化输出"; }

GATEWAY_URL="http://localhost:8080"
METRICS_URL="http://localhost:9090"

echo "网关地址: $GATEWAY_URL"
echo "指标地址: $METRICS_URL"
echo ""

# 检查网关健康状态
echo "1. 检查网关健康状态..."
curl -s "$GATEWAY_URL/health" | jq . 2>/dev/null || curl -s "$GATEWAY_URL/health"
echo -e "\n"

# 检查详细健康状态
echo "2. 检查详细健康状态..."
curl -s "$GATEWAY_URL/health/detailed" | jq . 2>/dev/null || curl -s "$GATEWAY_URL/health/detailed"
echo -e "\n"

# 用户登录
echo "3. 用户登录..."
LOGIN_RESPONSE=$(curl -s -X POST "$GATEWAY_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"password123"}')

echo "$LOGIN_RESPONSE" | jq . 2>/dev/null || echo "$LOGIN_RESPONSE"

# 提取访问令牌
ACCESS_TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.access_token' 2>/dev/null)
if [ "$ACCESS_TOKEN" = "null" ] || [ -z "$ACCESS_TOKEN" ]; then
    echo "登录失败，无法获取访问令牌"
    exit 1
fi

echo "访问令牌: ${ACCESS_TOKEN:0:50}..."
echo ""

# 访问需要认证的管理端点
echo "4. 访问管理端点（需要认证）..."
curl -s "$GATEWAY_URL/admin/status" \
  -H "Authorization: Bearer $ACCESS_TOKEN" | jq . 2>/dev/null || \
curl -s "$GATEWAY_URL/admin/status" -H "Authorization: Bearer $ACCESS_TOKEN"
echo -e "\n"

# 访问后端服务状态
echo "5. 查看后端服务状态..."
curl -s "$GATEWAY_URL/admin/backends" \
  -H "Authorization: Bearer $ACCESS_TOKEN" | jq . 2>/dev/null || \
curl -s "$GATEWAY_URL/admin/backends" -H "Authorization: Bearer $ACCESS_TOKEN"
echo -e "\n"

# 测试速率限制
echo "6. 测试速率限制（快速连续请求）..."
for i in {1..5}; do
    echo "请求 $i:"
    curl -s -w "状态码: %{http_code}, 响应时间: %{time_total}s\n" \
         -o /dev/null "$GATEWAY_URL/health"
done
echo ""

# 测试无效的认证
echo "7. 测试无效认证..."
curl -s -w "状态码: %{http_code}\n" \
     -o /dev/null "$GATEWAY_URL/admin/status" \
     -H "Authorization: Bearer invalid-token"
echo ""

# 检查指标
echo "8. 检查Prometheus指标..."
curl -s "$METRICS_URL/metrics" | head -20
echo "..."
echo "（显示前20行，更多指标请访问 $METRICS_URL/metrics）"
echo ""

# 刷新令牌演示
echo "9. 刷新访问令牌..."
REFRESH_TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.refresh_token' 2>/dev/null)
if [ "$REFRESH_TOKEN" != "null" ] && [ -n "$REFRESH_TOKEN" ]; then
    REFRESH_RESPONSE=$(curl -s -X POST "$GATEWAY_URL/auth/refresh" \
      -H "Content-Type: application/json" \
      -d "{\"refresh_token\":\"$REFRESH_TOKEN\"}")
    echo "$REFRESH_RESPONSE" | jq . 2>/dev/null || echo "$REFRESH_RESPONSE"
else
    echo "无法获取刷新令牌"
fi
echo ""

# 登出
echo "10. 用户登出..."
curl -s -X POST "$GATEWAY_URL/auth/logout" \
  -H "Authorization: Bearer $ACCESS_TOKEN" | jq . 2>/dev/null || \
curl -s -X POST "$GATEWAY_URL/auth/logout" -H "Authorization: Bearer $ACCESS_TOKEN"
echo -e "\n"

echo "=== 演示完成 ==="
echo ""
echo "其他可用端点:"
echo "- GET $GATEWAY_URL/health - 健康检查"
echo "- GET $GATEWAY_URL/health/detailed - 详细健康检查"
echo "- POST $GATEWAY_URL/auth/login - 用户登录"
echo "- POST $GATEWAY_URL/auth/refresh - 刷新令牌"
echo "- POST $GATEWAY_URL/auth/logout - 用户登出"
echo "- GET $GATEWAY_URL/admin/status - 网关状态（需要认证）"
echo "- GET $GATEWAY_URL/admin/backends - 后端服务状态（需要认证）"
echo "- GET $METRICS_URL/metrics - Prometheus指标"
echo "- GET $METRICS_URL/health - 指标服务健康检查"
