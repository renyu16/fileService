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

	metaSchema := `CREATE TABLE IF NOT EXISTS file_meta (
		filename     TEXT PRIMARY KEY,
		display_name TEXT
	);`
	if _, err := db.Exec(metaSchema); err != nil {
		panic(fmt.Sprintf("Failed to init file_meta schema: %v", err))
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