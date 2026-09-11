# 文件服务器认证系统 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为文件服务器增加认证系统：未登录可下载，登录子账号可上传，管理员可管理子账号。

**Architecture:** 在现有 Gin 服务上增加 Session 认证中间件。SQLite 存储用户，bcrypt 哈希密码，首次启动自动创建 admin。前端增加登录页和管理页。

**Tech Stack:** Gin + modernc.org/sqlite + gorilla/sessions + golang.org/x/crypto/bcrypt

**设计文档:** `docs/plans/2026-09-11-auth-design.md`

---

### Task 1: 添加依赖并初始化代码结构

**Files:**
- Modify: `go.mod`
- Create: `user.go`, `db.go`, `middleware.go`, `auth.go`

**Step 1: 添加依赖**

```bash
go get modernc.org/sqlite@latest golang.org/x/crypto@latest github.com/gorilla/sessions@latest
```

**Step 2: 创建 db.go - SQLite 初始化 + admin 自动创建**

```go
package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var db *sql.DB

const dbDir = "./data"

func initDB() {
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		panic(fmt.Sprintf("Failed to create data dir: %v", err))
	}

	var err error
	db, err = sql.Open("sqlite", filepath.Join(dbDir, "fileserver.db"))
	if err != nil {
		panic(fmt.Sprintf("Failed to open db: %v", err))
	}

	schema := `CREATE TABLE IF NOT EXISTS users (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		username   TEXT    UNIQUE NOT NULL,
		password   TEXT    NOT NULL,
		role       TEXT    NOT NULL DEFAULT 'user',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := db.Exec(schema); err != nil {
		panic(fmt.Sprintf("Failed to init schema: %v", err))
	}

	ensureAdmin()
}

func ensureAdmin() {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		panic(err)
	}
	if count > 0 {
		return
	}

	// 生成随机密码
	bytes := make([]byte, 4)
	rand.Read(bytes)
	password := hex.EncodeToString(bytes)

	hash, err := hashPassword(password)
	if err != nil {
		panic(err)
	}

	if _, err := db.Exec(`INSERT INTO users (username, password, role) VALUES (?, ?, 'admin')`, "admin", hash); err != nil {
		panic(err)
	}

	log.Printf("======================================")
	log.Printf("初始管理员账号已创建")
	log.Printf("用户名: admin")
	log.Printf("密码: %s", password)
	log.Printf("请登录后立即修改密码！")
	log.Printf("======================================")
}
```

**Step 3: 创建 user.go - 用户模型**

```go
package main

