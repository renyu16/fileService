package main

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
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