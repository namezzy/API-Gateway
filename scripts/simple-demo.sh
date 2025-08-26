#!/bin/bash

# 简化版API网关演示脚本

echo "=== 简化版API Gateway 演示 ==="

GATEWAY_URL="http://localhost:8080"

echo "网关地址: $GATEWAY_URL"
echo ""

# 启动网关（在后台）
echo "1. 启动API网关..."
cd /root/API-Gateway
./simple-gateway -port 8080 &
GATEWAY_PID=$!

# 等待网关启动
sleep 3

# 检查网关是否启动成功
if ! curl -s "$GATEWAY_URL/health" > /dev/null; then
    echo "网关启动失败"
    kill $GATEWAY_PID 2>/dev/null
    exit 1
fi

echo "网关启动成功！"
echo ""

# 测试健康检查
echo "2. 测试健康检查..."
curl -s "$GATEWAY_URL/health" | head -10
echo ""
echo ""

# 测试状态端点
echo "3. 测试状态端点..."
curl -s "$GATEWAY_URL/status" | head -15
echo ""
echo ""

# 测试指标端点
echo "4. 测试指标端点..."
curl -s "$GATEWAY_URL/metrics"
echo ""
echo ""

# 测试代理功能
echo "5. 测试代理功能（可能需要网络连接）..."
echo "尝试通过网关访问外部API..."

# 测试JSONPlaceholder API
echo "请求示例 - 获取用户列表:"
curl -s -w "状态码: %{http_code}, 响应时间: %{time_total}s\n" \
     "$GATEWAY_URL/api/users" | head -5
echo ""

# 测试速率限制
echo "6. 测试速率限制（快速连续请求）..."
for i in {1..5}; do
    echo "请求 $i:"
    curl -s -w "状态码: %{http_code}, 响应时间: %{time_total}s\n" \
         -o /dev/null "$GATEWAY_URL/health"
done
echo ""

# 再次检查指标
echo "7. 查看更新后的指标..."
curl -s "$GATEWAY_URL/metrics"
echo ""
echo ""

# 查看最终状态
echo "8. 查看最终状态..."
curl -s "$GATEWAY_URL/status" | head -20
echo ""

# 清理
echo ""
echo "=== 演示完成，正在停止网关 ==="
kill $GATEWAY_PID 2>/dev/null
wait $GATEWAY_PID 2>/dev/null

echo ""
echo "网关功能演示："
echo "✓ 健康检查 - GET $GATEWAY_URL/health"
echo "✓ 状态监控 - GET $GATEWAY_URL/status"
echo "✓ 指标收集 - GET $GATEWAY_URL/metrics"
echo "✓ 负载均衡 - 轮询选择后端服务"
echo "✓ 速率限制 - 每分钟100次请求限制"
echo "✓ 反向代理 - 代理请求到后端服务"
echo "✓ 连接监控 - 实时统计活跃连接数"
echo ""
echo "可以使用以下命令手动启动网关:"
echo "cd /root/API-Gateway && ./simple-gateway -port 8080"
