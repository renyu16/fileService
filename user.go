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