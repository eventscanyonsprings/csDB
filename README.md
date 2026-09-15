# csDB (canyon-springs-residents) - root files
Web app for Canyon Springs residents and suites data.

Go + SQLite (modernc.org/sqlite). Templates in templates/, DB logic in store/, dev tools in tools/, dummy-data seeder in seed/.

Run from this directory (where go.mod lives). DB defaults to ./canyon-springs.db, override with CANYON_DB. HTTP defaults to :8081, override with CANYON_ADDR.

## Files in this directory

### main.go (Go, package main)
The whole HTTP server. main() opens the DB via store.Open, parses templates, wires routes (/login, /logout, /units, /edit, /report/fire, /admin, /api/search, /api/units, /export/spreadsheet, /suite/...) and serves on CANYON_ADDR. All handlers live here: handleLogin, handleLogout, handleUnits, handleSearch, handleSpreadsheet, handlePDF, handleSuite, handleSave, handleAdminPage, handleAdminUsers, handleAdminUserAction, plus logging and writeJSON helpers.

### auth.go (Go, package main)
Auth middleware and helpers used by main.go. loadUser reads the cs_session cookie via store.FindSession. requireAuth redirects to /login when the users table is non-empty and nobody is logged in (open access when user count is 0, for first-time bootstrap). requireAdmin enforces the admin role. getUser, isAdmin, userCount are small helpers.

### test_client.go (Go, build ignore, package main)
Manual login smoke test, excluded from normal builds by the go:build ignore tag. Run with: go run test_client.go against a running server (CANYON_TEST_ADDR, default http://localhost:8099). Tests: wrong password rejected, correct login (admin/admin) sets cookie, /units and /admin reachable, logout invalidates session and /units redirects to /login.

### _dbschema.py (Python helper)
Quick DB inspector. Opens canyon-springs.db and prints every table plus columns (sqlite_master plus PRAGMA table_info). Leading underscore keeps Go tooling from touching it. Usage: python _dbschema.py.

### go.mod (Go module)
Module canyon-springs-residents, Go 1.26.0. Direct deps: github.com/go-pdf/fpdf (PDF export) and modernc.org/sqlite (pure-Go SQLite driver).

### go.sum (Go checksums)
Pinned hashes for go.mod deps plus transitives (golang.org/x/crypto for bcrypt, modernc.org/libc, etc.). Needed for reproducible go build and go run.

### .gitignore (Git)
Keeps local and generated files out of git: server.exe, the real canyon-springs.db, dummy and test DBs (dummy*.db, test_*.db, verify*.db, login-smoke.db, hash-smoke.db), bak files, seed.exe, log files, srv.err.

### canyon-springs.db (SQLite database)
The live data file (suites, residents, contacts, vehicles, users, sessions). Auto-created and migrated by store.Open and store.Initialize on first open, so deleting it means fresh start (you must re-create a user via tools/create-admin, see tools/README.md). Git-ignored, do not commit.

### server.exe (built binary)
Compiled server from go build (about 19 MB). Run it with server.exe. Git-ignored. Rebuild after changing Go files or templates (templates are embedded via go:embed, so template edits also need a rebuild).

## Directories (for context)

- store/ - DB layer: schema.go (Open/Initialize plus schema and migrations), users.go (bcrypt users), sessions.go (login tokens), load.go and save.go (suite read/write), types.go (Suite/Resident structs), helpers.go (SuiteCount).
- templates/ - HTML pages embedded by main.go: login.html, units.html, edit.html, fire.html, admin.html.
- tools/ - CLIs: create-admin (bootstrap/add users), dump-logins (list username:hash), create-user/ (empty placeholder). See tools/README.md.
- seed/ - Dummy-data generator: go run ./seed -db dummy.db [-force] creates 132 fake suites (11 floors x 12). Refuses to touch the real canyon-springs.db.

## Related docs

- tools/README.md - how to create the first user on a fresh DB and how to dump logins.
