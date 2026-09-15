package main

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

//go:embed static/*
var staticFiles embed.FS

const (
	uploadDir = "./apks"
	port      = ":8086"
)

func main() {
	initDB()

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		panic(fmt.Sprintf("Failed to create upload directory: %v", err))
	}

	r := gin.Default()
	r.MaxMultipartMemory = 32 << 20 // 32 MiB

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(fmt.Sprintf("Failed to init static files: %v", err))
	}
	r.StaticFS("/static", http.FS(staticFS))
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/static/index.html")
	})

	// 文件接口
	api := r.Group("/api")
	{
		api.GET("/files", listFiles)
		api.POST("/files", requireLogin(), uploadFile)
		api.GET("/files/:filename", downloadFile)
		api.DELETE("/files/:filename", requireLogin(), deleteFile)
		api.PUT("/files/:filename/name", requireLogin(), updateFileName)
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

	fmt.Printf("Server starting on http://localhost%s\n", port)
	fmt.Printf("File storage directory: %s\n", uploadDir)
	if err := r.Run(port); err != nil {
		panic(err)
	}
}

func listFiles(c *gin.Context) {
	entries, err := os.ReadDir(uploadDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var files []map[string]interface{}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == ".gitkeep" {
			continue
		}
		info, _ := entry.Info()
		displayName := ""
		db.QueryRow(`SELECT display_name FROM file_meta WHERE filename = ?`, entry.Name()).Scan(&displayName)
		files = append(files, map[string]interface{}{
			"name":         entry.Name(),
			"display_name": displayName,
			"size":         info.Size(),
			"size_human":   humanSize(info.Size()),
			"modified":     info.ModTime().Format(time.RFC3339),
			"download_url": fmt.Sprintf("/api/files/%s", entry.Name()),
		})
	}

	c.JSON(http.StatusOK, gin.H{"files": files})
}

func uploadFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}
	defer file.Close()

	dstPath := filepath.Join(uploadDir, header.Filename)
	dst, err := os.Create(dstPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Upload successful",
		"filename":     header.Filename,
		"download_url": fmt.Sprintf("/api/files/%s", header.Filename),
	})
}

func downloadFile(c *gin.Context) {
	filename := c.Param("filename")

	filePath := filepath.Join(uploadDir, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.File(filePath)
}

func deleteFile(c *gin.Context) {
	filename := c.Param("filename")
	filePath := filepath.Join(uploadDir, filename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	if err := os.Remove(filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file"})
		return
	}

	if _, err := db.Exec(`DELETE FROM file_meta WHERE filename = ?`, filename); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file meta"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "File deleted"})
}

type updateNameRequest struct {
	Name string `json:"name"`
}

func updateFileName(c *gin.Context) {
	filename := c.Param("filename")
	filePath := filepath.Join(uploadDir, filename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	var req updateNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)

	if req.Name == "" {
		if _, err := db.Exec(`DELETE FROM file_meta WHERE filename = ?`, filename); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
			return
		}
	} else {
		if _, err := db.Exec(`INSERT INTO file_meta (filename, display_name) VALUES (?, ?)
			ON CONFLICT(filename) DO UPDATE SET display_name = excluded.display_name`, filename, req.Name); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "名称已更新"})
}

func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}