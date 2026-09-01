# Mocha Web

Mocha 的 React 19 前端，使用 Vite 5 开发和构建。界面包含登录、注册以及登录后的三栏聊天工作台。服务端协议以 [`../doc/api.md`](../doc/api.md) 为准。

## 已接入的能力

- 认证：发送注册验证码、注册、登录、退出和 Refresh Token 轮换。“保持登录”决定会话写入 `localStorage` 还是 `sessionStorage`；多个并发 `UNAUTHENTICATED` 请求共用一次刷新。
- 用户：查看和修改当前用户资料、更换头像，以及按手机号或昵称、地区、年龄、性别等条件搜索用户并游标分页。
- 媒体：申请上传、向对象存储的签名 URL 直传文件、确认上传完成；客户端可区分图片、视频、音频和普通文件，并为图片提供发送前、本地乐观消息、实时消息和历史消息预览。
- 会话：创建或获取私聊、创建群聊、分页获取会话列表、获取单个会话，以及按 `before_seq` / `after_seq` 分页获取历史消息和同步缺口。
- 实时消息：通过一次性 Ticket 建立 WebSocket，等待 `auth.ok` 后发送 `message.send` 及送达/已读进度，处理 `message.accepted`、`message.created`、`message.rejected`、`conversation.delivered` 和 `conversation.read`，并在断线后重新申请 Ticket、退避重连。
- 当前协议边界：服务端不支持会话置顶、消息撤回/编辑/删除、群成员变更或群资料修改。附件使用消息接口返回的临时 URL；图片 URL 失效时客户端会重新拉取对应消息刷新地址。

当前客户端封装覆盖以下 HTTP 与 Upgrade 入口：

| 模块 | 接口 |
| --- | --- |
| 认证 | `POST /api/v1/auth/register-code`、`POST /api/v1/auth/register`、`POST /api/v1/auth/login`、`POST /api/v1/auth/refresh`、`POST /api/v1/auth/logout` |
| 用户 | `GET /api/v1/users/me`、`PATCH /api/v1/users/me`、`PUT /api/v1/users/me/avatar`、`GET /api/v1/users` |
| 媒体 | `POST /api/v1/media/uploads`、`POST /api/v1/media/uploads/:id/complete` |
| 会话 | `POST /api/v1/conversations/direct`、`POST /api/v1/conversations/group`、`GET /api/v1/conversations`、`GET /api/v1/conversations/:id`、`GET /api/v1/conversations/:id/messages` |
| WebSocket | `POST /api/v1/ws/tickets`、`GET /api/v1/ws?ticket=...` |

## 联调前置条件

后端必须从仓库根目录启动，因为配置和日志使用相对路径。仓库目前没有 Compose、Makefile 或 `.env.example`，需要先自行启动以下服务，并与 [`../configs/config.toml`](../configs/config.toml) 保持一致：

| 依赖 | 默认地址 | 联调要求 |
| --- | --- | --- |
| MySQL | `127.0.0.1:3306` | 预先创建 `mocha` 数据库和可登录的 `mocha` 用户，默认密码为配置中的 `123456`。启动时会自动迁移表结构，所以该用户需要 DDL 权限。 |
| Redis | `127.0.0.1:6379` | 默认密码 `123456`、DB `0`；用于验证码、登录会话、WebSocket Ticket 和消息推送。后端启动时不主动 Ping Redis，需要单独确认其可连通。 |
| Kafka | `127.0.0.1:9092` | 必须在 HTTP 服务启动前可连通；准备 `chat.message.command.v1` 和 `chat.message.committed.v1` 两个 Topic。即使只联调登录，Kafka 不可用也会阻止后端启动。 |
| MinIO | `127.0.0.1:9000` | 预先创建 `mocha` Bucket，并为浏览器直传配置允许 `http://127.0.0.1:5173` 的 CORS；否则头像和附件上传会失败。 |

启动后端前还需要设置：

```bash
export MOCHA_JWT_SECRET='replace-with-a-secret-of-at-least-32-bytes'
export MOCHA_MINIO_ACCESS_KEY='replace-with-your-access-key'
export MOCHA_MINIO_SECRET_KEY='replace-with-your-secret-key'
```

`MOCHA_JWT_SECRET` 少于 32 字节时后端会直接退出。MinIO 密钥缺失或 Bucket/CORS 未配置不一定阻止 HTTP 服务启动，但会阻断媒体联调。

## 本地启动

1. 在仓库根目录启动 Go 服务：

   ```bash
   go run ./cmd/server
   ```

   默认监听 `http://127.0.0.1:6666`。本地短信发送器不会真正发短信，注册验证码会输出到后端终端和 `logs/app.log`。

2. 在另一个终端启动前端：

   ```bash
   cd frontend
   npm ci
   npm run dev
   ```

3. 访问 `http://127.0.0.1:5173`。不要改用 `http://localhost:5173`：当前 HTTP CORS 和 WebSocket Origin 白名单均精确配置为 `http://127.0.0.1:5173`。

开发服务器默认将 `/api`（包含 WebSocket Upgrade）和 `/static` 代理到 `http://127.0.0.1:6666`。当前媒体主流程使用 MinIO 签名 URL；后端声明的 `/static/avatars` 和 `/static/files` 仅在对应目录存在且有内容时可用。

## 前端环境变量

| 变量 | 用途 | 默认值 |
| --- | --- | --- |
| `VITE_PROXY_TARGET` | 修改 Vite 开发代理目标，同时影响 HTTP、WebSocket 和 `/static` | `http://127.0.0.1:6666` |
| `VITE_API_BASE` | 让浏览器直接请求指定的 HTTP API 根地址；空值时走同源路径 | 空 |
| `VITE_WS_URL` | 显式指定 WebSocket 端点，例如 `ws://127.0.0.1:6666/api/v1/ws` | 由 `VITE_API_BASE` 或当前页面地址推导 |

仅更改开发后端时，优先使用代理：

```bash
VITE_PROXY_TARGET=http://127.0.0.1:6666 npm run dev
```

如果设置 `VITE_API_BASE` 绕过代理，需要同步放行后端 HTTP CORS；如果设置 `VITE_WS_URL`，也需要放行 WebSocket Origin。

## 构建与检查

```bash
cd frontend
npm run build
```

产物输出到 `frontend/dist`。Go 服务不托管该目录；生产环境应用静态服务器托管 `dist`，并将 `/api`、`/static` 和 WebSocket Upgrade 反向代理到 Go 服务。如果前后端分域部署，则需要在构建时设置 `VITE_API_BASE` / `VITE_WS_URL`，并配置两类 Origin 白名单。

后端基础检查：

```bash
go test ./...
go vet ./...
```

当前 Go 包没有自动化测试文件，上述 `go test` 主要验证全量编译。
