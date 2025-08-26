# API Gateway Dashboard (React + Vite + MUI)

现代化可视化面板，用于查看 Go API Gateway 的健康状态、后端服务、基础指标。

## 功能
- 用户登录 (JWT) 示例
- 网关健康 & 路由汇总
- 后端服务表格（健康 / 连接数 / 权重）
- 简单指标曲线解析（Prometheus 文本抓取示例）
- 暗黑 / 明亮主题切换
- 响应式布局 + 数据自动轮询

## 启动
```bash
cd frontend
npm install  # 或 pnpm install / yarn
npm run dev
```
浏览器访问: http://localhost:5173

默认代理： `/auth`, `/admin`, `/health`, `/metrics` 指向后端 8080 / 9090 端口。

## 生产构建
```bash
npm run build
npm run preview
```

## 技术栈
- React 18 + TypeScript
- Vite 构建
- Material UI 组件系统
- React Query 数据层
- Axios 请求封装
- Recharts 简易图表

## 后续可拓展
- 令牌刷新 / 登出黑名单
- 指标 WebSocket 推送
- 后端动态拓扑 / 拓展面板
- Grafana 嵌入 iframe
- 权限粒度 (RBAC)

## 与后端集成注意
后端登录接口返回的 access_token 已写入 Axios 默认 Authorization 头部，刷新策略可在 `AuthContext` 中扩展。

---

MIT License
