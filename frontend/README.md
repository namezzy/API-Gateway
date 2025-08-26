# API Gateway Dashboard (React + Vite + MUI)

现代化可视化面板，用于查看 Go API Gateway 的健康状态、后端服务、指标与 PromQL 查询。

## 功能
- 用户登录 (JWT) 示例
- 网关健康 & 路由汇总
- 后端服务表格（健康 / 连接数 / 权重）
- 简单指标曲线解析（Prometheus 文本抓取示例）
- PromQL 实时查询 (Prometheus HTTP API)
- JWT 访问令牌 + 自动刷新（Access + Refresh）
- Axios 拦截器 + 全局 Snackbar 通知
- 暗黑 / 明亮主题切换 + 品牌主色/次色自定义 (🎨 面板)
- 响应式布局 + 数据自动轮询

## 启动（本地开发）
```bash
cd frontend
npm install  # 或 pnpm install / yarn
npm run dev
```
浏览器访问: http://localhost:5173

默认代理： `/auth`, `/admin`, `/health`, `/metrics` 指向后端 8080 / 9090 端口。Prometheus 额外通过 `http://localhost:9091` 暴露，PromQL 直接调用其 `/api/v1/query`。

如需修改：见 `vite.config.ts`。

## 生产构建 (独立)
```bash
npm run build
npm run preview
```

## 与后端 Docker 一体化打包

后端 `Dockerfile` 已集成多阶段构建：

1. Go 编译阶段 (go-builder)
2. 前端构建阶段 (fe-builder) -> `frontend/dist`
3. 最终 alpine 镜像复制 `gateway` + `/public` 静态文件

运行容器后访问 `http://localhost:8080/` 即可打开前端 (SPA fallback)。

若不需要前端，将 `Dockerfile` 中前端相关段落删除即可。

## CI 集成

GitHub Actions 中新增 `frontend` job：
- 安装依赖 (Node 20)
- Lint (`eslint`)
- Build (Vite)
- 上传 `dist` Artifact

## 技术栈与关键模块
- React 18 + TypeScript
- Vite 构建
- Material UI 组件系统
- React Query 数据层
- Axios 请求封装
- Recharts 简易图表
- Prometheus HTTP API 查询 (PromQL)
- Axios 封装 + Token 自动续期 + 全局错误提示

## Token 刷新机制说明

登录成功后保存：
```
localStorage.gateway.tokens = {
	access: <JWT>,
	refresh: <JWT>,
	expiresAt: <ms>
}
```
`AuthContext` 解析 access_token 的 `exp`，在过期前 60s 自动调用 `/auth/refresh` 获取新 access token (后端当前不返回新的 refresh)。失败会清除本地状态并提示重新登录。

## PromQL 查询

在 指标 -> PromQL 部分输入表达式（默认 `up`），点击 执行 调用 `http://localhost:9091/api/v1/query?query=...` 显示即时结果（标签+数值）。可与 Docker Compose 中的 Prometheus 服务联动。

## 品牌主题自定义

点击顶部 🎨 图标打开设置，可动态修改 `primary` / `secondary` 主色并立即应用 (MUI 动态 theme)。支持重置。

## 后续可拓展
- 登出刷新令牌黑名单 (后端持久化 revoke)
- 指标 WebSocket 推送
- 后端动态拓扑 / 拓展面板
- Grafana 嵌入 iframe
- 权限粒度 (RBAC)

## 与后端集成注意
后端登录接口返回的 access_token 已写入 Axios 默认 Authorization 头部，刷新策略可在 `AuthContext` 中扩展。

---

MIT License
