# Message load test

本命令只测消息发送、落库和在线推送链路。登录、Ticket 和 WebSocket 建连属于准备阶段，不计入消息延迟。

先通过真实注册、登录和创建群聊接口准备 10 个用户：

```bash
go run ./cmd/message-loadtest \
  -prepare-users 10 \
  -users /tmp/mmchat-load-users.json
```

一次准备 10 个互不重叠的群、每群 50 个用户：

```bash
go run ./cmd/message-loadtest \
  -prepare-groups 10 \
  -group-members 50 \
  -group-name message-load-test \
  -users /tmp/mmchat-load-users.json
```

准备命令从本地 `configs/config.toml` 连接 Redis，只用于读取本地短信验证码；用户、登录 Session 和群聊均由真实 HTTP 接口创建。输出文件权限为 `0600`。

用户文件格式：

```json
[
  {
    "user_id": 101,
    "access_token": "<access_token>",
	"refresh_token": "<refresh_token>",
    "conversation_ids": [301]
  },
  {
    "user_id": 102,
    "access_token": "<access_token>",
	"refresh_token": "<refresh_token>",
    "conversation_ids": [301]
  }
]
```

每个用户必须拥有独立登录会话，并且必须是所列会话的成员。同一个会话列出的压测用户都会建立 WebSocket，工具据此统计接收方实时推送数量。

运行：

```bash
go run ./cmd/message-loadtest \
  -users /absolute/path/users.json \
  -rate 100 \
  -duration 5m \
  -drain 30s
```

常用参数：

```text
-base-url          mmchat HTTP 地址，默认 http://127.0.0.1:6666
-rate              所有用户合计每秒发送消息数
-duration          持续发送时间
-drain             停止发送后等待最终事件的时间
-connect-workers   并行建立连接的数量
-client-queue      每个压测客户端的本地发送队列长度
-report-interval   中间结果输出间隔
-prepare-users     创建真实用户和群聊，输出用户文件后退出
-prepare-groups    创建多个互不重叠的群聊，输出用户文件后退出
-group-members     多群准备模式下每个群的用户数
-password          准备用户时使用的统一密码
-group-name        准备用户时创建的群名称
```

容量以 `send_to_created` 和 `send_to_receiver` 为准，不能以 `message.accepted` 为准。`realtime_missing` 可以通过 HTTP 恢复，但 `gaps_after_sync` 必须为 `0`。

准备模式会保存 Refresh Token。Access Token 过期时，压测客户端会为对应用户自动轮换 Token，再继续申请 Ticket 或执行 HTTP 缺口同步。
