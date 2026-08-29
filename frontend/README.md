# Mocha Web

基于 React 19 与 HeroUI 3 构建的 Mocha 登录/注册前端。

## 本地运行

```bash
npm install
npm run dev
```

开发地址为 `http://127.0.0.1:5173`。开发服务器会把 `/api` 与 `/static` 请求代理到 `http://127.0.0.1:6666` 的 Go 服务。

生产构建：

```bash
npm run build
```

如前端与后端不在同一域名，可通过 `VITE_API_BASE` 指定接口地址。

## 当前范围与接口状态

- 包含登录、注册、登录后的三栏工作台、用户搜索和个人资料管理页面。
- 注册页的“获取验证码”已接入 `POST /api/v1/auth/register-code`。
- 完成注册已接入 `POST /api/v1/auth/register`。
- 登录已接入 `POST /api/v1/auth/login`，并按照“保持登录”选项将令牌写入本地或会话存储。
- 登录后默认进入消息工作台，个人资料通过“我的”入口访问。
- 个人资料概览与编辑已接入 `GET /api/v1/users/me` 与 `PATCH /api/v1/users/me`。
- 用户搜索已接入 `GET /api/v1/users`，支持手机号精确搜索以及昵称、地区、年龄、性别组合筛选和游标分页。
- 资料编辑支持昵称、性别、个性签名、生日、国家/地区代码和省份；手机号与邮箱只读。
- 用户标识仅供内部请求处理，不在页面中展示。
- 登录后界面采用原生 React/CSS 三栏聊天布局，并直接展示接口返回的 `avatar_url`；HeroUI 仅用于登录与注册表单。
- 访问令牌过期后会使用 `POST /api/v1/auth/refresh` 自动刷新一次；刷新失败则返回登录页。
- 注册验证码遵循 5 分钟有效期、60 秒发送间隔和每小时 5 次的接口限制。