import "golang.org/x/crypto/bcrypt"

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func checkPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func getUserByUsername(username string) (*User, error) {
	var u User
	err := db.QueryRow(`SELECT id, username, role FROM users WHERE username = ?`, username).Scan(&u.ID, &u.Username, &u.Role)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func getUserByID(id int64) (*User, error) {
	var u User
	err := db.QueryRow(`SELECT id, username, role FROM users WHERE id = ?`, id).Scan(&u.ID, &u.Username, &u.Role)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func getPasswordHash(username string) (string, error) {
	var hash string
	err := db.QueryRow(`SELECT password FROM users WHERE username = ?`, username).Scan(&hash)
	return hash, err
}
```

**Step 4: 创建 middleware.go - 认证中间件**

```go
package main

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

var store = sessions.NewCookieStore(randomKey())

func randomKey() []byte {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return []byte(hex.EncodeToString(bytes))
}

func getSession(c *gin.Context) *sessions.Session {
	session, _ := store.Get(c.Request, "fileserver_session")
	return session
}

func currentUser(c *gin.Context) *User {
	session := getSession(c)
	uid, ok := session.Values["user_id"].(int64)
	if !ok {
		return nil
	}
	user, err := getUserByID(uid)
	if err != nil {
		return nil
	}
	c.Set("user", user)
	return user
}

func requireLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUser(c) == nil {
			c.JSON(401, gin.H{"error": "请先登录"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUser(c)
		if user == nil {
			c.JSON(401, gin.H{"error": "请先登录"})
			c.Abort()
			return
		}
		if user.Role != "admin" {
			c.JSON(403, gin.H{"error": "无权限"})
			c.Abort()
			return
		}
		c.Next()
	}
}
```

**Step 5: 提交**

```bash
git add go.mod go.sum user.go db.go middleware.go
git commit -m "feat: 添加用户模型、数据库初始化和认证中间件"
```

---

### Task 2: 认证 handler

**Files:**
- Create: `auth.go`
- Modify: `main.go`

**Step 1: 创建 auth.go - 登录/登出/改密/重置**

```go
package main

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func handleLogin(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入用户名和密码"})
		return
	}

	hash, err := getPasswordHash(req.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	if !checkPassword(hash, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	user, err := getUserByUsername(req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}

	session := getSession(c)
	session.Values["user_id"] = user.ID
	session.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
	}
	session.Save(c.Request, c.Writer)

	c.JSON(http.StatusOK, gin.H{"user": user})
}

func handleLogout(c *gin.Context) {
	session := getSession(c)
	session.Values["user_id"] = nil
	session.Options = &sessions.Options{
		Path:   "/",
		MaxAge: -1,
	}
	session.Save(c.Request, c.Writer)
	c.JSON(http.StatusOK, gin.H{"message": "已退出登录"})
}

func handleMe(c *gin.Context) {
	user := currentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

func handleChangePassword(c *gin.Context) {
	user := currentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	oldHash, err := getPasswordHash(user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}

	if !checkPassword(oldHash, req.OldPassword) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "旧密码错误"})
		return
	}

	newHash, err := hashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}

	if _, err := db.Exec(`UPDATE users SET password = ? WHERE id = ?`, newHash, user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码修改成功"})
}

type resetPasswordRequest struct {
	Username string `json:"username" binding:"required"`
}

func handleResetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入用户名"})
		return
	}

	if _, err := getUserByUsername(req.Username); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	newHash, err := hashPassword("123456")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}

	if _, err := db.Exec(`UPDATE users SET password = ? WHERE username = ?`, newHash, req.Username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码已重置为 123456"})
}
```

**Step 2: 创建管理后台 handler（追加到 auth.go）**

```go
type createUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func handleCreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入用户名和密码"})
		return
	}

	if len(req.Username) < 2 || len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名至少2位，密码至少6位"})
		return
	}

	if _, err := getUserByUsername(req.Username); err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名已存在"})
		return
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}

	result, err := db.Exec(`INSERT INTO users (username, password, role) VALUES (?, ?, 'user')`, req.Username, hash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}

	id, _ := result.LastInsertId()
	c.JSON(http.StatusOK, gin.H{"user": User{ID: id, Username: req.Username, Role: "user"}})
}

func handleListUsers(c *gin.Context) {
	rows, err := db.Query(`SELECT id, username, role FROM users WHERE role = 'user' ORDER BY id`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		rows.Scan(&u.ID, &u.Username, &u.Role)
		users = append(users, u)
	}

	c.JSON(http.StatusOK, gin.H{"users": users})
}

func handleDeleteUser(c *gin.Context) {
	id := c.Param("id")

	var role string
	err := db.QueryRow(`SELECT role FROM users WHERE id = ?`, id).Scan(&role)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}

	if role == "admin" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除管理员账号"})
		return
	}

	if _, err := db.Exec(`DELETE FROM users WHERE id = ?`, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "用户已删除"})
}
```

**Step 3: 更新 main.go - 路由注册**

```go
// 文件接口
files := r.Group("/api")
{
	files.GET("/files", listAPKs)
	files.GET("/files/:name", downloadFile)
	files.POST("/files", requireLogin(), uploadFile)
	files.DELETE("/files/:name", requireAdmin(), deleteFile)
}

// 认证接口
auth := r.Group("/api/auth")
{
	auth.POST("/login", handleLogin)
	auth.POST("/logout", requireLogin(), handleLogout)
	auth.GET("/me", handleMe)
	auth.POST("/change-password", requireLogin(), handleChangePassword)
	auth.POST("/reset-password", requireAdmin(), handleResetPassword)
}

