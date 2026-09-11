# 文件服务器添加认证系统设计文档

日期：2026-09-11

## 背景

文件服务器当前无任何认证，任何人都可上传、下载、删除文件。需增加登录认证：
- 未登录用户可下载、浏览文件列表
- 子账号可上传文件
- 管理员可上传、删除文件，并管理子账号

## 需求总结

| 角色 | 权限 |
|------|------|
| 访客（未登录） | 下载、浏览列表 |
| 子账号 | 上传 + 下载 |
| 管理员 | 上传、下载、删除、管理子账号 |

## 技术选型

- Gin 中间件 + SQLite（`modernc.org/sqlite`，纯 Go 无 CGO）
- Session 管理（`gorilla/sessions`，cookie-store）
- 密码哈希：bcrypt（`golang.org/x/crypto/bcrypt`）
- 首次启动自动创建 admin 账号，密码输出到日志
- 仅管理员可创建子账号，不允许自行注册

## 数据模型

```sql
CREATE TABLE users (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    username   TEXT    UNIQUE NOT NULL,
    password   TEXT    NOT NULL,   -- bcrypt hash
    role       TEXT    NOT NULL DEFAULT 'user',  -- 'admin' | 'user'
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

SQLite 数据文件：`./data/fileserver.db`，随程序自动创建。

## 认证流程

```
登录：POST /api/auth/login  →  设置 session cookie
登出：POST /api/auth/logout →  清除 session
检查：GET  /api/auth/me     →  返回当前用户信息/401
```

- 密码 bcrypt 哈希存储
- Session 存 cookie-store，密钥随机生成
- Cookie 名 `fileserver_session`，24 小时过期

## 密码管理

```
修改密码：POST /api/auth/change-password  → 本人修改，需验证旧密码
重置密码：POST /api/auth/reset-password    → 管理员重置为默认 123456
```

## API 接口

```yaml
# 公开接口（无需登录）
GET    /api/files           →  文件列表
GET    /api/files/:name     →  下载文件

# 需登录接口
POST   /api/files           →  上传文件
POST   /api/auth/logout     →  登出
GET    /api/auth/me         →  当前用户信息
POST   /api/auth/change-password  →  修改自己密码

# 仅管理员接口
POST   /api/admin/users     →  创建子账号
GET    /api/admin/users     →  查看子账号列表
DELETE /api/admin/users/:id →  删除子账号
POST   /api/auth/reset-password   →  重置子账号密码为123456
```

## 前端变更

```
static/
├── index.html     → 文件浏览页（下载+列表，显示登录状态/用户名）
├── login.html     → 登录页
├── admin.html     → 管理后台页（仅管理员可访问）
├── style.css      → 样式（新增登录/管理页样式）
├── app.js         → 文件页逻辑（登录状态检测、上传带 cookie）
└── admin.js       → 管理后台逻辑（创建/删除子账号、重置密码）
```

## 项目结构

```
main.go        → 服务入口 + 路由注册
user.go        → 用户模型 + 密码哈希
auth.go        → 登录/登出/改密/重置 handler
middleware.go  → auth 中间件（需登录 / 需管理员）
db.go          → SQLite 初始化 + admin 自动创建
static/        → 前端页面
```

## 测试场景

1. 未登录访问首页 → 仅看到下载按钮
2. 未登录调上传 API → 401
3. 登录子账号 → 可上传，看不到管理入口
4. 管理员登录 → 可见管理入口，可创建/删除子账号
5. 管理员重置子账号密码 → 子账号可用 123456 登录
6. 修改密码 → 旧密码失效、新密码生效

## 部署

- 单二进制部署，Linux 交叉编译验证（GOOS=linux GOARCH=amd64）
- 自动创建 `data/` 目录和 admin 账号
- 部署方式不变，端口 8086