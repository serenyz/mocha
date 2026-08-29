# mmchat API 文档

更新时间：2026-08-29  
API 前缀：`/api/v1`

## 1. 接口列表

| 方法 | 路径 | 需要登录 | 说明 |
| --- | --- | --- | --- |
| `POST` | `/api/v1/auth/register-code` | 否 | 发送注册验证码 |
| `POST` | `/api/v1/auth/register` | 否 | 注册用户 |
| `POST` | `/api/v1/auth/login` | 否 | 登录 |
| `POST` | `/api/v1/auth/refresh` | 否 | 刷新 Token |
| `POST` | `/api/v1/auth/logout` | 是 | 退出登录 |
| `GET` | `/api/v1/users/me` | 是 | 获取当前用户资料 |
| `PATCH` | `/api/v1/users/me` | 是 | 修改当前用户资料 |
| `PUT` | `/api/v1/users/me/avatar` | 是 | 更换当前用户头像 |
| `GET` | `/api/v1/users` | 是 | 搜索用户 |
| `POST` | `/api/v1/media/uploads` | 是 | 申请媒体文件直传 |
| `POST` | `/api/v1/media/uploads/:uuid/complete` | 是 | 确认媒体文件上传完成 |

不提供通过 UUID 搜索用户的接口。

## 2. 通用约定

### 2.1 JSON 请求

带请求体的接口使用：

```http
Content-Type: application/json
```

### 2.2 登录鉴权

需要登录的接口必须携带：

```http
Authorization: Bearer <access_token>
```

### 2.3 成功响应

有数据：

```json
{
  "success": true,
  "data": {}
}
```

无数据：

```json
{
  "success": true
}
```

### 2.4 错误响应

```json
{
  "success": false,
  "error": {
    "code": "INVALID_ARGUMENT",
    "message": "请求参数错误"
  }
}
```

### 2.5 字段格式

- `birthday`：`YYYY-MM-DD`。
- `created_at`：RFC 3339 时间字符串。
- `expires_at`、`url_expired_at`：RFC 3339 时间字符串。
- Token 有效期字段：秒。
- `country`：两个大写字母组成的国家或地区代码，例如 `CN`、`US`、`JP`。
- `gender`：`0` 未知、`1` 男、`2` 女。
- `uuid`、`media_uuid`：UUID 字符串。
- `avatar_url`、`media_url` 和上传 URL 均可能是带签名的临时 URL，不应持久化保存。

## 3. 认证接口

### 3.1 发送注册验证码

```http
POST /api/v1/auth/register-code
```

请求：

```json
{
  "phone": "13800138000"
}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `phone` | string | 是 | 中国大陆手机号 |

成功：

```http
HTTP/1.1 200 OK
```

```json
{
  "success": true
}
```

当前验证码规则：

- 有效期为 5 分钟。
- 同一手机号 60 秒内不能重复发送。
- 同一手机号每小时最多发送 5 次。

可能的错误：

| HTTP 状态码 | 错误码 | 说明 |
| --- | --- | --- |
| `400` | `INVALID_ARGUMENT` | 请求结构不正确 |
| `400` | `INVALID_PHONE` | 手机号格式不正确 |
| `429` | `REGISTER_CODE_COOLDOWN` | 发送过于频繁 |
| `429` | `REGISTER_CODE_HOURLY_LIMIT` | 每小时发送次数达到上限 |

### 3.2 注册

```http
POST /api/v1/auth/register
```

请求：

```json
{
  "phone": "13800138000",
  "password": "example-password",
  "nickname": "Alice",
  "code": "123456"
}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `phone` | string | 是 | 中国大陆手机号 |
| `password` | string | 是 | 8 个字符以上，UTF-8 编码后最多 64 字节 |
| `nickname` | string | 是 | 昵称，1～50 个字符 |
| `code` | string | 是 | 6 位注册验证码 |

成功：

```http
HTTP/1.1 201 Created
```

```json
{
  "success": true
}
```

可能的错误：

| HTTP 状态码 | 错误码 | 说明 |
| --- | --- | --- |
| `400` | `INVALID_ARGUMENT` | 请求结构不正确 |
| `400` | `INVALID_PHONE` | 手机号格式不正确 |
| `400` | `WEAK_PASSWORD` | 密码格式不符合要求 |
| `400` | `INVALID_NICKNAME` | 昵称格式不符合要求 |
| `409` | `PHONE_REGISTERED` | 手机号已经注册 |
| `422` | `REGISTER_CODE_INVALID` | 验证码错误 |
| `422` | `REGISTER_CODE_EXPIRED` | 验证码已过期 |

