package store

import (
	"context"
	"database/sql"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func UserCount(ctx context.Context, db *sql.DB) (int, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func UserExists(ctx context.Context, db *sql.DB, username string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `SELECT 1 FROM users WHERE username=?`, username).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return exists, err
}

func CreateUser(ctx context.Context, db *sql.DB, username, password string, isAdmin bool) error {
	return CreateUserWithProfile(ctx, db, username, password, isAdmin, "", "")
}

func CreateUserWithProfile(ctx context.Context, db *sql.DB, username, password string, isAdmin bool, displayName, phone string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `INSERT INTO users(username,password_hash,is_admin,created_at,display_name,phone) VALUES(?,?,?,?,?,?)`,
		username, string(hash), isAdmin, time.Now().Format(time.RFC3339), displayName, phone)
	return err
}

func FindUser(ctx context.Context, db *sql.DB, username, password string) (bool, bool, error) {
	var hash string
	var isAdmin bool
	err := db.QueryRowContext(ctx, `SELECT password_hash, is_admin FROM users WHERE username=?`, username).Scan(&hash, &isAdmin)
	if err == sql.ErrNoRows {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return false, false, nil
	}
	return true, isAdmin, nil
}

func ListUsers(ctx context.Context, db *sql.DB) ([]User, error) {
	rows, err := db.QueryContext(ctx, `SELECT username, is_admin, created_at, COALESCE(display_name,''), COALESCE(phone,'') FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.Username, &u.IsAdmin, &u.CreatedAt, &u.DisplayName, &u.Phone); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func ListLoginHashes(ctx context.Context, db *sql.DB) ([]LoginHash, error) {
	rows, err := db.QueryContext(ctx, `SELECT username, password_hash FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logins []LoginHash
	for rows.Next() {
		var login LoginHash
		if err := rows.Scan(&login.Username, &login.PasswordHash); err != nil {
			return nil, err
		}
		logins = append(logins, login)
	}
	return logins, rows.Err()
}

func DeleteUser(ctx context.Context, db *sql.DB, username string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM users WHERE username=?`, username)
	return err
}

func SetAdmin(ctx context.Context, db *sql.DB, username string, isAdmin bool) error {
	_, err := db.ExecContext(ctx, `UPDATE users SET is_admin=? WHERE username=?`, isAdmin, username)
	return err
}

func IsAdmin(ctx context.Context, db *sql.DB, username string) (bool, error) {
	var isAdmin bool
	err := db.QueryRowContext(ctx, `SELECT is_admin FROM users WHERE username=?`, username).Scan(&isAdmin)
	if err != nil {
		return false, err
	}
	return isAdmin, nil
}

func UpdatePassword(ctx context.Context, db *sql.DB, username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `UPDATE users SET password_hash=? WHERE username=?`, string(hash), username)
	return err
}

func UpdateProfile(ctx context.Context, db *sql.DB, username, displayName, phone string) error {
	_, err := db.ExecContext(ctx, `UPDATE users SET display_name=?, phone=? WHERE username=?`, displayName, phone, username)
	return err
}

type User struct {
	Username    string `json:"username"`
	IsAdmin     bool   `json:"is_admin"`
	CreatedAt   string `json:"created_at"`
	DisplayName string `json:"display_name"`
	Phone       string `json:"phone"`
}

type LoginHash struct {
	Username     string
	PasswordHash string
}
