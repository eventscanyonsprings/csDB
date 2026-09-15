package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"
)

func CreateSession(ctx context.Context, db *sql.DB, username string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	expiry := time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339)
	_, err := db.ExecContext(ctx, `INSERT INTO sessions(token,username,created_at,expires_at) VALUES(?,?,?,?)`,
		token, username, time.Now().Format(time.RFC3339), expiry)
	return token, err
}

func FindSession(ctx context.Context, db *sql.DB, token string) (string, bool, error) {
	var username string
	var expiresAt string
	err := db.QueryRowContext(ctx, `SELECT username, expires_at FROM sessions WHERE token=?`, token).Scan(&username, &expiresAt)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if expiresAt != "" && time.Now().Format(time.RFC3339) > expiresAt {
		DeleteSession(ctx, db, token)
		return "", false, nil
	}
	return username, true, nil
}

func DeleteSession(ctx context.Context, db *sql.DB, token string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE token=?`, token)
	return err
}
