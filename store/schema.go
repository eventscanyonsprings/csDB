package store

import (
	"database/sql"
)

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := Initialize(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func Initialize(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS suites (
		suite_id TEXT PRIMARY KEY, date_updated TEXT NOT NULL DEFAULT '',
		entercode TEXT NOT NULL DEFAULT '', backup_keyset_num TEXT NOT NULL DEFAULT '',
		owner_is_resident BOOLEAN NOT NULL DEFAULT 1
	);
	CREATE TABLE IF NOT EXISTS residents (
		id INTEGER PRIMARY KEY AUTOINCREMENT, suite_id TEXT NOT NULL REFERENCES suites(suite_id) ON DELETE CASCADE,
		first_name TEXT, last_name TEXT, is_child BOOLEAN NOT NULL DEFAULT 0, child_age INTEGER, phone_cell TEXT, phone_home TEXT, phone_business TEXT, medical_notes TEXT, fire_notes TEXT DEFAULT ''
	);
	CREATE TABLE IF NOT EXISTS emergency_contacts (
		id INTEGER PRIMARY KEY AUTOINCREMENT, suite_id TEXT NOT NULL REFERENCES suites(suite_id) ON DELETE CASCADE,
		contact_name TEXT, relationship TEXT, address TEXT, phone_cell TEXT, phone_home TEXT, phone_business TEXT, medical_notes TEXT, notes TEXT
	);
	CREATE TABLE IF NOT EXISTS vehicles (
		id INTEGER PRIMARY KEY AUTOINCREMENT, suite_id TEXT NOT NULL REFERENCES suites(suite_id) ON DELETE CASCADE,
		make_model TEXT, year TEXT, plate_number TEXT
	);
	CREATE TABLE IF NOT EXISTS parking_spots (
		id INTEGER PRIMARY KEY AUTOINCREMENT, suite_id TEXT NOT NULL REFERENCES suites(suite_id) ON DELETE CASCADE, spot_number TEXT
	);
	CREATE TABLE IF NOT EXISTS lockers (
		id INTEGER PRIMARY KEY AUTOINCREMENT, suite_id TEXT NOT NULL REFERENCES suites(suite_id) ON DELETE CASCADE, locker_number TEXT
	);
	CREATE TABLE IF NOT EXISTS owners (
		suite_id TEXT PRIMARY KEY REFERENCES suites(suite_id) ON DELETE CASCADE, owner_name TEXT, address TEXT,
		phone_cell TEXT, phone_home TEXT, phone_business TEXT
	);
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		is_admin BOOLEAN NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT ''
	);
	CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		username TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT '',
		expires_at TEXT NOT NULL DEFAULT ''
	);`)
	if err != nil {
		return err
	}
	var usersTable string
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='users'`).Scan(&usersTable)
	if err == sql.ErrNoRows {
		_, err = db.Exec(`CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			is_admin BOOLEAN NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT ''
		)`)
		if err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}
	var sessionsTable string
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='sessions'`).Scan(&sessionsTable)
	if err == sql.ErrNoRows {
		_, err = db.Exec(`CREATE TABLE sessions (
			token TEXT PRIMARY KEY,
			username TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT '',
			expires_at TEXT NOT NULL DEFAULT ''
		)`)
		if err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}
	var residentMedicalNotesColumn string
	err = db.QueryRow(`SELECT name FROM pragma_table_info('residents') WHERE name='medical_notes'`).Scan(&residentMedicalNotesColumn)
	if err == sql.ErrNoRows {
		_, err = db.Exec(`ALTER TABLE residents ADD COLUMN medical_notes TEXT`)
		if err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}
	var residentFireNotesColumn string
	err = db.QueryRow(`SELECT name FROM pragma_table_info('residents') WHERE name='fire_notes'`).Scan(&residentFireNotesColumn)
	if err == sql.ErrNoRows {
		_, err = db.Exec(`ALTER TABLE residents ADD COLUMN fire_notes TEXT DEFAULT ''`)
	}
	if err != nil {
		return err
	}
	var contactNotesColumn string
	err = db.QueryRow(`SELECT name FROM pragma_table_info('emergency_contacts') WHERE name='notes'`).Scan(&contactNotesColumn)
	if err == sql.ErrNoRows {
		_, err = db.Exec(`ALTER TABLE emergency_contacts ADD COLUMN notes TEXT DEFAULT ''`)
	}
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS audit_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		created_at TEXT NOT NULL DEFAULT '',
		actor TEXT NOT NULL DEFAULT '',
		action TEXT NOT NULL DEFAULT '',
		target TEXT NOT NULL DEFAULT '',
		detail TEXT NOT NULL DEFAULT ''
	)`)
	if err != nil {
		return err
	}
	// User profile fields (name + phone, added after initial schema).
	for _, col := range []struct{ name, ddl string }{
		{"display_name", `ALTER TABLE users ADD COLUMN display_name TEXT NOT NULL DEFAULT ''`},
		{"phone", `ALTER TABLE users ADD COLUMN phone TEXT NOT NULL DEFAULT ''`},
	} {
		var found string
		err = db.QueryRow(`SELECT name FROM pragma_table_info('users') WHERE name='` + col.name + `'`).Scan(&found)
		if err == sql.ErrNoRows {
			if _, err = db.Exec(col.ddl); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	return nil
}