### 3.3 登录

```http
POST /api/v1/auth/login
```

请求：

```json
{
  "phone": "13800138000",
  "password": "example-password"
}
```

成功：

```http
HTTP/1.1 200 OK
```

```json
{
  "success": true,
  "data": {
    "access_token": "<access_token>",
    "refresh_token": "<refresh_token>",
    "token_type": "Bearer",
    "expires_in": 900,
    "refresh_expires_in": 2592000,
    "user": {
      "uuid": "019d4a55-6f40-7ab0-a2ad-7d677ea1d836",
      "nickname": "Alice"
    }
  }
}
```

可能的错误：

| HTTP 状态码 | 错误码 | 说明 |
| --- | --- | --- |
| `400` | `INVALID_ARGUMENT` | 请求结构不正确 |
| `400` | `INVALID_PHONE` | 手机号格式不正确 |
| `401` | `INVALID_CREDENTIALS` | 手机号或密码错误 |
| `403` | `ACCOUNT_DISABLED` | 账号不可用 |

### 3.4 刷新 Token

```http
POST /api/v1/auth/refresh
```

请求：

```json
{
  "refresh_token": "<refresh_token>"
}
```

成功：

```http
HTTP/1.1 200 OK
```

```json
{
  "success": true,
  "data": {
    "access_token": "<new_access_token>",
    "refresh_token": "<new_refresh_token>",
    "token_type": "Bearer",
    "expires_in": 900,
    "refresh_expires_in": 2591900
  }
}
```

刷新成功后，客户端必须保存新的 Access Token 和 Refresh Token，旧 Refresh Token 不再使用。

可能的错误：

| HTTP 状态码 | 错误码 | 说明 |
| --- | --- | --- |
| `400` | `INVALID_ARGUMENT` | 缺少 `refresh_token` |
| `401` | `INVALID_REFRESH_TOKEN` | Refresh Token 无效或已过期 |
| `403` | `ACCOUNT_DISABLED` | 账号不可用 |

### 3.5 退出登录

```http
POST /api/v1/auth/logout
Authorization: Bearer <access_token>
```

成功：

```http
HTTP/1.1 200 OK
```

```json
{
  "success": true
}
```

可能的错误：

| HTTP 状态码 | 错误码 | 说明 |
| --- | --- | --- |
| `401` | `UNAUTHENTICATED` | 登录状态无效 |

## 4. 用户接口

### 4.1 获取当前用户资料

```http
GET /api/v1/users/me
Authorization: Bearer <access_token>
```

成功：

```json
{
  "success": true,
  "data": {
    "phone": "13800138000",
    "email": "alice@example.com",
    "created_at": "2026-08-28T10:00:00+08:00",
    "uuid": "019d4a55-6f40-7ab0-a2ad-7d677ea1d836",
    "nickname": "Alice",
    "avatar_url": "http://127.0.0.1:9000/mocha/media/image/2026/08/019d...?X-Amz-Algorithm=...",
    "url_expired_at": "2026-08-29T10:15:00Z",
    "gender": 2,
    "birthday": "2001-03-18",
    "country": "US",
    "province": "California",
    "signature": "Hello"
  }
}
```

`email` 未绑定时不返回该字段。`avatar_url` 是临时访问地址，当前有效期为 15 分钟；前端不能将其作为永久地址保存。

可能的错误：

| HTTP 状态码 | 错误码 | 说明 |
| --- | --- | --- |
| `401` | `UNAUTHENTICATED` | 登录状态无效 |
| `403` | `ACCOUNT_DISABLED` | 账号不可用 |
| `404` | `USER_NOT_FOUND` | 用户不存在 |

### 4.2 修改当前用户资料

```http
PATCH /api/v1/users/me
Authorization: Bearer <access_token>
Content-Type: application/json
```

请求字段均为可选，只提交需要修改的字段：