// 管理接口
admin := r.Group("/api/admin", requireAdmin())
{
	admin.GET("/users", handleListUsers)
	admin.POST("/users", handleCreateUser)
	admin.DELETE("/users/:id", handleDeleteUser)
}
```

**Step 4: 修改 downloadAPK 函数名和路由**

将 `downloadAPK` 改为 `downloadFile`，删除文件校验 `strings.HasSuffix` 已移除，保持现有实现。

**Step 5: 提交**

```bash
git add auth.go main.go
git commit -m "feat: 添加登录、密码管理和管理后台接口"
```

---

### Task 3: 前端登录页和管理页

**Files:**
- Create: `static/login.html`, `static/admin.html`
- Modify: `static/style.css`, `static/index.html`, `static/app.js`

**Step 1: 创建 login.html**

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>登录 - 文件服务器</title>
    <link rel="stylesheet" href="/static/style.css">
</head>
<body>
    <div class="container auth-container">
        <div class="auth-card">
            <h1>文件服务器</h1>
            <p class="subtitle">请登录后上传文件</p>
            <form id="loginForm">
                <div class="form-group">
                    <input type="text" id="username" placeholder="用户名" required>
                </div>
                <div class="form-group">
                    <input type="password" id="password" placeholder="密码" required>
                </div>
                <button type="submit" class="btn btn-primary btn-block">登录</button>
            </form>
            <div id="loginMessage" class="message"></div>
            <a href="/" class="back-link">← 返回文件列表</a>
        </div>
    </div>
    <script src="/static/login.js"></script>
</body>
</html>
```

**Step 2: 创建 login.js**

```js
const loginForm = document.getElementById('loginForm');
const loginMessage = document.getElementById('loginMessage');

loginForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const username = document.getElementById('username').value.trim();
    const password = document.getElementById('password').value;

    if (!username || !password) {
        showLoginMessage('请输入用户名和密码', 'error');
        return;
    }

    try {
        const resp = await fetch('/api/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'same-origin',
            body: JSON.stringify({ username, password })
        });
        const data = await resp.json();
        if (resp.ok) {
            window.location.href = '/';
        } else {
            showLoginMessage(data.error || '登录失败', 'error');
        }
    } catch (err) {
        showLoginMessage('网络错误', 'error');
    }
});

function showLoginMessage(text, type) {
    loginMessage.textContent = text;
    loginMessage.className = 'message ' + type;
    setTimeout(() => { loginMessage.className = 'message'; }, 3000);
}
```

**Step 3: 创建 admin.html**

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>账号管理 - 文件服务器</title>
    <link rel="stylesheet" href="/static/style.css">
</head>
<body>
    <div class="container">
        <header>
            <h1>账号管理</h1>
            <a href="/" class="back-link">← 返回文件列表</a>
        </header>

        <section>
            <h2>创建子账号</h2>
            <form id="createUserForm">
                <div class="form-group">
                    <input type="text" id="newUsername" placeholder="用户名" required>
                </div>
                <div class="form-group">
                    <input type="password" id="newPassword" placeholder="密码（至少6位）" required>
                </div>
                <button type="submit" class="btn btn-primary">创建账号</button>
            </form>
            <div id="createMessage" class="message"></div>
        </section>

        <section>
            <h2>子账号列表</h2>
            <div id="usersList" class="files-list">
                <p class="loading">加载中...</p>
            </div>
        </section>
    </div>
    <script src="/static/admin.js"></script>
</body>
</html>
```

**Step 4: 创建 admin.js**

```js
const API_BASE = '/api';
const createUserForm = document.getElementById('createUserForm');
const createMessage = document.getElementById('createMessage');

createUserForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const username = document.getElementById('newUsername').value.trim();
    const password = document.getElementById('newPassword').value;

    if (!username) {
        showMsg(createMessage, '请输入用户名', 'error');
        return;
    }
    if (username.length < 2) {
        showMsg(createMessage, '用户名至少2位', 'error');
        return;
    }
    if (password.length < 6) {
        showMsg(createMessage, '密码至少6位', 'error');
        return;
    }

    try {
        const resp = await fetch(API_BASE + '/admin/users', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'same-origin',
            body: JSON.stringify({ username, password })
        });
        const data = await resp.json();
        if (resp.ok) {
            showMsg(createMessage, '创建成功', 'success');
            document.getElementById('newUsername').value = '';
            document.getElementById('newPassword').value = '';
            loadUsers();
        } else {
            showMsg(createMessage, data.error || '创建失败', 'error');
        }
    } catch (err) {
        showMsg(createMessage, '网络错误', 'error');
    }
});

async function loadUsers() {
    const usersList = document.getElementById('usersList');
    usersList.innerHTML = '<p class="loading">加载中...</p>';
    try {
        const resp = await fetch(API_BASE + '/admin/users', { credentials: 'same-origin' });
        const data = await resp.json();
        if (!resp.ok) {
            usersList.innerHTML = '<p class="empty">' + escapeHtml(data.error || '加载失败') + '</p>';
            return;
        }
        const users = data.users || [];
        if (users.length === 0) {
            usersList.innerHTML = '<p class="empty">暂无子账号</p>';
            return;
        }
        usersList.innerHTML = users.map(u => `
            <div class="file-item">
                <div class="file-info">
                    <div class="file-name">${escapeHtml(u.username)}</div>
                    <div class="file-meta">ID: ${u.id}</div>
                </div>
                <div class="file-actions">
                    <button onclick="resetPassword('${escapeHtml(u.username)}')" class="btn btn-small">重置密码</button>
                    <button onclick="deleteUser(${u.id})" class="btn btn-delete">删除</button>
                </div>
            </div>
        `).join('');
    } catch (err) {
        usersList.innerHTML = '<p class="empty">加载失败</p>';
    }
}

async function resetPassword(username) {
    if (!confirm('确认将 ' + username + ' 的密码重置为 123456？')) return;
    try {
        const resp = await fetch(API_BASE + '/auth/reset-password', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'same-origin',
            body: JSON.stringify({ username })
        });
        const data = await resp.json();
        alert(data.message || data.error || '操作完成');
    } catch (err) {
        alert('网络错误');
    }
}

async function deleteUser(id) {
    if (!confirm('确认删除该账号？')) return;
    try {
        const resp = await fetch(API_BASE + '/admin/users/' + id, {
            method: 'DELETE',
            credentials: 'same-origin'
        });
        const data = await resp.json();
        if (resp.ok) {
            loadUsers();
        } else {
            alert(data.error || '删除失败');
        }
    } catch (err) {
        alert('网络错误');
    }
}

function showMsg(el, text, type) {
    el.textContent = text;
    el.className = 'message ' + type;
    setTimeout(() => { el.className = 'message'; }, 3000);
}

function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

loadUsers();
```

**Step 5: 更新 style.css - 补充表单样式**

```css
.auth-container {
    display: flex;
    justify-content: center;
    align-items: flex-start;
    padding-top: 60px;
}

.auth-card {
    background: #fff;
    padding: 40px;
    border-radius: 8px;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
    width: 100%;
    max-width: 360px;
    text-align: center;
}

.auth-card h1 {
    margin-bottom: 5px;
}

.form-group {
    margin-bottom: 15px;
}

.form-group input {
    width: 100%;
    padding: 10px;
    border: 1px solid #ccc;
    border-radius: 4px;
    font-size: 1em;
    box-sizing: border-box;
}

.form-group input:focus {
    outline: none;
    border-color: #1a73e8;
}

.btn-block {
    width: 100%;
}

.back-link {
    display: inline-block;
    margin-top: 15px;
    color: #1a73e8;
    text-decoration: none;
}

.btn-small {
    padding: 5px 12px;
    font-size: 0.85em;
    background: #e8f0fe;
    color: #1a73e8;
}
```

**Step 6: Commit**

```bash
git add static/
git commit -m "feat: 添加登录页和管理后台页面"
```

---

### Task 4: 更新文件列表页（登录状态 + 上传按钮）

**Files:**
- Modify: `static/index.html`, `static/app.js`

**Step 1: 更新 index.html - 头部显示登录状态**

```html
<header>
    <h1>文件服务器</h1>
    <p class="subtitle">上传、管理和下载文件</p>
    <div id="userInfo" class="user-info">
        <span id="loginStatus"></span>
    </div>
