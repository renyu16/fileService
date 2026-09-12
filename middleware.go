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