```json
{
  "nickname": "New Alice",
  "gender": 2,
  "signature": "New signature",
  "birthday": "2001-03-18",
  "country": "US",
  "province": "California"
}
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `nickname` | string | 昵称，1～50 个字符 |
| `gender` | integer | `0`、`1` 或 `2` |
| `signature` | string | 个性签名，最长 200 个字符，可传空字符串 |
| `birthday` | string | `YYYY-MM-DD`，不能晚于当天 |
| `country` | string | 国家或地区代码，可传空字符串 |
| `province` | string | 一级行政区，最长 100 个字符，可传空字符串 |

成功响应为修改后的完整当前用户资料，结构与 `GET /api/v1/users/me` 相同。

可能的错误：

| HTTP 状态码 | 错误码 | 说明 |
| --- | --- | --- |
| `400` | `INVALID_ARGUMENT` | 请求结构不正确 |
| `400` | `INVALID_NICKNAME` | 昵称格式不正确 |
| `400` | `INVALID_GENDER` | 性别参数不正确 |
| `400` | `INVALID_SIGNATURE` | 个性签名格式不正确 |
| `400` | `INVALID_BIRTHDAY` | 生日格式不正确 |
| `400` | `INVALID_COUNTRY` | 国家或地区格式不正确 |
| `400` | `INVALID_PROVINCE` | 一级行政区格式不正确 |
| `401` | `UNAUTHENTICATED` | 登录状态无效 |
| `403` | `ACCOUNT_DISABLED` | 账号不可用 |
| `404` | `USER_NOT_FOUND` | 用户不存在 |

### 4.3 更换当前用户头像

```http
PUT /api/v1/users/me/avatar
Authorization: Bearer <access_token>
Content-Type: application/json
```

请求：

```json
{
  "media_uuid": "019d4a55-6f40-7ab0-a2ad-7d677ea1d836"
}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `media_uuid` | string | 是 | 已上传完成、属于当前用户的图片 Media UUID |

前端必须先完成媒体上传申请、对象存储直传和上传完成确认，再调用本接口更换头像。

成功：

```http
HTTP/1.1 200 OK
```

```json
{
  "success": true,
  "data": {
    "media_uuid": "019d4a55-6f40-7ab0-a2ad-7d677ea1d836",
    "avatar_url": "http://127.0.0.1:9000/mocha/media/image/2026/08/019d...?X-Amz-Algorithm=...",
    "url_expired_at": "2026-08-29T10:15:00Z"
  }
}
```

`avatar_url` 是临时访问地址，当前有效期为 15 分钟。服务器只保存头像与 Media 的关联，不保存该临时 URL。

可能的错误：

| HTTP 状态码 | 错误码 | 说明 |
| --- | --- | --- |
| `400` | `INVALID_ARGUMENT` | 请求体或 `media_uuid` 格式不正确 |
| `401` | `UNAUTHENTICATED` | 登录状态无效 |
| `404` | `MEDIA_NOT_FOUND` | Media 不存在、不属于当前用户或尚未上传完成 |

### 4.4 搜索用户

```http
GET /api/v1/users
Authorization: Bearer <access_token>
```

查询参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `phone` | string | 否 | 中国大陆手机号，精确匹配 |
| `nickname` | string | 否 | 昵称前缀匹配 |
| `country` | string | 否 | 国家或地区代码，精确匹配 |
| `province` | string | 否 | 一级行政区，精确匹配 |
| `age` | integer | 否 | 周岁，范围 `0～150` |
| `gender` | integer | 否 | 性别，范围 `0～2` |
| `cursor` | integer | 否 | 上一页响应中的 `next_cursor` |
| `limit` | integer | 否 | 返回数量，默认 `20`，最大 `50` |

规则：

- 至少提供一个搜索条件。
- `phone` 执行精确搜索，最多匹配一个用户。
- `phone` 不能和其他搜索条件同时使用。
- 非手机号条件可以组合，条件之间为 `AND`。
- 只返回正常状态的用户。
- 没有结果时返回空数组，不返回错误。
- 请求下一页时，搜索条件必须保持不变，并传入上一页的 `next_cursor`。
- `cursor` 仅用于分页，前端应原样传回，不要解析其含义。

手机号搜索：

```http
GET /api/v1/users?phone=13800138000
```

组合搜索：

```http
GET /api/v1/users?nickname=Ali&country=US&age=25&gender=2&limit=20
```

下一页：

```http
GET /api/v1/users?nickname=Ali&country=US&age=25&gender=2&cursor=120&limit=20
```

成功：

```json
{
  "success": true,
  "data": [
    {
      "uuid": "019d4a55-6f40-7ab0-a2ad-7d677ea1d836",
      "nickname": "Alice",
      "avatar_url": "http://127.0.0.1:9000/mocha/media/image/2026/08/019d...?X-Amz-Algorithm=...",
      "url_expired_at": "2026-08-29T10:15:00Z",
      "gender": 2,
      "birthday": "2001-03-18",
      "country": "US",
      "province": "California",
      "signature": "Hello"
    }
  ],
  "meta": {
    "next_cursor": 120,
    "has_more": true,
    "limit": 20
  }
}
```

没有下一页时：