</header>
```

上传区域改为"未登录不显示"：

```html
<section class="upload-section" id="uploadSection">
```

**Step 2: 更新 app.js - 登录状态检测**

```js
async function initUser() {
    const userInfo = document.getElementById('loginStatus');
    try {
        const resp = await fetch('/api/auth/me', { credentials: 'same-origin' });
        if (resp.ok) {
            const data = await resp.json();
            const isAdmin = data.user.role === 'admin';
            userInfo.innerHTML = `
                <span>当前用户：${escapeHtml(data.user.username)}${isAdmin ? '（管理员）' : ''}</span>
                ${isAdmin ? '<a href="/static/admin.html" class="btn btn-small">账号管理</a>' : ''}
                <button onclick="logout()" class="btn btn-small btn-secondary">退出登录</button>
            `;
            document.getElementById('uploadSection').style.display = 'block';
        } else {
            userInfo.innerHTML = '<a href="/static/login.html" class="btn btn-small">登录</a>';
            document.getElementById('uploadSection').style.display = 'none';
        }
    } catch (err) {
        userInfo.innerHTML = '<a href="/static/login.html" class="btn btn-small">登录</a>';
    }
}

async function logout() {
    try {
        await fetch('/api/auth/logout', { method: 'POST', credentials: 'same-origin' });
        window.location.reload();
    } catch (err) {
        alert('退出失败');
    }
}

initUser();
```

**Step 3: 删除按钮仅管理员可见**

在 `loadAPKs` 中，根据用户角色决定是否显示删除按钮：需要拿到当前用户 role，存全局变量 `currentRole`。在文件列表渲染时：

```js
const deleteBtn = currentRole === 'admin' ? `<button onclick="deleteAPK(...)" class="btn btn-delete">删除</button>` : '';
```

**Step 4: Commit**

```bash
git add static/index.html static/app.js
git commit -m "feat: 文件列表页集成登录状态和权限控制"
```

---

### Task 5: 构建验证与文档更新

**Step 1: 本地构建测试**

```bash
go build -o fileservice.exe .
go vet ./...
```

需确保：`downloadFile` 无遗漏引用，imports 无多余。

**Step 2: 手动测试 API 流程（PowerShell）**

```bash
# 启动
./fileservice.exe
# 检查 admin 日志输出，获取初始密码（注意：已有数据库则复用旧admin密码）
# 登录
Invoke-RestMethod -Uri http://localhost:8086/api/auth/login -Method Post -Body '{"username":"admin","password":"<pw>"}' -ContentType 'application/json' -SessionVariable s
# 创建子账号
Invoke-RestMethod -Uri http://localhost:8086/api/admin/users -Method Post -Body '{"username":"test1","password":"test123"}' -ContentType 'application/json' -WebSession $s
# 列出文件
Invoke-RestMethod -Uri http://localhost:8086/api/files
```

**Step 3: 交叉编译 Linux**

```bash
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o fileservice .
```

**Step 4: 更新设计实现状态**

在 build 验证通过后，提交最终代码。

**Step 5: Commit**

```bash
git add .
git commit -m "chore: 构建验证认证系统"
```

---

## 验证清单

| 场景 | 预期行为 |
|------|---------|
| 未登录访问首页 | 仅显示下载按钮，无上传区域 |
| 未登录调上传 API | 401 |
| 未登录调下载 API | 200 |
| 登录子账号 | 显示上传区域，无管理入口 |
| 管理员登录 | 显示上传区域 + 账号管理入口 |
| 管理员创建子账号 | 创建成功，列表可见 |
| 管理员删除子账号 | 删除成功 |
| 管理员重置密码 | 子账号用 123456 可登录 |
| 修改自己密码 | 旧密码失效，新密码生效 |
| 子账号访问管理接口 | 403 |