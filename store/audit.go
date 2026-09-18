package store

import (
	"context"
	"database/sql"
	"time"
)

type AuditEntry struct {
	ID        int64  `json:"id"`
	CreatedAt string `json:"created_at"`
	Actor     string `json:"actor"`
	Action    string `json:"action"`
	Target    string `json:"target"`
	Detail    string `json:"detail"`
}

func RecordAudit(ctx context.Context, db *sql.DB, actor, action, target, detail string) error {
	_, err := db.ExecContext(ctx, `INSERT INTO audit_logs(created_at,actor,action,target,detail) VALUES(?,?,?,?,?)`,
		time.Now().Format(time.RFC3339), actor, action, target, detail)
	return err
}

func ListAudit(ctx context.Context, db *sql.DB, limit int) ([]AuditEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := db.QueryContext(ctx, `SELECT id, created_at, actor, action, target, COALESCE(detail,'') FROM audit_logs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []AuditEntry{}
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.CreatedAt, &e.Actor, &e.Action, &e.Target, &e.Detail); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