```json
{
  "success": true,
  "data": [],
  "meta": {
    "next_cursor": null,
    "has_more": false,
    "limit": 20
  }
}
```

可能的错误：

| HTTP 状态码 | 错误码 | 说明 |
| --- | --- | --- |
| `400` | `INVALID_ARGUMENT` | 搜索条件或分页参数不正确 |
| `400` | `INVALID_PHONE` | 手机号格式不正确 |
| `400` | `INVALID_NICKNAME` | 昵称格式不正确 |
| `400` | `INVALID_COUNTRY` | 国家或地区格式不正确 |
| `400` | `INVALID_PROVINCE` | 一级行政区格式不正确 |
| `400` | `INVALID_GENDER` | 性别参数不正确 |
| `401` | `UNAUTHENTICATED` | 登录状态无效 |

## 5. 媒体接口

媒体文件采用对象存储直传，完整流程如下：

```text
申请上传
→ 前端 PUT 文件到返回的对象存储 URL
→ 确认上传完成
→ 使用 Media UUID 执行业务操作，例如更换头像
```

当前只支持单次 PUT 直传，不支持分片上传。单个文件最大为 32 MiB。

### 5.1 申请媒体文件上传

```http
POST /api/v1/media/uploads
Authorization: Bearer <access_token>
Content-Type: application/json
```

请求：

```json
{
  "type": "image",
  "filename": "avatar.jpg",
  "mime_type": "image/jpeg",
  "size": 245678
}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `type` | string | 是 | `image`、`video`、`audio` 或 `file` |
| `filename` | string | 是 | 原始文件名，UTF-8 编码后最多 255 字节，不能包含 `/` 或 `\` |
| `mime_type` | string | 是 | 标准 MIME 类型，不携带额外参数 |
| `size` | integer | 是 | 文件字节数，范围 `1～33554432` |

`type` 必须和 `mime_type` 一致：

| `type` | `mime_type` |
| --- | --- |
| `image` | 以 `image/` 开头 |
| `video` | 以 `video/` 开头 |
| `audio` | 以 `audio/` 开头 |
| `file` | 不属于以上三类的其他 MIME 类型 |

成功：

```http
HTTP/1.1 200 OK
```

```json
{
  "success": true,
  "data": {
    "media_uuid": "019d4a55-6f40-7ab0-a2ad-7d677ea1d836",
    "upload": {
      "method": "PUT",
      "url": "http://127.0.0.1:9000/mocha/media/image/2026/08/019d...?X-Amz-Algorithm=...",
      "headers": {
        "Content-Type": "image/jpeg"
      },
      "expires_at": "2026-08-29T11:00:00Z"
    }
  }
}
```

上传地址当前有效期为 1 小时。前端上传文件时：

- 请求方法必须使用响应中的 `upload.method`。
- 请求地址必须直接使用响应中的 `upload.url`，不能自行拼接。
- 必须携带响应中的全部 `upload.headers`。
- 请求体直接放文件二进制内容，不使用 JSON 或 `multipart/form-data`。
- 不携带 mmchat 的 Access Token。
- PUT 成功后必须调用“确认媒体文件上传完成”接口。

示例：

```http
PUT <upload.url>
Content-Type: image/jpeg

<文件二进制内容>
```

可能的错误：

| HTTP 状态码 | 错误码 | 说明 |
| --- | --- | --- |
| `400` | `INVALID_ARGUMENT` | 请求结构、文件名或文件大小不正确 |
| `400` | `INVALID_MEDIA_TYPE` | `type` 不受支持 |
| `401` | `UNAUTHENTICATED` | 登录状态无效 |
| `403` | `ACCOUNT_DISABLED` | 账号不可用 |
| `404` | `USER_NOT_FOUND` | 用户不存在 |
| `413` | `MEDIA_TOO_LARGE` | 文件超过 32 MiB |
| `415` | `UNSUPPORTED_MEDIA_FORMAT` | MIME 类型无效，或与 `type` 不一致 |

### 5.2 确认媒体文件上传完成

```http
POST /api/v1/media/uploads/:uuid/complete
Authorization: Bearer <access_token>
```

路径参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `uuid` | string | 申请上传时返回的 `media_uuid` |

本接口没有请求体。

服务器会向对象存储查询文件，并校验实际文件大小、MIME 类型和上传时间。不能只完成 PUT 而跳过本接口。

成功：

```http
HTTP/1.1 200 OK
```

```json
{
  "success": true,
  "data": {
    "media_uuid": "019d4a55-6f40-7ab0-a2ad-7d677ea1d836",
    "filename": "avatar.jpg",
    "mime_type": "image/jpeg",
    "filesize": 245678,
    "media_url": "http://127.0.0.1:9000/mocha/media/image/2026/08/019d...?X-Amz-Algorithm=...",
    "url_expired_at": "2026-08-29T10:15:00Z",
    "status": "uploaded"
  }
}
```

规则：

- 只有 Media 所属用户可以确认上传；其他用户请求时同样返回 `MEDIA_NOT_FOUND`。
- 对象存储中的实际文件大小必须等于申请时的 `size`。
- 对象存储中的实际 Content-Type 必须等于申请时的 `mime_type`。
- 对象最后修改时间不能晚于上传申请的 `expires_at`。
- 已确认成功的 Media 再次调用本接口时仍返回成功，并生成新的临时访问地址。
- `media_url` 当前有效期为 15 分钟，不应持久化保存。

可能的错误：

| HTTP 状态码 | 错误码 | 说明 |
| --- | --- | --- |
| `400` | `INVALID_ARGUMENT` | 路径中的 UUID 格式不正确 |
| `401` | `UNAUTHENTICATED` | 登录状态无效 |
| `404` | `MEDIA_NOT_FOUND` | Media 不存在或不属于当前用户 |
| `409` | `MEDIA_UPLOAD_INCOMPLETE` | 对象存储中尚未找到文件 |
| `409` | `MEDIA_STATUS_CONFLICT` | Media 当前状态不允许完成上传 |
| `410` | `MEDIA_UPLOAD_EXPIRED` | 文件上传时间晚于申请有效期 |
| `422` | `MEDIA_SIZE_MISMATCH` | 实际文件大小与申请不一致 |
| `422` | `MEDIA_MIME_TYPE_MISMATCH` | 实际 MIME 类型与申请不一致 |

## 6. 全局错误码

| HTTP 状态码 | 错误码 | 说明 |
| --- | --- | --- |
| `400` | `INVALID_ARGUMENT` | 请求参数错误 |
| `400` | `INVALID_PHONE` | 手机号格式不正确 |
| `400` | `WEAK_PASSWORD` | 密码格式不符合要求 |
| `400` | `INVALID_NICKNAME` | 昵称格式不符合要求 |
| `400` | `INVALID_GENDER` | 性别参数不正确 |
| `400` | `INVALID_SIGNATURE` | 个性签名格式不正确 |
| `400` | `INVALID_BIRTHDAY` | 生日格式不正确 |
| `400` | `INVALID_COUNTRY` | 国家或地区格式不正确 |
| `400` | `INVALID_PROVINCE` | 一级行政区格式不正确 |
| `400` | `INVALID_MEDIA_TYPE` | 媒体类型不正确 |
| `401` | `INVALID_CREDENTIALS` | 手机号或密码错误 |
| `401` | `UNAUTHENTICATED` | 请先登录 |
| `401` | `INVALID_REFRESH_TOKEN` | 登录状态已失效，请重新登录 |
| `403` | `ACCOUNT_DISABLED` | 账号不可用 |
| `404` | `USER_NOT_FOUND` | 用户不存在 |
| `404` | `MEDIA_NOT_FOUND` | 媒体不存在 |
| `409` | `PHONE_REGISTERED` | 手机号已经注册 |
| `409` | `MEDIA_UPLOAD_INCOMPLETE` | 媒体文件尚未上传完成 |
| `409` | `MEDIA_STATUS_CONFLICT` | 媒体状态不允许执行当前操作 |
| `410` | `MEDIA_UPLOAD_EXPIRED` | 上传申请已过期 |
| `413` | `MEDIA_TOO_LARGE` | 媒体文件过大 |
| `415` | `UNSUPPORTED_MEDIA_FORMAT` | 不支持的媒体格式 |
| `422` | `REGISTER_CODE_INVALID` | 验证码错误 |
| `422` | `REGISTER_CODE_EXPIRED` | 验证码已过期 |
| `422` | `MEDIA_SIZE_MISMATCH` | 媒体文件大小与申请不一致 |
| `422` | `MEDIA_MIME_TYPE_MISMATCH` | 媒体文件格式与申请不一致 |
| `429` | `REGISTER_CODE_COOLDOWN` | 验证码发送过于频繁 |
| `429` | `REGISTER_CODE_HOURLY_LIMIT` | 验证码发送次数过多 |
| `500` | `INTERNAL_ERROR` | 服务器内部错误 |

## 7. 静态资源路径

前端可以通过以下路径访问服务端静态资源：

```text
/static/avatars/*path
/static/files/*path
```
